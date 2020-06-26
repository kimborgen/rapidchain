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
	initialRandomness := coordinatorSetup(conn, portNumber, nodeCtx)
	initialRandomness = initialRandomness
	// launch listener
	go listen(listener, nodeCtx)
	if nodeCtx.self.Debug {
		go debug(nodeCtx)
	}
	rand.Seed(69)

	// launch leader election protocol
	var leaderPub *PubKey = leaderElection(nodeCtx)
	// fmt.Printf("Own: %x, leader: %x\n", nodeCtx.self.Priv.Pub.Bytes, leaderPub.Bytes)

	// If this node is leader then initate ida-gossip
	if leaderPub.Bytes == nodeCtx.self.Priv.Pub.Bytes {
		//fmt.Println("\n\n\n", nodeCtx.routingTable, "\n\n\n")

		// test bytes/big.Int xoring

		// wait untill tx pool is ok large
		for len(nodeCtx.txPool.pool) < 10 {

		}

		leader(nodeCtx)
	}

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
	fmt.Println(msg.Typ)
	switch msg.Typ {
	case "IDAGossipMsg":
		idaMsg, ok := msg.Msg.(IDAGossipMsg)
		fmt.Println("  ", idaMsg.Typ)
		notOkErr(ok, "IDAGossipMsg decoding")
		reconstructed := handleIDAGossipMsg(idaMsg, nodeCtx)
		if reconstructed {

			switch idaMsg.Typ {
			case "tx":
				// reconstruct tx
				txData := nodeCtx.reconstructedIdaMsgs.getData(idaMsg.MerkleRoot)
				tx := Transaction{}
				tx.decode(txData)

				// add tx to pool
				nodeCtx.txPool.safeAdd(&tx)
			case "block":
				// ProposedBlock
				blockByte := nodeCtx.reconstructedIdaMsgs.getData(idaMsg.MerkleRoot)
				block := new(ProposedBlock)
				block.decode(blockByte)

				// TODO verify block
				nodeCtx.blockchain.addProposedBlock(block)

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

		// check if the underlying block has been recived
		if !nodeCtx.blockchain.isProposedBlock(cMsg.GossipHash) {
			time.Sleep(default_delta * 2 * time.Millisecond)
			if !nodeCtx.blockchain.isProposedBlock(cMsg.GossipHash) {
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

		handleFindNode(nodeCtx, conn, kMsg)
	case "transaction":
		// recived a transaction
		tMsg, ok := msg.Msg.(Transaction)
		notOkErr(ok, "transaction decoding") //todo dont need such strict err
		// figure out which committee the transaction belongs to
		cID := txFindClosestCommittee(nodeCtx, tMsg.Hash)

		// if current committe then initiate IDA-Gossip
		if cID == nodeCtx.self.CommitteeID {
			// ida gossip the tx so the rest of the committee gets the tx
			IDAGossip(nodeCtx, tMsg.encode(), "tx")

			// add to tx pool
			ok := nodeCtx.txPool.safeAdd(&tMsg)
			if ok {
				// log.Println("Tx recived to target committee and added to txpool", tMsg.Hash)
			}
		} else {
			// log.Println("Tx not target committe, routing", tMsg.Hash)
			go routeTx(nodeCtx, msg, cID)
		}

	default:
		log.Fatal("[Error] no known message type")
	}

}
