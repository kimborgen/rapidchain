package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"
)

func launchNode(flagArgs *FlagArgs) {

	conn := dial("127.0.0.1:8080")
	// start listening. We do this here becuase we need to choose a unique port
	// number, and send that port number to coordinator so every node has correct port and ip
	listener, err := net.Listen("tcp", ":0")
	ifErrFatal(err, "listener node")
	portNumber := listener.Addr().(*net.TCPAddr).Port

	//fmt.Print(allInfo, initialRandomness, currentCommittee, "\n\n")

	nodeCtx := new(NodeCtx)
	nodeCtx.flagArgs = *flagArgs
	initialRandomness := coordinatorSetup(conn, portNumber, nodeCtx)
	initialRandomness = initialRandomness
	// launch listener
	go listen(listener, nodeCtx)

	// launch leader election protocol
	var leaderPub *PubKey = leaderElection(nodeCtx)

	fmt.Printf("Own: %x, leader: %x\n", nodeCtx.self.Priv.Pub.Bytes, leaderPub.Bytes)

	// If this node is leader then initate ida-gossip
	if leaderPub.Bytes == nodeCtx.self.Priv.Pub.Bytes {
		//fmt.Println("\n\n\n", nodeCtx.routingTable, "\n\n\n")

		leader(nodeCtx)

		/*
			time.Sleep(3 * time.Second)
			// fmt.Println("\n\n", nodeCtx.self.CommitteeID, "\n", nodeCtx.routingTable, "\n\n")
			// find a committee that is not in routing table
			for _, n := range nodeCtx.allInfo {
				var isIn bool = false
				if n.CommitteeID == nodeCtx.self.CommitteeID {
					isIn = true
				} else {
					for _, c := range nodeCtx.routingTable.l {
						if n.CommitteeID == c.ID {
							isIn = true
							break
						}
					}
				}
				if isIn {
					continue
				}
				// fmt.Println("\n\n\n", nodeCtx.self.CommitteeID, n.CommitteeID, "\n\n\n")
				c := findNode(nodeCtx, n.CommitteeID)
				log.Println("Found committee! ", c)
				break
			}
		*/
	}

	// blocker
	for {
	}
}

func listen(
	listener net.Listener,
	nodeCtx *NodeCtx) {

	// block main and listen to all incoming connections
	for {
		// accept new connection
		conn, err := listener.Accept()
		ifErrFatal(err, "tcp accept")

		// TODO do I need to have a mutex lock on the maps?
		go nodeHandleConnection(conn, nodeCtx)
	}
}

func nodeHandleConnection(
	conn net.Conn,
	nodeCtx *NodeCtx) {
	// decode the msg using the genereic Msg struct
	var msg Msg
	reciveMsg(conn, &msg)

	// determine msg type and msg struct using Msg.typ
	switch msg.Typ {
	case "IDAGossipMsg":
		idaMsg, ok := msg.Msg.(IDAGossipMsg)
		notOkErr(ok, "IDAGossipMsg decoding")
		go handleIDAGossipMsg(idaMsg, nodeCtx)

	case "consensus":
		blockHeader, ok := msg.Msg.(ConsensusBlockHeader)
		notOkErr(ok, "BlockHeader decoding")

		if i := nodeCtx.i.getI(); blockHeader.I >= i {
			nodeCtx.i.add()

			// empty channel
			for len(nodeCtx.channels.echoChan) > 0 {
				<-nodeCtx.channels.echoChan
			}
			go handleConsensusEcho(blockHeader, nodeCtx)
		}

		go handleConsensus(nodeCtx, blockHeader, msg.FromPub)
	case "find_node":
		kMsg, ok := msg.Msg.(KademliaFindNodeMsg)
		notOkErr(ok, "findNode decoding")

		if kMsg.ID == nodeCtx.self.CommitteeID {
			errFatal(nil, "Got a find_node msg but I am the target committee")
		}

		handleFindNode(nodeCtx, conn, kMsg)

	default:
		log.Fatal("[Error] no known message type")
	}

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

	log.Printf("Leader: %d", lowestID)
	return lowestID
}

func createBlock(B uint) []byte {
	// create a random bytearray for size B for now
	b := make([]byte, B)
	rand.Read(b)
	return b
}

func leader(nodeCtx *NodeCtx) {
	// initates leader process

	// create a block
	block := createBlock(nodeCtx.flagArgs.B)

	// ida-gossip the block
	merkleRoot := IDAGossip(nodeCtx, block)

	// we should wait untill IDA gossip finnishes...
	//dur := time.Duration(math.Log(float64(flagArgs.m)))
	//log.Println("sleeping for ", dur)
	//time.Sleep(dur)
	// wait until we have recivied and recreated IDA message

	for !nodeCtx.blocks.isBlock(merkleRoot) {
		time.Sleep(100 * time.Millisecond)
	}
	// sleep a delta before iniation consensus
	time.Sleep(default_delta * time.Millisecond)

	// create block header
	header := new(ConsensusBlockHeader)
	header.I = 0
	header.Root = merkleRoot
	header.LeaderPub = nodeCtx.self.Priv.Pub
	header.Tag = "propose"

	msg := Msg{"consensus", header, nodeCtx.self.Priv.Pub}

	// start consensus rounds.
	log.Printf("Leader starting conseuss \n")
	sendMsgToCommittee(msg, &nodeCtx.committee)
}

func handleConsensus(
	nodeCtx *NodeCtx,
	header ConsensusBlockHeader,
	fromPub *PubKey) {

	switch header.Tag {
	case "propose":
		fmt.Println("Recived header from leader", header)
		// TODO validate block with header

		// TODO check header actually comes from leader both by sig, and by election protocol

		// double check
		if header.LeaderPub.Bytes != fromPub.Bytes {
			errFatal(nil, "LeaderID not the same as FromID")
		}

		// check if we have recivied a message from same id in this iter and add if not
		nodeCtx.consensusMsgs.safeAdd(header.I, header.LeaderPub.Bytes, header, "leader block allready exists")

		// now that we have verified header then we should move on to round 2

		log.Println("sent echo")
		header.Tag = "echo"
		msg := Msg{"consensus", header, nodeCtx.self.Priv.Pub}
		sendMsgToCommittee(msg, &nodeCtx.committee)

	case "echo":
		// wait for delta so each node will recive msg
		go func() {
			dur := default_delta * time.Millisecond
			time.Sleep(dur)
			log.Println("Echo recived from ", fromPub)

			_msg := Msg{"consensus", "echo", nodeCtx.self.Priv.Pub}
			go dialAndSend("127.0.0.1:8080", _msg)

			// TODO check valid header

			// TODO check if every header we have recived is unique
			// if not, then send special header with tag pending

			// if from id is leader id
			if fromPub.Bytes != header.LeaderPub.Bytes {
				// chck that we have not recived a header previously from the sender
				if nodeCtx.consensusMsgs.blockHeaderExists(header.I, fromPub.Bytes) {
					errFatal(nil, "allready recivied header")
				}
			}

			// fmt.Println((*consensusMsgs)[header.I][fromID])
			// add header to consensusMsgs
			nodeCtx.consensusMsgs.safeAdd(header.I, fromPub.Bytes, header, "echo: block header allready exists")

			nodeCtx.channels.echoChan <- true
		}()

	case "pending":
		// don't accept this iteration

		// TODO check validity of header

		// TODO check that it is different from other recivied valid headers

		// set header of fromid to this pending, so accept round can check
		nodeCtx.consensusMsgs.add(header.I, fromPub.Bytes, header)

		_msg := Msg{"consensus", "pending", nodeCtx.self.Priv.Pub}
		go dialAndSend("127.0.0.1:8080", _msg)
		// terminate without accepting
		return
	case "accept":
		// TODO check validity of header osv
		go func() {
			dur := default_delta * time.Millisecond
			time.Sleep(dur)

			nodeCtx.consensusMsgs.add(header.I, fromPub.Bytes, header)

			_msg := Msg{"consensus", "accept", nodeCtx.self.Priv.Pub}
			go dialAndSend("127.0.0.1:8080", _msg)

			// todo add coordinator feedback here

			log.Println("Success, recived accept from ", fromPub)
		}()
	default:
		errFatal(nil, "header tag not known")
	}
}

func handleConsensusEcho(
	header ConsensusBlockHeader,
	nodeCtx *NodeCtx) {

	requiredVotes := (uint(len(nodeCtx.committee.Members)) / nodeCtx.flagArgs.committeeF) + 1

	// leader gossip, echo gossip
	time.Sleep(2 * default_delta * time.Millisecond)

	if len(nodeCtx.channels.echoChan) < int(requiredVotes) {
		//  wait a few ms to be sure (computing)
		timeout := 0
		for len(nodeCtx.channels.echoChan) < int(requiredVotes) {
			time.Sleep(10 * time.Millisecond)
			timeout += 1
			if t := default_delta / (10 * 2); timeout >= t {
				errr(nil, fmt.Sprintf("Echos not recived in time %d", t))
				return
			}
		}
	}

	// check if some header is pending
	for _, v := range nodeCtx.consensusMsgs.getBlockHeaders(header.I) {
		if v.Tag == "pending" {
			errr(nil, "some header was pending")
			return
		}
	}

	// check if we have enough required votes
	totalVotes := uint(0)
	for _, v := range nodeCtx.consensusMsgs.getBlockHeaders(header.I) {
		if v.Tag == "echo" || v.Tag == "accept" {
			totalVotes += 1
		}
	}

	// TODO change to flagArgs
	if totalVotes >= requiredVotes+uint(1) {
		// enough votes, send accept
		header.Tag = "accept"
		msg := Msg{"consensus", header, nodeCtx.self.Priv.Pub}
		sendMsgToCommittee(msg, &nodeCtx.committee)
	} else {
		// not enough votes, terminate
		// TODO add coordinator feedback here
		log.Println("Not enough votes ", totalVotes)
		return
	}

}
