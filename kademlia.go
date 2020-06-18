package main

import (
	"fmt"
	"net"
	"sync"
)

func findClosestsCommittee(nodeCtx *NodeCtx, committeeID uint) Committee {
	xored := nodeCtx.self.CommitteeID ^ committeeID
	r := nodeCtx.routingTable.get()
	var closest int = 0
	// it can't be less the first entry in our routing table since that is our closest neighbor
	for i := range r {
		// fmt.Println(nodeCtx.self.CommitteeID, committeeID, r)
		curr := nodeCtx.self.CommitteeID ^ r[i].ID
		if i == len(r)-1 {
			// there is no i+1 so break
			if curr > xored {
				//fmt.Println(i, len(r), curr, xored)
				errFatal(nil, "what2 findNode")
			} else if curr == xored {
				errFatal(nil, "2: got committteID that was in routing table")
			}
			// the closest node is the one furthes away in our routing table
			closest = i
			break
		}
		next := nodeCtx.self.CommitteeID ^ r[i+1].ID

		if xored == curr || xored == next {
			errFatal(nil, "got a committeID that was in routing table")
		}
		if curr < xored {
			if xored < next {
				// find out which of them is closer.
				if xored-curr < next-xored {
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

func findNodeAndSend(nodeCtx *NodeCtx, commiteeID uint, msg interface{}) {
	c := findNode(nodeCtx, commiteeID)

	for _, v := range c.Members {
		go dialAndSend(v.IP, msg)
	}
}

func findNode(nodeCtx *NodeCtx, committeeID uint) Committee {
	// given that committeeID is not in our routing table, then send findNode request to closests committe to committeeID

	c := findClosestsCommittee(nodeCtx, committeeID)

	if c.ID == committeeID {
		errFatal(nil, "what3")
	}

	// find closest committee in our routing table to committeeID
	return recursiveFindNode(nodeCtx, committeeID, c)
}

func recursiveFindNode(nodeCtx *NodeCtx, committeeID uint, nCommittee Committee) Committee {
	// construct findNode message and send it.
	findNodeMsg := KademliaFindNodeMsg{committeeID}
	msg := Msg{"find_node", findNodeMsg, nodeCtx.self.ID}
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

func aggregateResponses(resp []KademliaFindNodeResponse, ID uint) Committee {
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
	if c.ID == 0 {
		fmt.Println("HMMMMM", c, msg)
	}
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
