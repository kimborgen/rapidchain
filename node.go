package main

import (
	"fmt"
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

func launchNode() {
	conn := dial("127.0.0.1:8080")
	allInfo, initialRandomness, currentCommittee := coordinatorSetup(conn)
	fmt.Print(allInfo, initialRandomness, currentCommittee, "\n\n")

	// launch listener
	go listen()

	// blocker
	for {
	}
}

func coordinatorSetup(conn net.Conn) (AllInfo, int, Committee) {
	// setup with the help of coordinator

	// create a random ID
	ID := uint(rand.Intn(maxId))
	msg := Node_InitialMessageToCoordinator{ID}
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
		if v.CommitteeID == currentCommittee.ID && k != ID {
			fmt.Println("\nk: ", k, " v: ", v)
			currentCommittee.Members[k] = CommitteeMember{v.ID, v.IP}
		}
	}

	log.Printf("Coordinaton setup finished \n")
	return allInfo, initialRandomness, currentCommittee
}

func listen() {
	listener, err := net.Listen("tcp", ":8080")
	ifErrFatal(err, "tcp listen on port 8080")

	// block main and listen to all incoming connections
	for {

		// accept new connection
		conn, err := listener.Accept()
		ifErrFatal(err, "tcp accept")

		go nodeHandleConnection(conn)
		// spawn off goroutine to able to accept new connections
		//go coordinatorHandleConnection(conn, chanToCoordinator, chanToNodes[i], &wg, &wg_done)
	}
}

func nodeHandleConnection(conn net.Conn) {

}
