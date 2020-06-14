package main

import (
	"log"
	"math/rand"
	"net"
)

// Representation of a member beloning to the current committee of a node
type CommitteeMember struct {
	ID uint
	IP string
}

// Representation of a committee from the point of view of a node
type Committee struct {
	ID      uint
	Members map[uint]CommitteeMember
}

// Cheat data to know all nodes in the system
type AllInfo struct {
	self  NodeAllInfo
	nodes map[uint]NodeAllInfo
}

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

	// launch listener
	go listen(listener, &currentCommittee, &currentNeighbours)

	// launch leader election protocol
	var leaderID uint = leaderElection(&currentCommittee)

	// If this node is leader then initate ida-gossip
	if leaderID == allInfo.self.ID {
		leader(flagArgs, &currentCommittee, &currentNeighbours)
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

func listen(listener net.Listener, currentCommittee *Committee, currentNeighbours *[]uint) {
	idaMsgs := make(map[[32]byte][]IDAGossipMsg)

	// block main and listen to all incoming connections
	for {
		// accept new connection
		conn, err := listener.Accept()
		ifErrFatal(err, "tcp accept")

		go nodeHandleConnection(conn, currentCommittee, currentNeighbours, &idaMsgs)
	}
}

func nodeHandleConnection(
	conn net.Conn,
	currentCommittee *Committee,
	currentNeighbours *[]uint,
	idaMsgs *map[[32]byte][]IDAGossipMsg) {
	// decode the msg using the genereic Msg struct
	var msg Msg
	reciveMsg(conn, &msg)

	// determine msg type and msg struct using Msg.typ
	switch msg.Typ {
	case "IDAGossipMsg":
		idaMsg, ok := msg.Msg.(IDAGossipMsg)
		if !ok {
			errFatal(ok, "IDAGossipMsg decoding")
		}
		handleIDAGossipMsg(idaMsg, idaMsgs, currentCommittee, currentNeighbours)

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

func leader(flagArgs *FlagArgs, currentCommittee *Committee, currentNeighbours *[]uint) {
	// initates leader process

	// create a block
	block := createBlock(flagArgs.B)

	// ida-gossip the block
	IDAGossip(flagArgs, currentCommittee, currentNeighbours, &block)

	// consensus on the merkle-root of the block
}
