package main

import (
	"log"
	"time"

	"github.com/kimborgen/go-merkletree"
)

// createBlock pops transaction from transaction pool and creates a new block
func createProposeBlock(nodeCtx *NodeCtx) *ProposedBlock {

	block := new(ProposedBlock)

	// block all blockchain operations while creating new block
	nodeCtx.blockchain.mux.Lock()

	// set previous block hash
	block.PreviousGossipHash = nodeCtx.blockchain.LatestBlock

	// block tx pool operations
	nodeCtx.txPool.mux.Lock()

	// get all transactions
	// TODO limit the amout of transactions to be included
	txes := nodeCtx.txPool._popAll()
	// assumes all txes signatures are correct

	// TODO actually go trough txes, verify that they are correct, create cross-tx and so on
	// for now assume that all txes input is in this committee
	block.Transactions = txes

	block.Iteration = nodeCtx.i.getI()
	block.CommitteeID = nodeCtx.self.CommitteeID
	block.LeaderPub = nodeCtx.self.Priv.Pub

	// create merkle tree of transactions
	data := make([][]byte, len(txes))
	for i := range data {
		data[i] = txes[i].encode()
	}
	tree, err := merkletree.New(data)
	ifErrFatal(err, "Could not create transaction merkle tree")
	block.MerkleRoot = toByte32(tree.Root())

	// set hash
	block.setHash()

	// sign hash
	block.LeaderSig = nodeCtx.self.Priv.sign(block.GossipHash)

	// unlock locked mutexes
	nodeCtx.txPool.mux.Unlock()
	nodeCtx.blockchain.mux.Unlock()
	return block
}

func leaderElection(nodeCtx *NodeCtx) *PubKey {
	// Find out who is the leader, returns ID of leader
	// For now just pick the one with the lowest ID.
	// TODO: create an actual leader election protocol based on epoch randomness and nonce, assume every node is online
	// TODO: figure out some complete leader election protocol

	// set yourself to the lowest seen
	var lowestID *PubKey = nodeCtx.self.Priv.Pub
	lowestIDBigInt := toBigInt(lowestID.Bytes)

	for k := range nodeCtx.committee.Members {
		kBI := toBigInt(k)
		if kBI.Cmp(lowestIDBigInt) < 0 {
			lowestID = nodeCtx.committee.Members[k].Pub
			lowestIDBigInt = kBI
		}
	}

	if true {
		// go trough all nodes in system and give the lowest id instead of lowest id in committee
		for k := range nodeCtx.allInfo {
			kBI := toBigInt(k)
			if kBI.Cmp(lowestIDBigInt) < 0 {
				lowestID = nodeCtx.allInfo[k].Pub
				lowestIDBigInt = kBI
			}
		}
	}

	log.Printf("Leader: %d", lowestID)
	return lowestID
}

func leader(nodeCtx *NodeCtx) {
	// initates leader process

	// create a block
	block := createProposeBlock(nodeCtx)

	// ida-gossip the block
	IDAGossip(nodeCtx, block.encode(), "block")

	// wait until we have recivied and recreated IDA message
	for !nodeCtx.blockchain._isProposedBlock(block.GossipHash) {
		time.Sleep(100 * time.Millisecond)
	}

	// sleep a delta before iniation consensus
	time.Sleep(default_delta * time.Millisecond)

	// create a propose msg to initate consensus
	cMsg := new(ConsensusMsg)
	cMsg.GossipHash = block.GossipHash
	cMsg.Tag = "propose"
	cMsg.Pub = nodeCtx.self.Priv.Pub
	cMsg.sign(nodeCtx.self.Priv)

	msg := Msg{"consensus", cMsg, nodeCtx.self.Priv.Pub}

	// start consensus rounds.
	log.Printf("Leader starting conseuss \n")
	sendMsgToCommitteeAndSelf(msg, nodeCtx)
}
