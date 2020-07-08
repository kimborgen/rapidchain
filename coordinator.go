package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type InitialMessageToCoordinator struct {
	pub *PubKey
	ip  string
}

type committeeInfo struct {
	id  [32]byte
	npm uint
	f   int
}

type consensusResult struct {
	echos, pending, accepts int
}

// measure routing of tx
type routetxresults struct {
	start      time.Time              // first node recives transaction
	end        time.Time              // first node in target committee recives tx
	committees map[[32]byte]time.Time // first node in intermediary committee recives tx
	// mux        sync.Mutex
}

func (r *routetxresults) init() {
	r.committees = make(map[[32]byte]time.Time)
}

// adds a committee with timestamp if it does not allready exist
func (r *routetxresults) add(cId [32]byte, tim time.Time) bool {
	if r.committees == nil {
		r.init()
	} else if _, ok := r.committees[cId]; ok {
		return false
	}
	r.committees[cId] = tim
	return true
}

// adds start timestamp only if it has not been added before
func (r *routetxresults) addStart(tim time.Time) bool {
	if r.start.IsZero() {
		r.start = tim
		return true
	}
	return false
}

// adds end timestamp only if it has not been added before
func (r *routetxresults) addEnd(tim time.Time) bool {
	if r.end.IsZero() {
		r.end = tim
		return true
	}
	return false
}

func launchCoordinator(flagArgs *FlagArgs) {
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
	chanToCoordinator := make(chan InitialMessageToCoordinator, flagArgs.n)

	// To be used to send result back to node connection
	chanToNodes := make([]chan ResponseToNodes, flagArgs.n)
	for i := uint(0); i < flagArgs.n; i++ {
		chanToNodes[i] = make(chan ResponseToNodes)
	}

	// waitgroup for all node connections to have recived an ID
	var wg sync.WaitGroup
	wg.Add(int(flagArgs.n))

	// waitgroup for when coordinator is done and sent all data to connections
	var wg_done sync.WaitGroup
	wg_done.Add(int(flagArgs.n))

	rand.Seed(1337)

	finalBlockChan := make(chan FinalBlock, flagArgs.m*2)

	var err error

	// result files
	files := make([]*os.File, 4)
	files[0], err = os.Create("results/tx" + time.Now().String() + ".csv")
	ifErrFatal(err, "txresfile")
	files[1], err = os.Create("results/pocverify" + time.Now().String() + ".csv")
	ifErrFatal(err, "pocverifyfile")
	files[2], err = os.Create("results/pocadd" + time.Now().String() + ".csv")
	ifErrFatal(err, "pocaddfile")
	files[3], err = os.Create("results/routing" + time.Now().String() + ".csv")
	ifErrFatal(err, "routing")
	for _, f := range files {
		defer f.Close()
	}

	go coordinator(chanToCoordinator, chanToNodes, &wg, flagArgs, finalBlockChan, files)

	listener, err := net.Listen("tcp", ":8080")
	ifErrFatal(err, "tcp listen on port 8080")

	var i uint = 0

	// block main and listen to all incoming connections
	for i < flagArgs.n {

		// accept new connection
		conn, err := listener.Accept()
		ifErrFatal(err, "tcp accept")

		// spawn off goroutine to able to accept new connections
		go coordinatorHandleConnection(conn, chanToCoordinator, chanToNodes[i], &wg, &wg_done)

		if flagArgs.n > 20 && i%(flagArgs.n/10) == 0 {
			fmt.Printf("#connections: %d", i)
		}
		i += 1
	}

	wg_done.Wait()
	log.Println("Coordination executed")

	// merkleroot -> number of nodes succesfully recreated it
	successfullGossips := make(map[[32]byte]int)

	// committee -> iteration -> echo, pending, accept messages
	consensusResults := new(consensusResult)

	// routetx map
	// txid -> committeeid ->
	routetxmap := make(map[[32]byte]*routetxresults)

	// start listening for debug/stats
	for {
		// accept new connection
		conn, err := listener.Accept()
		ifErrFatal(err, "tcp accept")
		// spawn off goroutine to able to accept new connections
		go coordinatorDebugStatsHandleConnection(conn, &successfullGossips, consensusResults, finalBlockChan, files, routetxmap)
	}
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
	// remove port number and add rec_msg.Port instead
	//fmt.Println("1: ", clientAddr)
	clientAddr = fmt.Sprintf("%s:%d", clientAddr[:strings.IndexByte(clientAddr, ':')], rec_msg.Port)
	//fmt.Println("2: ", clientAddr)

	chanToCoordinator <- InitialMessageToCoordinator{rec_msg.Pub, clientAddr}

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
	flagArgs *FlagArgs,
	finalBlockChan chan FinalBlock,
	files []*os.File) {

	// wait untill all node connections have pushed an ID/IP to chan
	wg.Wait()
	close(chanToCoordinator)

	// create array of structs that has all info about a node and assign it id/ip
	nodeInfos := make([]NodeAllInfo, flagArgs.n)
	i := 0
	for elem := range chanToCoordinator {
		nodeInfos[i].Pub = elem.pub
		nodeInfos[i].IP = elem.ip
		i += 1
	}

	// shuffle the list
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(nodeInfos), func(i, j int) { nodeInfos[i], nodeInfos[j] = nodeInfos[j], nodeInfos[i] })

	// Create committees with id
	committees := make([][32]byte, flagArgs.m)
	for i := uint(0); i < flagArgs.m; i++ {
		committees[i] = hash(getBytes(rand.Intn(maxId)))
	}

	fmt.Println("Committees: ", committees)

	// divide idIdPairs into equal m chunks and assign them to the committees
	npm := int(flagArgs.n / flagArgs.m)
	rest := int(flagArgs.n % flagArgs.m)
	c := 0
	for i := int(0); i < int(flagArgs.n); i++ {
		t_npm := npm
		if i != 0 && i%npm == 0 {
			if i != int(flagArgs.n)-rest { // if there is a rest, it will be put into the last committee
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
	committeeInfos := make([]committeeInfo, flagArgs.m)
	iCommittee := 0
	for i := int(0); i < int(flagArgs.n); i++ {
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

		if committeeInfos[i].f >= int(math.Ceil(float64(committeeInfos[i].npm)/float64(flagArgs.committeeF))) {
			log.Fatal("Comitte %d has too many adversaries %d", committeeInfos[i].id, committeeInfos[i].f)
		}

		checkTotalF += committeeInfos[i].f
	}

	if flagArgs.n/flagArgs.m != 1 && int(flagArgs.n)/checkTotalF < 1/int(flagArgs.totalF) {
		log.Fatal("There was too many adversaries in total %d", checkTotalF)
	}

	fmt.Println("Total adversary percentage: ", float64(checkTotalF)/float64(flagArgs.n))

	// gen set of idenetites
	users := genUsers(flagArgs)
	genesisBlocks := genGenesisBlock(flagArgs, committeeInfos, users)

	// create reconfiguration block
	rBlock := new(ReconfigurationBlock)
	rBlock.init()
	for _, committeeInfo := range committeeInfos {
		newCom := new(Committee)
		newCom.init(committeeInfo.id)
		for _, node := range nodeInfos {
			if node.CommitteeID == newCom.ID {
				tmp := new(CommitteeMember)
				tmp.Pub = node.Pub
				tmp.IP = node.IP
				newCom.addMember(tmp)
			}
		}
		rBlock.Committees[newCom.ID] = newCom
	}
	// create initial randomness
	rnd := make([]byte, 32)
	rand.Read(rnd)
	rBlock.Randomness = hash(rnd)
	rBlock.setHash()

	msg := ResponseToNodes{nodeInfos, genesisBlocks, nodeInfos[0].Pub.Bytes, rBlock}

	for _, c := range chanToNodes {
		c <- msg
	}

	txGenerator(flagArgs, nodeInfos, users, genesisBlocks, finalBlockChan, files)
}

func prepareResultString(s string) string {
	tmp := strconv.FormatInt(time.Now().Unix(), 10)
	tmp += ","
	tmp += s
	tmp += "\n"
	return tmp
}

func writeIntToFile(integer int64, f *os.File) {

	s := prepareResultString(strconv.FormatInt(integer, 10))

	f.WriteString(s)
	f.Sync()
}

func writeStringToFile(s string, f *os.File) {

	newS := prepareResultString(s)

	f.WriteString(newS)
	f.Sync()
}

func coordinatorDebugStatsHandleConnection(conn net.Conn,
	successfullGossips *map[[32]byte]int,
	consensusResults *consensusResult,
	finalBlockChan chan FinalBlock,
	files []*os.File,
	rMap map[[32]byte]*routetxresults) {
	msg := new(Msg)
	reciveMsg(conn, msg)
	switch msg.Typ {
	case "IDASuccess":
		_, ok := msg.Msg.([32]byte)
		if !ok {
			errFatal(ok, "IDASuccess decoding")
		}
		//coordinatorHandleIDASuccess(idaMsg, successfullGossips)

	case "consensus":
		_, ok := msg.Msg.(string)
		notOkErr(ok, "coordinator consensus cMsg decoding")
		//coordinatorHandleConsensus(cMsg, consensusResults)
	case "finalblock":
		block, ok := msg.Msg.(FinalBlock)
		notOkErr(ok, "finalblock")
		finalBlockChan <- block
	case "pocverify":
		dur, ok := msg.Msg.(time.Duration)
		notOkErr(ok, "pocverify")
		writeIntToFile(dur.Nanoseconds(), files[1])
	case "pocadd":
		dur, ok := msg.Msg.(time.Duration)
		notOkErr(ok, "pocadd")
		writeIntToFile(dur.Nanoseconds(), files[2])
	case "routetx":
		tx, ok := msg.Msg.(ByteArrayAndTimestamp)
		notOkErr(ok, "routtx")
		ID := toByte32(tx.B)
		if rMap[ID] == nil {
			rMap[ID] = new(routetxresults)
			rMap[ID].init()
		}
		rMap[ID].addStart(tx.T)
	case "find_node":
		tuple, ok := msg.Msg.(ByteArrayAndTimestamp)
		notOkErr(ok, "find_node")
		txid := toByte32(tuple.B[:32])
		committeeID := toByte32(tuple.B[32:])
		if rMap[txid] == nil {
			rMap[txid] = new(routetxresults)
			rMap[txid].init()
		}
		rMap[txid].add(committeeID, tuple.T)
	case "transaction_recieved":
		bat, ok := msg.Msg.(ByteArrayAndTimestamp)
		notOkErr(ok, "find_node")
		ID := toByte32(bat.B)
		if rMap[ID] == nil {
			rMap[ID] = new(routetxresults)
			rMap[ID].init()
		}
		ok = rMap[ID].addEnd(bat.T)
		if ok {
			// sleep for a delta to let incomming request be processed
			time.Sleep(default_delta * 3 * time.Millisecond)
			var s string
			if rMap[ID].start.IsZero() {
				s += "0"
			} else {
				s += strconv.FormatInt(rMap[ID].start.Unix(), 10)
			}
			s += ","
			s += strconv.FormatInt(rMap[ID].end.Unix(), 10)
			for cID, tStamp := range rMap[ID].committees {
				s += ","
				s += bytes32ToString(cID)
				s += ","
				s += strconv.FormatInt(tStamp.Unix(), 10)
			}
			writeStringToFile(s, files[3])
		}
	default:
		errFatal(nil, "no known message type (coordinator)")
	}
}

func coordinatorHandleIDASuccess(root [32]byte, successfullGossips *map[[32]byte]int) {
	/*
		(*successfullGossips)[root] += 1
		if (*successfullGossips)[root] >= int(default_n/default_m) {
			// this is not perfect, but it will atleast show if all nodes recived a successfull ida msg
			log.Println("IDAGossip success for root ", root, "with ", (*successfullGossips)[root], " nodes succesfull")
		}
	*/
}

func coordinatorHandleConsensus(tag string, consensusResults *consensusResult) {

	tmp := (*consensusResults)
	switch tag {
	case "echo":
		tmp.echos += 1
	case "pending":
		tmp.pending += 1
	case "accept":
		tmp.accepts += 1
	}
	(*consensusResults) = tmp
	// TODO fix this to handle multiple committees
	/*
		if v := uint((default_n/default_m)/default_committeeF) + uint(1); uint(tmp.accepts) >= v*v {
			log.Println(tmp.accepts, " accepts")
		} else if v*v <= uint(tmp.pending) {
			log.Println(tmp.pending, " pendings")
		} else if v*v <= uint(tmp.echos) {
			log.Println(tmp.echos, " echos")
		}
	*/

}
