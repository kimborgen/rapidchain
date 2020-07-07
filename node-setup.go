package main

import (
	"fmt"
	"math"
	"math/big"
	"net"
	"sort"
)

func coordinatorSetup(conn net.Conn, portNumber int, nodeCtx *NodeCtx) {
	// setup with the help of coordinator

	// generate a key
	privKey := new(PrivKey)
	privKey.gen()

	msg := Node_InitialMessageToCoordinator{privKey.Pub, portNumber}

	sendMsg(conn, msg)

	// fmt.Printf("%d Waiting for return message\n", ID)

	response := new(ResponseToNodes)
	reciveMsg(conn, response)

	// declare variables to return
	allInfo := make(map[[32]byte]NodeAllInfo)
	var selfInfo SelfInfo
	var currentCommittee Committee
	var routingTable RoutingTable

	for _, elem := range response.Nodes {
		allInfo[elem.Pub.Bytes] = elem
	}

	// set selfInfo
	selfInfo.Priv = privKey
	selfInfo.CommitteeID = allInfo[privKey.Pub.Bytes].CommitteeID
	selfInfo.IP = allInfo[privKey.Pub.Bytes].IP
	selfInfo.IsHonest = allInfo[privKey.Pub.Bytes].IsHonest
	if response.DebugNode == selfInfo.Priv.Pub.Bytes {
		selfInfo.Debug = true
	} else {
		selfInfo.Debug = false
	}

	// delete self from allInfo
	delete(allInfo, privKey.Pub.Bytes)

	// create current commitee info
	currentCommittee.init(selfInfo.CommitteeID)

	for k, v := range allInfo {
		if v.CommitteeID == currentCommittee.ID {
			tmp := new(CommitteeMember)
			tmp.IP = v.IP
			tmp.Pub = v.Pub
			currentCommittee.Members[k] = tmp
		}
	}

	// create routing table,
	length := math.Ceil(math.Log(float64(nodeCtx.flagArgs.m))) + 1

	// check if m is power of two:
	if (nodeCtx.flagArgs.m & (nodeCtx.flagArgs.m - 1)) != 0 {
		// if it, increase length by one to get the last committee in routing table
		length++
	}
	//log.Println("Routingtable length: ", length, int(length))

	routingTable.init(int(length))

	committees := make(map[[32]byte]bool)
	// get a list of committees
	for _, node := range allInfo {
		// exclude own committee
		if selfInfo.CommitteeID == node.CommitteeID {
			continue
		}
		committees[node.CommitteeID] = true
	}

	// generate committeeList with own committee
	committeeList := make([][32]byte, len(committees)+1)
	committeeList[0] = selfInfo.CommitteeID

	iC := 1
	for k := range committees {
		committeeList[iC] = k
		iC++
	}
	nodeCtx.committeeList = committeeList

	selfCommitteeID := new(big.Int).SetBytes(selfInfo.CommitteeID[:])

	xored := make([]*big.Int, len(committees))
	// sort committes after some distance metric (XOR kademlia)
	i := 0
	for k := range committees {
		// bitwise XOR
		xored[i] = new(big.Int)
		xored[i].Xor(selfCommitteeID, new(big.Int).SetBytes(k[:]))
		i++
	}
	// now we sort by increasing value since the closest ids are the ones with the longest leading zero
	sort.Slice(xored, func(i, j int) bool { return xored[i].Cmp(xored[j]) < 0 })

	/*
		for _, x := range xored {
			fmt.Println(selfInfo.CommitteeID, "   ", selfInfo.CommitteeID^x, "   ", x)
		}
	*/

	kademliaCommittees := [][32]byte{}
	// pick committees in 2^i distances
	for i := uint(0); true; i++ {
		dist := int(math.Pow(2, float64(i))) - 1
		if dist >= len(xored) {
			dist = len(xored) - 1
		}

		// we need to xor the xored to get the original id
		app := toByte32(new(big.Int).Xor(selfCommitteeID, xored[dist]).Bytes())
		kademliaCommittees = append(kademliaCommittees, app)
		routingTable.addCommittee(i, app)
		if dist == len(xored)-1 {
			break
		}
	}

	// get all nodes from these committees
	nodesInKadamliaCommittees := make(map[[32]byte][]NodeAllInfo)
	for _, k := range kademliaCommittees {
		nodesInKadamliaCommittees[k] = []NodeAllInfo{}
	}
	for _, n := range allInfo {
		for _, k := range kademliaCommittees {
			if k == n.CommitteeID {
				nodesInKadamliaCommittees[n.CommitteeID] = append(nodesInKadamliaCommittees[n.CommitteeID], n)
			}
		}
	}

	// fmt.Println(nodesInKadamliaCommittees)

	// fmt.Printf("Had %d committes and turned it into %d neighbors\n", len(xored), len(kademliaCommittees))

	// fmt.Printf("Nodes per committee %d, and inted %d\n", math.Log(math.Log(float64(flagArgs.n))), int(math.Log(math.Log(float64(flagArgs.n)))))
	// get i random index arrays, and use them to get loglogn nodes from each of those committees
	for i, c := range kademliaCommittees {
		// pick loglogn
		l := len(nodesInKadamliaCommittees[c])
		per := int(math.Ceil(math.Log(float64(l))))
		indexes := randIndexesWithoutReplacement(l, per)
		// fmt.Println("Per committee: ", per)
		if per == 0 {
			errFatal(nil, "routingtable per was 0")
		}
		for _, j := range indexes {
			node := nodesInKadamliaCommittees[c][j]
			tmp := new(CommitteeMember)
			tmp.Pub = node.Pub
			tmp.IP = node.IP
			routingTable.addMember(uint(i), tmp)
		}
	}

	// and success!

	//log.Printf("Coordinaton setup finished \n")

	nodeCtx.committee = currentCommittee
	nodeCtx.self = selfInfo
	nodeCtx.allInfo = allInfo
	nodeCtx.idaMsgs = IdaMsgs{}
	nodeCtx.idaMsgs.init()
	nodeCtx.consensusMsgs = ConsensusMsgs{}
	nodeCtx.consensusMsgs.init()

	nodeCtx.channels = Channels{}
	nodeCtx.channels.init(len(currentCommittee.Members))

	nodeCtx.reconstructedIdaMsgs = ReconstructedIdaMsgs{}
	nodeCtx.reconstructedIdaMsgs.init()
	// add genesis block here

	nodeCtx.i = CurrentIteration{}

	nodeCtx.routingTable = routingTable

	nodeCtx.txPool = TxPool{}
	nodeCtx.txPool.init()

	nodeCtx.crossTxPool = CrossTxPool{}
	nodeCtx.crossTxPool.init()

	nodeCtx.utxoSet = new(UTXOSet)
	nodeCtx.utxoSet.init()

	nodeCtx.blockchain = Blockchain{}
	nodeCtx.blockchain.init(selfInfo.CommitteeID)

	nodeCtx.blockchain.addRecBlock(response.ReconfigurationBlock)

	gb := response.GensisisBlocks
	// fmt.Println(gb)
	// fmt.Println("Len genesis blocks", len(gb))
	for _, b := range gb {
		// fmt.Print("this committee ", b.ProposedBlock.CommitteeID == nodeCtx.self.CommitteeID, "\n")
		if b.ProposedBlock.CommitteeID == nodeCtx.self.CommitteeID {
			b.processBlock(nodeCtx)
			nodeCtx.blockchain._add(b)
			break
		}
	}

	nodeCtx.utxoSet.verifyNonces()

	buildCurrentNeighbours(nodeCtx)
}

// builds a list of neighbours of length flagArgs.d where all nodes in a committee is sorted on id and a ring is formed.
// Each neighbour is of distance 2^i from your id where distance is just index in the sorted members array.
// this guarantess connectivity, whereas the original random graph where probabilistic (and failed on small sizes)
func buildCurrentNeighbours(nodeCtx *NodeCtx) {
	currentNeighbours := make([][32]byte, nodeCtx.flagArgs.d)

	// selfID := toBigInt(nodeCtx.self.Priv.Pub.Bytes)
	members := nodeCtx.committee.getMemberIDsAsSortedList()

	// copy array and append self
	membersCopy := make([][32]byte, len(members)+1)
	copy(membersCopy, members)
	membersCopy[len(members)] = nodeCtx.self.Priv.Pub.Bytes

	// sort new array
	sort.Slice(membersCopy, func(i, j int) bool {
		return toBigInt(membersCopy[i]).Cmp(toBigInt(membersCopy[j])) < 0
	})

	// find you index
	var selfIndex int
	for i, m := range membersCopy {
		if m == nodeCtx.self.Priv.Pub.Bytes {
			selfIndex = i
		}
	}

	// since self have inserted himself then the next neighbor to self is the index, but in the original members array
	// therefor begin constructing at that index
	taken := make(map[int]bool)
	for i := uint(0); i < nodeCtx.flagArgs.d; i++ {
		dist := (int(math.Pow(2, float64(i))) - 1 + selfIndex) % len(members)
		if !taken[dist] {
			currentNeighbours[i] = members[dist]
			taken[dist] = true
		}
	}

	fmt.Println("\n\nCommittee", bytes32ToString(nodeCtx.self.CommitteeID))
	fmt.Println("Self id ", bytes32ToString(nodeCtx.self.Priv.Pub.Bytes))
	for i, m := range currentNeighbours {
		fmt.Printf("Neigh %d id %s\n", i, bytes32ToString(m))
	}

	// if leaderElection(nodeCtx).Bytes == nodeCtx.self.Priv.Pub.Bytes {
	// 	// members
	// 	for i, m := range members {
	// 		fmt.Printf("m %d %s \n", i, bytes32ToString(m))
	// 	}

	// 	for i, m := range membersCopy {
	// 		fmt.Printf("mc %d %s\n", i, bytes32ToString(m))
	// 	}

	// }

	nodeCtx.neighbors = currentNeighbours
}
