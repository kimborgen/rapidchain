package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
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

	self := allInfo.self
	// launch listener
	go listen(listener, &currentCommittee, &currentNeighbours, &self)

	// launch leader election protocol
	var leaderID uint = leaderElection(&currentCommittee)

	// If this node is leader then initate ida-gossip
	if leaderID == allInfo.self.ID {
		leader(flagArgs, leaderID, &currentCommittee, &currentNeighbours)
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

type Channels struct {
	echoChan chan bool
}

// SafeCounter is safe to use concurrently.
type CurrentIteration struct {
	i   uint
	mux sync.Mutex
}

// Inc increments the counter for the given key.
func (c *CurrentIteration) add() {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	c.i++
	c.mux.Unlock()
}

// Value returns the current value of the counter for the given key.
func (c *CurrentIteration) getI() uint {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	return c.i
}

func listen(
	listener net.Listener,
	currentCommittee *Committee,
	currentNeighbours *[]uint,
	self *NodeAllInfo) {

	idaMsgs := make(map[[32]byte][]IDAGossipMsg)
	blocks := make(map[[32]byte][][]byte)
	consensusMsgs := make(map[uint]map[uint]BlockHeader) // iteration -> (id -> blockheader)

	channels := new(Channels)
	l := len((*currentCommittee).Members)
	channels.echoChan = make(chan bool, l)

	currentIteration := new(CurrentIteration)
	// block main and listen to all incoming connections
	for {
		// accept new connection
		conn, err := listener.Accept()
		ifErrFatal(err, "tcp accept")

		// TODO do I need to have a mutex lock on the maps?
		go nodeHandleConnection(conn, currentCommittee, currentNeighbours, self, &idaMsgs, &blocks, &consensusMsgs, channels, currentIteration)
	}
}

func nodeHandleConnection(
	conn net.Conn,
	currentCommittee *Committee,
	currentNeighbours *[]uint,
	self *NodeAllInfo,
	idaMsgs *map[[32]byte][]IDAGossipMsg,
	blocks *map[[32]byte][][]byte,
	consensusMsgs *map[uint]map[uint]BlockHeader,
	channels *Channels,
	currentIteration *CurrentIteration) {
	// decode the msg using the genereic Msg struct
	var msg Msg
	reciveMsg(conn, &msg)

	// determine msg type and msg struct using Msg.typ
	switch msg.Typ {
	case "IDAGossipMsg":
		idaMsg, ok := msg.Msg.(IDAGossipMsg)
		notOkErr(ok, "IDAGossipMsg decoding")

		go func() {
			root, block := handleIDAGossipMsg(idaMsg, idaMsgs, currentCommittee, currentNeighbours, self)
			if getLenOfChunks((*idaMsgs)[idaMsg.MerkleRoot]) > default_kappa {
				log.Printf("have enough blocks allready")
				return
			}
			if block != nil {
				if (*blocks)[root] != nil {
					errFatal(nil, "Already have a block for this root")
				}
				(*blocks)[root] = block
			}
		}()

	case "consensus":
		blockHeader, ok := msg.Msg.(BlockHeader)
		notOkErr(ok, "BlockHeader decoding")

		if i := currentIteration.getI(); blockHeader.I >= i {
			currentIteration.add()

			// empty channel
			for len(channels.echoChan) > 0 {
				<-channels.echoChan
			}
			go handleConsensusEcho(blockHeader, currentCommittee, self, consensusMsgs, channels)
		}

		go handleConsensus(blockHeader, msg.FromID, currentCommittee, currentNeighbours, self, blocks, consensusMsgs, channels)
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

func leader(flagArgs *FlagArgs, selfID uint, currentCommittee *Committee, currentNeighbours *[]uint) {
	// initates leader process

	// create a block
	block := createBlock(flagArgs.B)

	// ida-gossip the block
	merkleRoot := IDAGossip(flagArgs, currentCommittee, currentNeighbours, selfID, &block)

	// we should wait untill IDA gossip finnishes...
	//dur := time.Duration(math.Log(float64(flagArgs.m)))
	//log.Println("sleeping for ", dur)
	//time.Sleep(dur)
	time.Sleep(3 * time.Second)
	// create block header
	header := new(BlockHeader)
	header.I = 0
	header.Root = merkleRoot
	header.LeaderID = selfID
	header.Tag = "propose"

	msg := Msg{"consensus", header, selfID}

	// start consensus rounds.
	log.Printf("Leader starting conseuss \n", len((*currentCommittee).Members))
	sendMsgToCommittee(msg, currentCommittee)
}

func handleConsensus(
	header BlockHeader,
	fromID uint,
	currentCommittee *Committee,
	currentNeighbours *[]uint,
	self *NodeAllInfo,
	blocks *map[[32]byte][][]byte,
	consensusMsgs *map[uint]map[uint]BlockHeader,
	channels *Channels) {

	switch header.Tag {
	case "propose":
		fmt.Println("Recived header from leader", header)
		// TODO validate block with header

		// TODO check header actually comes from leader both by sig, and by election protocol

		// double check
		if header.LeaderID != fromID {
			errFatal(nil, "LeaderID not the same as FromID")
		}

		// check if we have recivied a message from same id in this iter
		if (*consensusMsgs)[header.I] == nil {
			// First message this iteration
			(*consensusMsgs)[header.I] = make(map[uint]BlockHeader)
			(*consensusMsgs)[header.I][header.LeaderID] = header
		} else {
			// just double check that we actually have a header
			if _, ok := (*consensusMsgs)[header.I][header.LeaderID]; !ok {
				//do something here
				errFatal(nil, "should have inserted header from leader allready")
			}

			// this means we have allready recivied the header from leader and should therefor terminate
			errFatal(nil, "Allready recived header from Leader of this iteration")
		}

		// now that we have verified header then we should move on to round 2

		log.Println("sent echo")
		header.Tag = "echo"
		msg := Msg{"consensus", header, self.ID}
		sendMsgToCommittee(msg, currentCommittee)

	case "echo":
		// wait for delta so each node will recive msg
		go func() {
			dur := default_delta * time.Millisecond
			time.Sleep(dur)
			log.Println("Echo recived from ", fromID)

			_msg := Msg{"consensus", "echo", self.ID}
			go dialAndSend("127.0.0.1:8080", _msg)

			// TODO check valid header

			// TODO check if every header we have recived is unique
			// if not, then send special header with tag pending

			// if from id is leader id
			if fromID != header.LeaderID {
				// chck that we have not recived a header previously from the sender
				if _, ok := (*consensusMsgs)[header.I][fromID]; ok {
					errFatal(nil, "allready recivied header")
				}
			}

			// fmt.Println((*consensusMsgs)[header.I][fromID])
			// add header to consensusMsgs
			(*consensusMsgs)[header.I][fromID] = header

			channels.echoChan <- true
		}()

	case "pending":
		// don't accept this iteration

		// TODO check validity of header

		// TODO check that it is different from other recivied valid headers

		// set header of fromid to this pending, so accept round can check
		(*consensusMsgs)[header.I][fromID] = header

		_msg := Msg{"consensus", "pending", self.ID}
		go dialAndSend("127.0.0.1:8080", _msg)
		// terminate without accepting
		return
	case "accept":
		// TODO check validity of header osv
		go func() {
			dur := default_delta * time.Millisecond
			time.Sleep(dur)

			(*consensusMsgs)[header.I][fromID] = header

			_msg := Msg{"consensus", "accept", self.ID}
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
	currentCommittee *Committee,
	self *NodeAllInfo,
	consensusMsgs *map[uint]map[uint]BlockHeader,
	channels *Channels) {

	requiredVotes := (uint(len((*currentCommittee).Members)) / default_committeeF) + 1

	// leader gossip, echo gossip
	time.Sleep(2 * default_delta * time.Millisecond)

	if len(channels.echoChan) < int(requiredVotes) {
		//  wait a few ms to be sure (computing)
		timeout := 0
		for len(channels.echoChan) < int(requiredVotes) {
			time.Sleep(10 * time.Millisecond)
			timeout += 1
			if t := default_delta / (10 * 2); timeout >= t {
				errr(nil, fmt.Sprintf("Echos not recived in time %d", t))
				return
			}
		}
	}

	// check if some header is pending
	for _, v := range (*consensusMsgs)[header.I] {
		if v.Tag == "pending" {
			errr(nil, "some header was pending")
			return
		}
	}

	// check if we have enough required votes
	totalVotes := uint(0)
	for _, v := range (*consensusMsgs)[header.I] {
		if v.Tag == "echo" || v.Tag == "accept" {
			totalVotes += 1
		}
	}

	// TODO change to flagArgs
	if totalVotes >= requiredVotes+uint(1) {
		// enough votes, send accept
		header.Tag = "accept"
		msg := Msg{"consensus", header, self.ID}
		sendMsgToCommittee(msg, currentCommittee)
	} else {
		// not enough votes, terminate
		// TODO add coordinator feedback here
		log.Println("Not enough votes ", totalVotes)
		return
	}

}
