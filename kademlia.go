package main

import (
	"math/big"
	"net"
	"sync"
)

// todo replace xor operations with these functions
// todo check that the inverse property holds
func xor(b1, b2 [32]byte) [32]byte {
	// returns the xor distance between b1 and b2
	dist := xorBigInt(toBigInt(b1), toBigInt(b2))
	return toByte32(dist.Bytes())
}

func xorBigInt(b1, b2 *big.Int) *big.Int {
	// return the xor distnace between b1 and b2 as bigint
	return new(big.Int).Xor(b1, b2)
}

func txFindClosestCommittee(nodeCtx *NodeCtx, txHash [32]byte) [32]byte {
	// returns the closest committee

	txInt := toBigInt(txHash)

	// xor all committeList with tx
	xored := make([]*big.Int, len(nodeCtx.committeeList))
	for i := range xored {
		xored[i] = xorBigInt(txInt, toBigInt(nodeCtx.committeeList[i]))
	}

	//sort xored
	sortBigIntArr(&xored)

	// lowest distance is now the first elem of the sorted array
	// xor it back with txInt to get original committee ID
	lowest := xorBigInt(txInt, xored[0])

	return toByte32(lowest.Bytes())
}

func routeTx(nodeCtx *NodeCtx, msg Msg, closestCommitteeID [32]byte) {
	// routes tx
	// closesCommitteID may or not be in routing table. But it is definitly not ownCommittteeID

	// check if closesCommitteeID is in routing table
	r := nodeCtx.routingTable.get()
	for _, c := range r {
		if c.ID == closestCommitteeID {
			// we have it! c
			sendMsgToCommittee(msg, &c)
			return
		}
	}

	// closestCommitteeID is not in routing table. Therefor iniate routing
	findNodeAndSend(nodeCtx, closestCommitteeID, msg)
}

func findClosestsCommittee(nodeCtx *NodeCtx, committeeIDbytes [32]byte) Committee {
	// convert to big ints to be able to do bitwise xor operations
	selfCommitteeID := new(big.Int)
	selfCommitteeID.SetBytes(nodeCtx.self.CommitteeID[:])
	committeeID := new(big.Int)
	committeeID.SetBytes(committeeIDbytes[:])

	xored := new(big.Int).Xor(selfCommitteeID, committeeID)
	r := nodeCtx.routingTable.get()
	var closest int = 0
	// it can't be less the first entry in our routing table since that is our closest neighbor
	for i := range r {
		// fmt.Println(nodeCtx.self.CommitteeID, committeeID, r)
		curr := new(big.Int).Xor(selfCommitteeID, r[i].BigIntID)

		if i == len(r)-1 {
			// there is no i+1 so break
			// curr > xored
			if curr.Cmp(xored) > 0 {
				//fmt.Println(i, len(r), curr, xored)
				errFatal(nil, "what2 findNode")
			} else if curr.Cmp(xored) == 0 {
				errFatal(nil, "2: got committteID that was in routing table")
			}
			// the closest node is the one furthes away in our routing table
			closest = i
			break
		}
		next := new(big.Int).Xor(selfCommitteeID, r[i+1].BigIntID)

		if xored.Cmp(curr) == 0 || xored.Cmp(next) == 0 {
			errFatal(nil, "got a committeID that was in routing table")
		}

		// curr < xored
		if curr.Cmp(xored) < 0 {
			if xored.Cmp(next) < 0 { //xored < next
				// find out which of them is closer.
				xMinC := new(big.Int).Sub(xored, curr)
				nMinX := new(big.Int).Sub(next, xored)
				// if xored-curr < next-xored {
				if xMinC.Cmp(nMinX) < 0 {
					closest = i
				} else {
					closest = i + 1
				}
				break
			} else {
				continue
			}
		} else {
			errFatal(nil, "what findNode")
		}
	}
	return r[closest]
}

func findNodeAndSend(nodeCtx *NodeCtx, commiteeID [32]byte, msg interface{}) {
	c := findNode(nodeCtx, commiteeID)

	for _, v := range c.Members {
		go dialAndSend(v.IP, msg)
	}
}

func findNode(nodeCtx *NodeCtx, committeeID [32]byte) Committee {
	// given that committeeID is not in our routing table, then send findNode request to closests committe to committeeID

	c := findClosestsCommittee(nodeCtx, committeeID)

	if c.ID == committeeID {
		errFatal(nil, "what3")
	}

	// find closest committee in our routing table to committeeID
	return recursiveFindNode(nodeCtx, committeeID, c)
}

func recursiveFindNode(nodeCtx *NodeCtx, committeeID [32]byte, nCommittee Committee) Committee {
	// construct findNode message and send it.
	findNodeMsg := KademliaFindNodeMsg{committeeID}
	msg := Msg{"find_node", findNodeMsg, nodeCtx.self.Priv.Pub}
	var wg sync.WaitGroup
	responses := make(chan KademliaFindNodeResponse, len(nCommittee.Members))
	for _, m := range nCommittee.Members {
		wg.Add(1)
		go func() {
			conn := dial(m.IP)
			// TODO not all connections may retrun positive
			sendMsg(conn, msg)
			response := new(KademliaFindNodeResponse)

			reciveMsg(conn, response)
			// fmt.Println(response)
			conn.Close()
			responses <- *response
			wg.Done()
		}()
	}
	wg.Wait()

	l := len(responses)
	resp := make([]KademliaFindNodeResponse, l)
	for i := 0; i < l; i++ {
		resp[i] = <-responses
	}

	// check that all gave the same committee ID
	_id := resp[0].Committee.ID
	for _, r := range resp {

		if r.Committee.ID != _id {
			// fmt.Println(resp, len(resp))
			errFatal(nil, "a response gave a differet committeeID")
		}
	}

	// fmt.Println("  ", _id, committeeID)
	if _id == committeeID {
		// success found the committee ID
		// return all members in that committee
		return aggregateResponses(resp, _id)
	}
	// aggregate all members and pick log(n/m) of them to continue
	c := aggregateResponses(resp, _id)
	// pick log(n/m) of them into new committe

	indexes := randIndexesWithoutReplacement(len(c.Members), len(nCommittee.Members))

	newC := Committee{}
	newC.init(_id)

	i := 0
	for _, v := range c.Members {
		var isIn bool = false
		for _, j := range indexes {
			if i == j {
				isIn = true
				break
			}
		}
		if !isIn {
			i++
			continue
		}
		newC.addMember(v)
		i++
	}
	return recursiveFindNode(nodeCtx, committeeID, newC)
}

func aggregateResponses(resp []KademliaFindNodeResponse, ID [32]byte) Committee {
	c := Committee{}
	c.init(ID)
	for _, r := range resp {
		for _, ms := range r.Committee.Members {
			c.addMember(ms)
		}
	}
	return c
}

func handleFindNode(nodeCtx *NodeCtx, conn net.Conn, msg KademliaFindNodeMsg) {
	c := _handleFindNode(nodeCtx, msg)
	response := KademliaFindNodeResponse{}
	response.Committee = c
	sendMsg(conn, response)
}

func _handleFindNode(nodeCtx *NodeCtx, msg KademliaFindNodeMsg) Committee {
	// check if we have committeeID in our routing table
	r := nodeCtx.routingTable.get()
	for _, c := range r {
		if c.ID == msg.ID {
			// we have it!
			return c
		}
	}

	// if not, then find the closest committee in our routing table to the target committee
	return findClosestsCommittee(nodeCtx, msg.ID)
}
