package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"github.com/jinzhu/copier"
)

func handleConsensus(
	nodeCtx *NodeCtx,
	cMsg *ConsensusMsg,
	fromPub *PubKey) {
	switch cMsg.Tag {
	case "propose":
		// TODO validate block with header

		// TODO check header actually comes from leader both by sig, and by election protocol
		// double check
		if cMsg.Pub.Bytes != fromPub.Bytes {
			errFatal(nil, "LeaderID not the same as FromID")
		}

		// lock consensusMsg operations
		nodeCtx.consensusMsgs.mux.Lock()

		// check that we do not have any other message with this gossipheader
		// TODO if blocks are reproposed then chagne this
		if nodeCtx.consensusMsgs._exists(cMsg.GossipHash) {
			fmt.Println(cMsg)
			errr(nil, "allready have msgs in this gossiphash")
			return
		}

		nodeCtx.consensusMsgs._add(cMsg.GossipHash, cMsg.Pub.Bytes, cMsg)

		// unlock mutex
		nodeCtx.consensusMsgs.mux.Unlock()

		// log.Println("sent echo")
		newMsg := new(ConsensusMsg)
		newMsg.GossipHash = cMsg.GossipHash
		newMsg.Tag = "echo"
		newMsg.Pub = nodeCtx.self.Priv.Pub
		newMsg.sign(nodeCtx.self.Priv)
		msg := Msg{"consensus", newMsg, nodeCtx.self.Priv.Pub}
		sendMsgToCommitteeAndSelf(msg, nodeCtx)

	case "echo":
		// add echo

		log.Println("Echo recived from ", fromPub.string())
		// wait for delta so each node will recive enough echos
		dur := default_delta * time.Millisecond
		time.Sleep(dur)

		_msg := Msg{"consensus", "echo", nodeCtx.self.Priv.Pub}
		go dialAndSend(coord+":8080", _msg)

		// TODO check valid header

		// TODO check if every header we have recived is unique
		// if not, then send special header with tag pending

		// TODO check that we recived propose from leader allready

		// check that we have recived a propose from this gossiphash

		if !nodeCtx.consensusMsgs.exists(cMsg.GossipHash) {
			timeout := 0
			for {
				time.Sleep(dur)
				if nodeCtx.consensusMsgs.exists(cMsg.GossipHash) {
					break
				}
				if timeout > 5 {
					// errFatal(nil, "Recived an echo, but have not recived a propose for this gossiphash")
					// handleConsensusAccept will deal with missing block
					return
				}
				timeout++
			}
		}
		nodeCtx.consensusMsgs.add(cMsg.GossipHash, cMsg.Pub.Bytes, cMsg)
		nodeCtx.channels.echoChan <- true

	case "pending":
		// don't accept this iteration

		// TODO check validity of header

		// TODO check that it is different from other recivied valid headers

		// set header of fromid to this pending, so accept round can check
		nodeCtx.consensusMsgs.add(cMsg.GossipHash, cMsg.Pub.Bytes, cMsg)

		_msg := Msg{"consensus", "pending", nodeCtx.self.Priv.Pub}
		go dialAndSend(coord+":8080", _msg)
		// terminate without accepting
		return
	case "accept":
		nodeCtx.consensusMsgs.add(cMsg.GossipHash, cMsg.Pub.Bytes, cMsg)

		_msg := Msg{"consensus", "accept", nodeCtx.self.Priv.Pub}
		go dialAndSend(coord+":8080", _msg)

		// now add final block if recived enough accepts

		// log.Println("Success, recived accept from ", fromPub)

	default:
		errFatal(nil, "header tag not known")
	}
}

// Because we start the synchronous rounds on the first propose from leader, then we spawn this,
func handleConsensusEcho(
	cMsg *ConsensusMsg,
	nodeCtx *NodeCtx) {

	requiredVotes := (len(nodeCtx.committee.Members) / int(nodeCtx.flagArgs.committeeF)) + 1

	// leader propose, echo gossip
	time.Sleep(2 * default_delta * time.Millisecond)

	if len(nodeCtx.channels.echoChan) < int(requiredVotes) {
		//  wait a few ms to be sure (computing)
		timeout := 0
		for len(nodeCtx.channels.echoChan) < int(requiredVotes) {
			time.Sleep(10 * time.Millisecond)
			timeout += 1
			if t := default_delta; timeout >= t {
				errr(nil, fmt.Sprintf("Echos not recived in time %d", t))
				return
			}
		}
	}

	// TODO handle pending

	// check if we have enough required votes
	totalVotes := nodeCtx.consensusMsgs.countValidVotes(cMsg.GossipHash, nodeCtx)

	// TODO change to flagArgs
	if totalVotes >= requiredVotes {
		// enough votes, send accept
		newMsg := new(ConsensusMsg)
		newMsg.GossipHash = cMsg.GossipHash
		newMsg.Tag = "accept"
		newMsg.Pub = nodeCtx.self.Priv.Pub
		newMsg.sign(nodeCtx.self.Priv)
		msg := Msg{"consensus", newMsg, nodeCtx.self.Priv.Pub}
		sendMsgToCommitteeAndSelf(msg, nodeCtx)

	} else {
		// not enough votes, terminate
		// TODO add coordinator feedback here
		log.Println("Not enough votes ", totalVotes)
		return
	}
}

func handleConsensusAccept(
	cMsg *ConsensusMsg,
	nodeCtx *NodeCtx,
	recursive int64) {

	requiredVotes := (len(nodeCtx.committee.Members) / int(nodeCtx.flagArgs.committeeF)) + 1

	if recursive > 0 {
		log.Println("Recursive iter", recursive)
		time.Sleep(default_delta * time.Millisecond)
	} else {
		// leader propose, echo gossip, accept gossip
		time.Sleep(3 * default_delta * time.Millisecond)
	}

	// check if we have enough required votes
	totalVotes := nodeCtx.consensusMsgs.countValidAccepts(cMsg.GossipHash)

	//log.Println("handleConsensusAccept", totalVotes, requiredVotes)
	// TODO change to flagArgs
	if totalVotes >= requiredVotes {
		// enough accepts
		consensusMsgs := nodeCtx.consensusMsgs.pop(cMsg.GossipHash)

		// get original block
		block := nodeCtx.blockchain.popProposedBlock(cMsg.GossipHash)

		// create new final block
		finalBlock := new(FinalBlock)
		finalBlock.ProposedBlock = block
		finalBlock.Signatures = *consensusMsgs

		// add to blockchain
		nodeCtx.blockchain.add(finalBlock)
		nodeCtx.i.add()

		// process block
		finalBlock.processBlock(nodeCtx)

		// create cross-tx-responses and send
		for _, t := range finalBlock.ProposedBlock.Transactions {
			what := t.whatAmI(nodeCtx)
			if what == "crosstxresponse_C_in" {

				// we do not want to have PoC on final blocks that are in this committee
				// so make a copy

				newTx := new(Transaction)
				copier.Copy(newTx, t)

				addProofOfConsensus(nodeCtx, newTx, finalBlock)

				msg := Msg{"crosstransactionresponse", newTx, nodeCtx.self.Priv.Pub}
				go routeTx(nodeCtx, msg, txFindClosestCommittee(nodeCtx, newTx.OrigTxHash))
			}
		}

		// call coordinator and send transaction list, but only if you are leader
		if nodeCtx.amILeader() {
			fmt.Println("Final block: ", finalBlock.ProposedBlock)
			fmt.Printf("\n\nsent final block to coordinator\n\n")
			msg := Msg{"finalblock", finalBlock, nodeCtx.self.Priv.Pub}
			go dialAndSend(coord+":8080", msg)
		}

		// increase iteration

		log.Println("Accept sucess!")
		// start new iteration
		startNewIteration(nodeCtx)
	} else {
		// not enough accepts, terminate
		// TODO add coordinator feedback here

		bat := new(ByteArrayAndTimestamp)
		totV := make([]byte, 8)
		binary.LittleEndian.PutUint64(totV, uint64(totalVotes))
		iter := make([]byte, 8)
		binary.LittleEndian.PutUint64(iter, uint64(nodeCtx.i.getI()))
		// 32 32 8 8 8
		rec := make([]byte, 8)
		binary.LittleEndian.PutUint64(rec, uint64(recursive))

		bat.B = byteSliceAppend(nodeCtx.self.CommitteeID[:], nodeCtx.self.Priv.Pub.Bytes[:], iter[:], totV[:], rec[:])
		bat.T = time.Now() // dont need timestamp but why not
		go dialAndSendToCoordinator("consensus_accept_fail", bat)

		log.Println("Not enough votes ", totalVotes)
		recursive++
		if recursive >= 5 {
			startNewIteration(nodeCtx)
		} else {
			handleConsensusAccept(cMsg, nodeCtx, recursive)
		}
		return
	}

}
