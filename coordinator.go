package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"
)

type InitialMessageToCoordinator struct {
	id uint
	ip string
}

type committeeInfo struct {
	id  uint
	npm uint
	f   int
}

func launchCoordinator(n, m, totalF, committeeF uint) {
	/*
		The coordinator should listen to incoming connections untill it has recived n different ids
		Then it should create:
			a map of identity <-> ip
			a map of committee <-> identity
			a map of identity <-> honest or malicious.
			an initial randomness
		These variables should then be sent to every node.
	*/

	// To be used to send ID and IP from node connection to coordinator
	chanToCoordinator := make(chan InitialMessageToCoordinator, n)

	// To be used to send result back to node connection
	chanToNodes := make([]chan ResponseToNodes, n)
	for i := uint(0); i < n; i++ {
		chanToNodes[i] = make(chan ResponseToNodes)
	}

	// waitgroup for all node connections to have recived an ID
	var wg sync.WaitGroup
	wg.Add(int(n))

	// waitgroup for when coordinator is done and sent all data to connections
	var wg_done sync.WaitGroup
	wg_done.Add(int(n))

	go coordinator(chanToCoordinator, chanToNodes, &wg, n, m, totalF, committeeF)

	listener, err := net.Listen("tcp", ":8080")
	ifErrFatal(err, "tcp listen on port 8080")

	var i uint = 0

	// block main and listen to all incoming connections
	for i < n {

		// accept new connection
		conn, err := listener.Accept()
		ifErrFatal(err, "tcp accept")

		// spawn off goroutine to able to accept new connections
		go coordinatorHandleConnection(conn, chanToCoordinator, chanToNodes[i], &wg, &wg_done)

		if n > 20 && i%(n/10) == 0 {
			fmt.Printf("#connections: %d", i)
		}
		i += 1
	}

	wg_done.Wait()
	log.Println("Coordination executed")
}

func coordinatorHandleConnection(conn net.Conn,
	chanToCoordinator chan<- InitialMessageToCoordinator,
	chanFromCoordinator <-chan ResponseToNodes,
	wg, wg_done *sync.WaitGroup) {

	dec := gob.NewDecoder(conn)
	rec_msg := new(Node_InitialMessageToCoordinator)
	err := dec.Decode(rec_msg)
	ifErrFatal(err, "decoding")

	// get the remote address of the client
	clientAddr := conn.RemoteAddr().String()

	chanToCoordinator <- InitialMessageToCoordinator{rec_msg.ID, clientAddr}

	// signalize to waitgroup that this connection has recived an ID
	wg.Done()

	fmt.Println("waiting for returnMessage")
	returnMessage := <-chanFromCoordinator

	enc := gob.NewEncoder(conn)
	err = enc.Encode(returnMessage)
	ifErrFatal(err, "encoding")
	wg_done.Done()
}

func coordinator(
	chanToCoordinator chan InitialMessageToCoordinator,
	chanToNodes []chan ResponseToNodes,
	wg *sync.WaitGroup,
	n, m, totalF, committeeF uint) {

	// wait untill all node connections have pushed an ID/IP to chan
	wg.Wait()
	close(chanToCoordinator)

	// create array of structs that has all info about a node and assign it id/ip
	nodeInfos := make([]NodeAllInfo, n)
	i := 0
	for elem := range chanToCoordinator {
		nodeInfos[i].ID = elem.id
		nodeInfos[i].IP = elem.ip
		i += 1
	}

	// shuffle the list
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(nodeInfos), func(i, j int) { nodeInfos[i], nodeInfos[j] = nodeInfos[j], nodeInfos[i] })

	// Create committees with id
	committees := make([]uint, m)
	for i := uint(0); i < m; i++ {
		committees[i] = uint(rand.Intn(maxId))
	}

	// divide idIdPairs into equal m chunks and assign them to the committees
	npm := int(n / m)
	rest := int(n % m)
	c := 0
	for i := int(0); i < int(n); i++ {
		t_npm := npm
		if i != 0 && i%npm == 0 {
			if i != int(n)-rest { // if there is a rest, it will be put into the last committee
				c++
			} else {
				t_npm += rest
			}
		}
		nodeInfos[i].CommitteeID = committees[c]

		// Every other committee should have 1/2 -1 adversaries and the other have 1/6 -1
		// This way we achive 1/3 total resiliency
		// first committee aka ref c should have 1/2 -1 f
		var c_div int
		if c%2 == 0 {
			c_div = 3
		} else {
			c_div = 1
		}
		// TODO: this is not variable with committeeF

		// amount of adversaries in this committee
		f := (t_npm / (6 / c_div))
		// if the above division created exactly 50% adversaries then we subtract one
		if t_npm%(6/c_div) == 0 {
			f--
		}

		if i%t_npm < f {
			nodeInfos[i].IsHonest = false
		} else {
			nodeInfos[i].IsHonest = true
		}
	}

	// double check amount of nodes in each committee and their adversaries
	lastCommittee := committees[0]
	committeeInfos := make([]committeeInfo, m)
	iCommittee := 0
	for i := int(0); i < int(n); i++ {
		if nodeInfos[i].CommitteeID != lastCommittee {
			lastCommittee = nodeInfos[i].CommitteeID
			iCommittee += 1
		}
		committeeInfos[iCommittee].id = nodeInfos[i].CommitteeID
		committeeInfos[iCommittee].npm += 1
		if !nodeInfos[i].IsHonest {
			committeeInfos[iCommittee].f += 1
		}
	}

	fmt.Println("Committee info: ", committeeInfos)

	// check that invariants are held
	checkTotalF := 0
	for i := 0; i < len(committeeInfos); i++ {
		if committeeInfos[i].npm != uint(npm) && committeeInfos[i].npm != uint(npm+rest) {
			log.Fatal("Number of nodes in committee not right", npm, npm+rest, committeeInfos[i].npm)
		}

		if committeeInfos[i].f >= int(math.Ceil(float64(committeeInfos[i].npm)/float64(committeeF))) {
			log.Fatal("Comitte %d has too many adversaries %d", committeeInfos[i].id, committeeInfos[i].f)
		}

		checkTotalF += committeeInfos[i].f
	}

	if int(n)/checkTotalF < 1/int(totalF) {
		log.Fatal("There was too many adversaries in total %d", checkTotalF)
	}

	fmt.Println("Total adversary percentage: ", float64(checkTotalF)/float64(n))

	rnd := rand.Intn(maxId)
	msg := ResponseToNodes{nodeInfos, rnd}

	for _, c := range chanToNodes {
		c <- msg
	}
}