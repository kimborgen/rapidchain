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
	allInfo, initialRandomness, currentCommittee := coordinatorSetup(conn, portNumber)
	initialRandomness = initialRandomness
	//fmt.Print(allInfo, initialRandomness, currentCommittee, "\n\n")

	// Build neighbour list
	currentNeighbours := buildCurrentNeighbours(flagArgs.d, &currentCommittee, allInfo.self.ID)

	nodeCtx := new(NodeCtx)
	nodeCtx.flagArgs = *flagArgs
	nodeCtx.committee = currentCommittee
	nodeCtx.neighbors = currentNeighbours
	nodeCtx.self = allInfo.self
	nodeCtx.allInfo = allInfo

	nodeCtx.idaMsgs = IdaMsgs{}
	nodeCtx.idaMsgs.init()

	nodeCtx.blocks = Blocks{}
	nodeCtx.blocks.init()

	nodeCtx.consensusMsgs = ConsensusMsgs{}
	nodeCtx.consensusMsgs.init()

	nodeCtx.channels = Channels{}
	nodeCtx.channels.init(len(currentCommittee.Members))

	nodeCtx.i = CurrentIteration{}

	// launch listener
	go listen(listener, nodeCtx)

	// launch leader election protocol
	var leaderID uint = leaderElection(&currentCommittee)

	// If this node is leader then initate ida-gossip
	if leaderID == allInfo.self.ID {
		leader(nodeCtx)
	}

	// blocker
	for {
	}
}

func coordinatorSetup(conn net.Conn, portNumber int) (AllInfo, int, Committee) {
	// setup with the help of coordinator

	// create a random ID
	ID := uint(rand.Intn(maxId))
	msg := Node_InitialMessageToCoordinator{ID, portNumber}
	sendMsg(conn, msg)

	// fmt.Printf("%d Waiting for return message\n", ID)

	response := new(ResponseToNodes)
	reciveMsg(conn, response)

	// declare variables to return
	var allInfo AllInfo
	var initialRandomness int
	var currentCommittee Committee

	allInfo.nodes = make(map[uint]NodeAllInfo)
	for _, elem := range response.Nodes {
		allInfo.nodes[elem.ID] = elem
	}
	allInfo.self = allInfo.nodes[ID]

	initialRandomness = response.InitalRandomness

	// create current commitee info
	currentCommittee.ID = allInfo.self.CommitteeID
	currentCommittee.Members = make(map[uint]CommitteeMember) // initialize map

	for k, v := range allInfo.nodes {
		if v.CommitteeID == currentCommittee.ID {
			// fmt.Println("\nk: ", k, " v: ", v)
			currentCommittee.Members[k] = CommitteeMember{v.ID, v.IP}
		}
	}

	//log.Printf("Coordinaton setup finished \n")
	return allInfo, initialRandomness, currentCommittee
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
		blockHeader, ok := msg.Msg.(BlockHeader)
		notOkErr(ok, "BlockHeader decoding")

		if i := nodeCtx.i.getI(); blockHeader.I >= i {
			nodeCtx.i.add()

			// empty channel
			for len(nodeCtx.channels.echoChan) > 0 {
				<-nodeCtx.channels.echoChan
			}
			go handleConsensusEcho(blockHeader, nodeCtx)
		}

		go handleConsensus(nodeCtx, blockHeader, msg.FromID)
	default:
		log.Fatal("[Error] no known message type")
	}

}

func buildCurrentNeighbours(d uint, currentCommittee *Committee, ownID uint) []uint {
	currentNeighbours := make([]uint, d)

	// sample list of neighbors
	indexes := randIndexesWithoutReplacement(len(currentCommittee.Members), int(d))

	i := 0 // index of this memeber
	c := 0 // index of neighbor
	for k := range currentCommittee.Members {
		for j := range indexes { // check if this members index is in indexe
			if i == j {
				// Now make sure that you are not part of this set.
				if k == ownID {
					// TODO get rid of this recursion
					return buildCurrentNeighbours(d, currentCommittee, ownID)
				}
				currentNeighbours[c] = k
				c += 1
			}
		}
		i += 1
	}
	return currentNeighbours
}

func leaderElection(currentCommittee *Committee) uint {
	// Find out who is the leader, returns ID of leader

	// For now just pick the one with the lowest ID.
	// TODO: create an actual leader election protocol based on epoch randomness and nonce, assume every node is online
	// TODO: figure out some complete leader election protocol

	var lowestID uint = uint(maxId) + 1
	for k := range currentCommittee.Members {
		if k < lowestID {
			lowestID = k
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
	header := new(BlockHeader)
	header.I = 0
	header.Root = merkleRoot
	header.LeaderID = nodeCtx.self.ID
	header.Tag = "propose"

	msg := Msg{"consensus", header, nodeCtx.self.ID}

	// start consensus rounds.
	log.Printf("Leader starting conseuss \n")
	sendMsgToCommittee(msg, &nodeCtx.committee)
}

func handleConsensus(
	nodeCtx *NodeCtx,
	header BlockHeader,
	fromID uint) {

	switch header.Tag {
	case "propose":
		fmt.Println("Recived header from leader", header)
		// TODO validate block with header

		// TODO check header actually comes from leader both by sig, and by election protocol

		// double check
		if header.LeaderID != fromID {
			errFatal(nil, "LeaderID not the same as FromID")
		}

		// check if we have recivied a message from same id in this iter and add if not
		nodeCtx.consensusMsgs.safeAdd(header.I, header.LeaderID, header, "leader block allready exists")

		// now that we have verified header then we should move on to round 2

		log.Println("sent echo")
		header.Tag = "echo"
		msg := Msg{"consensus", header, nodeCtx.self.ID}
		sendMsgToCommittee(msg, &nodeCtx.committee)

	case "echo":
		// wait for delta so each node will recive msg
		go func() {
			dur := default_delta * time.Millisecond
			time.Sleep(dur)
			log.Println("Echo recived from ", fromID)

			_msg := Msg{"consensus", "echo", nodeCtx.self.ID}
			go dialAndSend("127.0.0.1:8080", _msg)

			// TODO check valid header

			// TODO check if every header we have recived is unique
			// if not, then send special header with tag pending

			// if from id is leader id
			if fromID != header.LeaderID {
				// chck that we have not recived a header previously from the sender
				if nodeCtx.consensusMsgs.blockHeaderExists(header.I, fromID) {
					errFatal(nil, "allready recivied header")
				}
			}

			// fmt.Println((*consensusMsgs)[header.I][fromID])
			// add header to consensusMsgs
			nodeCtx.consensusMsgs.add(header.I, fromID, header)

			nodeCtx.channels.echoChan <- true
		}()

	case "pending":
		// don't accept this iteration

		// TODO check validity of header

		// TODO check that it is different from other recivied valid headers

		// set header of fromid to this pending, so accept round can check
		nodeCtx.consensusMsgs.add(header.I, fromID, header)

		_msg := Msg{"consensus", "pending", nodeCtx.self.ID}
		go dialAndSend("127.0.0.1:8080", _msg)
		// terminate without accepting
		return
	case "accept":
		// TODO check validity of header osv
		go func() {
			dur := default_delta * time.Millisecond
			time.Sleep(dur)

			nodeCtx.consensusMsgs.add(header.I, fromID, header)

			_msg := Msg{"consensus", "accept", nodeCtx.self.ID}
			go dialAndSend("127.0.0.1:8080", _msg)

			// todo add coordinator feedback here

			log.Println("Success, recived accept from ", fromID)
		}()
	default:
		errFatal(nil, "header tag not known")
	}
}

func handleConsensusEcho(
	header BlockHeader,
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
		msg := Msg{"consensus", header, nodeCtx.self.ID}
		sendMsgToCommittee(msg, &nodeCtx.committee)
	} else {
		// not enough votes, terminate
		// TODO add coordinator feedback here
		log.Println("Not enough votes ", totalVotes)
		return
	}

}
