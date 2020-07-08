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
	coordinatorSetup(conn, portNumber, nodeCtx)

	// launch listener
	go listen(listener, nodeCtx)
	// if nodeCtx.self.Debug {
	// 	go debug(nodeCtx)
	// }
	rand.Seed(69)
	startNewIteration(nodeCtx)

	// blocker
	for {
	}
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

			switch idaMsg.Typ {
			case "tx":
				// reconstruct tx
				txData := nodeCtx.reconstructedIdaMsgs.getData(idaMsg.MerkleRoot)
				tx := new(Transaction)
				tx.decode(txData)

				// add tx to pool
				nodeCtx.txPool.add(tx)
				//fmt.Println("Added to txpool")
			case "block":
				// ProposedBlock
				nodeCtx.blockchain.mux.Lock()
				blockByte := nodeCtx.reconstructedIdaMsgs.getData(idaMsg.MerkleRoot)
				block := new(ProposedBlock)
				block.decode(blockByte)

				// TODO verify block
				nodeCtx.blockchain._addProposedBlock(block)
				nodeCtx.blockchain.mux.Unlock()
				fmt.Printf("Block with gh %s added\n", bytes32ToString(block.GossipHash))

				// gossip crossTxes
				// don't need to wait untill block is finished because we have verified the block
				// and since we are honest we expect the block to pass
				go gossipCrossTxes(nodeCtx, block.Transactions)

			default:
				errFatal(nil, "Unknown IDAGossipMsg type")
			}
		}

	case "consensus":
		cMsg, ok := msg.Msg.(ConsensusMsg)
		notOkErr(ok, "ConsensusSignature decoding")

		// check if we allready have accepted the block
		if nodeCtx.blockchain.isBlock(cMsg.GossipHash) {
			log.Println("Block allready accepted")
			// TODO add signature maybe?
			return
		}

		// check if the underlying block has been recived
		if !nodeCtx.blockchain.isProposedBlock(cMsg.GossipHash) {
			timeout := 0
			var found bool = false
			for {
				time.Sleep(default_delta * time.Millisecond)
				if nodeCtx.blockchain.isProposedBlock(cMsg.GossipHash) {
					found = true
					break
				}
				timeout++
				fmt.Printf("waiting for proposed block %d\n", timeout)
				if timeout > 10 {
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
			go handleConsensusEcho(&cMsg, nodeCtx)
			go handleConsensusAccept(&cMsg, nodeCtx)
		}
		handleConsensus(nodeCtx, &cMsg, msg.FromPub)

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

	default:
		log.Fatal("[Error] no known message type")
	}

}
