package main

import (
	"log"
	"math/rand"
	"net"
)

type CommitteeMember struct {
	ID uint
	IP uint
}

type Committee struct {
	ID      uint
	members []CommitteeMember
}

var nodesAllInfo []NodeAllInfo
var selfAllInfo NodeAllInfo
var initialRandomness int
var currentCommittee Committee

// information about yourself

func launchNode() {
	conn := dial("127.0.0.1:8080")
	coordinatorSetup(conn)
}

func coordinatorSetup(conn net.Conn) {
	// setup with the help of coordinator

	// create a random ID
	ID := uint(rand.Intn(maxId))
	msg := Node_InitialMessageToCoordinator{ID}
	sendMsg(conn, msg)

	// fmt.Printf("%d Waiting for return message\n", ID)

	response := new(ResponseToNodes)
	reciveMsg(conn, response)

	nodesAllInfo = response.Nodes
	initialRandomness = response.InitalRandomness

	// go trough the list and find yourself
	// also double check if there is another node with same id
	// TODO: remove double check when pk scheme is implemented
	found := false

	/*
		for _, elem := range nodesAllInfo {
			if elem.ID == ID {
				if !found {
					selfAllInfo = elem
					found = true
				} else {
					log.Fatal("Found other node with same id")
				}
			}
		}
	*/

	log.Printf("Coordinaton setup finished \n")
}
