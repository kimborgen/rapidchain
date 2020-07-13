package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"
)

func launchNode(flagArgs *FlagArgs) {

	conn := dial(coord + ":8080")
	// start listening. We do this here becuase we need to choose a unique port
	// number, and send that port number to coordinator so every node has correct port and ip
	listener, err := net.Listen("tcp", ":0")
	ifErrFatal(err, "listener node")
	portNumber := listener.Addr().(*net.TCPAddr).Port

	//fmt.Print(allInfo, initialRandomness, currentCommittee, "\n\n")

	nodeCtx := new(NodeCtx)
	nodeCtx.flagArgs = *flagArgs
	// fmt.Println("Before coord")
	coordinatorSetup(conn, portNumber, nodeCtx)
	// fmt.Println("After coord")
	// launch listener
	go listen(listener, nodeCtx)
	// if nodeCtx.self.Debug {
	// 	go debug(nodeCtx)
	// }
	rand.Seed(69)
	startNewIteration(nodeCtx)

	// blocker
	// for {
	// }
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
	// fmt.Println(msg.Typ)
	switch msg.Typ {
	case "IDAGossipMsg":
		idaMsg, ok := msg.Msg.(IDAGossipMsg)
		// fmt.Println("  ", idaMsg.Typ)
		notOkErr(ok, "IDAGossipMsg decoding")
		reconstructed := handleIDAGossipMsg(idaMsg, nodeCtx)
		if reconstructed {

			// log to coordinator
			bat := new(ByteArrayAndTimestamp)
			data := nodeCtx.reconstructedIdaMsgs.getData(idaMsg.MerkleRoot)
			id := hash(data)
			bat.B = id[:]
			bat.T = time.Now()
			go dialAndSendToCoordinator("reconstructed_ida_gossip", bat)

			switch idaMsg.Typ {
			case "tx":
				// reconstruct tx
				tx := new(Transaction)
				tx.decode(data)

				// add tx to pool
				nodeCtx.txPool.add(tx)
				//fmt.Println("Added to txpool")
			case "block":
				// ProposedBlock
				nodeCtx.blockchain.mux.Lock()
				block := new(ProposedBlock)
				block.decode(data)

				// TODO verify block
				nodeCtx.blockchain._addProposedBlock(block)
				nodeCtx.blockchain.mux.Unlock()
				fmt.Printf("Block with gh %s added\n", bytes32ToString(block.GossipHash))
			default:
				errFatal(nil, "Unknown IDAGossipMsg type")
			}
		}

	case "consensus":
		cMsg, ok := msg.Msg.(ConsensusMsg)
		notOkErr(ok, "ConsensusSignature decoding")

		// check if we allready have accepted the block
		if nodeCtx.blockchain.isBlock(cMsg.GossipHash) {
			log.Println("Block allready accepted", bytes32ToString(cMsg.GossipHash), cMsg.Tag)
			// TODO add signature maybe?
			return
		}

		// check if the underlying block has been recived
		if !nodeCtx.blockchain.isProposedBlock(cMsg.GossipHash) {
			timeout := 0
			var found bool = false
			for {
				time.Sleep(time.Duration(nodeCtx.flagArgs.delta) * time.Millisecond)
				if nodeCtx.blockchain.isProposedBlock(cMsg.GossipHash) {
					found = true
					break
				}
				timeout++
				// fmt.Printf("waiting for proposed block %d\n", timeout)
				if timeout > 5 {
					break
				}
			}
			if !found {
				fmt.Println("Comittee ", bytes32ToString(nodeCtx.committee.ID))
				fmt.Println("Selfid ", bytes32ToString(nodeCtx.self.Priv.Pub.Bytes))
				fmt.Println("isLeader?", nodeCtx.committee.CurrentLeader.Bytes == nodeCtx.self.Priv.Pub.Bytes)
				fmt.Println("len of proposed blocks: ", len(nodeCtx.blockchain.ProposedBlocks))
				fmt.Println("Gossiphash: ", bytes32ToString(cMsg.GossipHash))
				fmt.Println("Tag ", cMsg.Tag)
				fmt.Println("len Idamsgs ", len(nodeCtx.idaMsgs.m))
				fmt.Println("len r ida ", len(nodeCtx.reconstructedIdaMsgs.m))
				errFatal(nil, "ProposedBlock not recivied")
			}
		}

		if cMsg.Tag == "propose" {
			// empty channel
			for len(nodeCtx.channels.echoChan) > 0 {
				<-nodeCtx.channels.echoChan
			}
			go handleConsensusEcho(cMsg, nodeCtx, 0)
			go handleConsensusAccept(cMsg, nodeCtx, 0)
		}

		handleConsensus(nodeCtx, cMsg, msg.FromPub)

	case "find_node":
		kMsg, ok := msg.Msg.(KademliaFindNodeMsg)
		notOkErr(ok, "findNode decoding")

		if kMsg.ID == nodeCtx.self.CommitteeID {
			errFatal(nil, "Got a find_node msg but I am the target committee")
		}

		b := new(ByteArrayAndTimestamp)
		b.B = byteSliceAppend(kMsg.ID[:], nodeCtx.self.CommitteeID[:])
		b.T = time.Now()

		go dialAndSendToCoordinator("find_node", b)

		handleFindNode(nodeCtx, conn, kMsg)
	case "transaction":
		// recived a transaction
		tMsg, ok := msg.Msg.(Transaction)
		notOkErr(ok, "transaction decoding") //todo dont need such strict err
		// figure out which committee the transaction belongs to
		if tMsg.Hash == [32]byte{} {
			errFatal(nil, fmt.Sprintf("tMsg.Hash was empty with t:", tMsg))
		}
		cID := txFindClosestCommittee(nodeCtx, tMsg.Hash)

		// if current committe then initiate IDA-Gossip
		if cID == nodeCtx.self.CommitteeID {
			// send log to coordinator that tx has been recived at target destination
			b := new(ByteArrayAndTimestamp)
			btmp := tMsg.id()
			b.B = btmp[:]
			b.T = time.Now()
			go dialAndSendToCoordinator("transaction_recieved", b)

			// ida gossip the tx so the rest of the committee gets the tx
			IDAGossip(nodeCtx, tMsg.encode(), "tx")

			// add to tx pool
			//ok := nodeCtx.txPool.safeAdd(&tMsg)
			//if ok {
			// log.Println("Tx recived to target committee and added to txpool", tMsg.Hash)
			//}
		} else {
			// log.Println("Tx not target committe, routing", tMsg.Hash)

			// indicate to coordinator that we are starting a routing

			b := new(ByteArrayAndTimestamp)
			btmp := tMsg.id()
			b.B = btmp[:]
			b.T = time.Now()

			go dialAndSendToCoordinator("routetx", b)

			go routeTx(nodeCtx, msg, cID)

		}
	case "crosstransaction":
		// recived a transaction
		tMsg, ok := msg.Msg.(Transaction)
		notOkErr(ok, "crosstransaction decoding") //todo dont need such strict err

		// ida gossip the tx so the rest of the committee gets the tx
		IDAGossip(nodeCtx, tMsg.encode(), "tx")

		// add to tx pool
		// nodeCtx.txPool.safeAdd(&tMsg)
	case "crosstransactionresponse":
		// recived a transaction
		tMsg, ok := msg.Msg.(Transaction)
		notOkErr(ok, "crosstransactionresponse decoding") //todo dont need such strict err

		// ida gossip the tx so the rest of the committee gets the tx
		IDAGossip(nodeCtx, tMsg.encode(), "tx")

		// add to tx pool
		// nodeCtx.txPool.safeAdd(&tMsg)
	case "request_block":
		iter, ok := msg.Msg.(uint64)

		notOkErr(ok, "request_block decoding")

		lastBlock := nodeCtx.blockchain.getLatest()
		block := nodeCtx.blockchain.getByIteration(iter)
		tmp := new(RequestBlockAnswer)
		// {block, uint64(nodeCtx.i.getI())}
		tmp.Block = block
		tmp.LastIteration = uint64(lastBlock.ProposedBlock.Iteration)
		sendMsg(conn, tmp)

	default:
		log.Fatal("[Error] no known message type")
	}

}

type RequestBlockAnswer struct {
	Block         *FinalBlock
	LastIteration uint64
}

// Requests and adds all missing blocks
func requestAndAddMissingBlocks(nodeCtx *NodeCtx) {
	lastBlock := nodeCtx.blockchain.getLatest()

	if lastBlock.ProposedBlock.Iteration+1 != nodeCtx.i.getI() {
		log.Println(lastBlock.ProposedBlock.Iteration+1, nodeCtx.i.getI())
		errFatal(nil, "blockchain and iteration not in sync not equal")
	}

	request := new(Msg)
	request.Typ = "request_block"
	request.Msg = uint64(lastBlock.ProposedBlock.Iteration + 1)
	request.FromPub = nodeCtx.self.Priv.Pub
	response := new(RequestBlockAnswer)

	// choose a random node from the committee
	for {
		rnd := rand.Intn(len(nodeCtx.committee.Members))
		var node *CommitteeMember
		i := 0
		for _, cm := range nodeCtx.committee.Members {
			if i == rnd {
				node = cm
				break
			}
			i++
		}
		if node == nil {
			errFatal(nil, "could not pick neighbour")
		}

		conn := dial(node.IP)

		sendMsg(conn, request)
		reciveMsg(conn, response)
		conn.Close()

		if response == nil {
			errFatal(nil, "response was nil")
		}

		if lastBlock.ProposedBlock.Iteration >= response.Block.ProposedBlock.Iteration {
			log.Println("No new block, try again with new node", lastBlock.ProposedBlock.Iteration, response.Block.ProposedBlock.Iteration)
			continue
		}
		break
	}

	// // TODO verify block and signatures
	// if len(response.Block.Signatures) < len(nodeCtx.committee.Members)/2 {
	// 	log.Println("not enough signatures")
	// 	return
	// }

	// if response.Block.ProposedBlock.PreviousGossipHash != lastBlock.ProposedBlock.GossipHash {
	// 	log.Println("PreviousGossipHash was not lastBlock")
	// 	return
	// }

	nodeCtx.blockchain.add(response.Block)
	response.Block.forceProcessBlock(nodeCtx)
	nodeCtx.i.add()

	if response.LastIteration != uint64(response.Block.ProposedBlock.Iteration) {
		log.Println(response.LastIteration, response.Block.ProposedBlock.Iteration, nodeCtx.i.getI())
		log.Println("Last iteration in response was not the returend block, therefor request next missing block")
		requestAndAddMissingBlocks(nodeCtx)
	}
}
