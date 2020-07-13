package main

import (
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/kimborgen/go-merkletree"
	"github.com/klauspost/reedsolomon"
)

type IDAGossipMsg struct {
	Typ        string
	Chunks     [][]byte
	Proofs     []*merkletree.Proof
	MerkleRoot [32]byte
}

func IDAGossip(nodeCtx *NodeCtx, msg []byte, typ string) [32]byte {
	// Initiates the IDA gossip process

	// Since msg is of unknown size, it cannot always be divided into equal kappa data shards.
	// Therefor we add a last datashard to contain the rest of the msg.
	// if msg can be evenly divded, then the last data shard is simply zero bytes

	// log to coordinator that we are initiating ida gossip process
	bat := new(ByteArrayAndTimestamp)
	battmp := hash(msg)
	bat.B = battmp[:]
	bat.T = time.Now()
	go dialAndSendToCoordinator("start_ida_gossip", bat)

	// initate some static variables
	// TODO: dynamicly create these
	var phi float64 = default_phi
	var kappa int = default_kappa
	var parity int = int(float64(kappa) * phi)
	if parity != default_parity {
		errFatal(nil, "parity not equal to default")
	}
	// log.Println("Paritiy: ", parity)

	// build reed solomon chunks
	enc, err := reedsolomon.New(kappa, parity)
	ifErrFatal(err, "createing reedsolomon")

	data := make([][]byte, kappa+parity)

	if len(msg)%kappa != 0 {
		// msg can not be evenly divided by kappa
		// therefor we must pad message to be divisible

		//fmt.Println(msg)
		originalMsg := make([]byte, len(msg))
		copy(originalMsg, msg)
		//fmt.Printf("Padding msg from len %d", len(msg))
		msg = padByteToBeDivisible(msg, uint(kappa))
		//fmt.Printf(" to %d\n", len(msg))
		//fmt.Println(msg)

		// test unpadding
		okk := isPadded(msg)
		notOkErr(okk, "msg was not padded?")
		unpadded := unPad(msg)
		if !reflect.DeepEqual(unpadded, originalMsg) {
			fmt.Println(unpadded, "\n\n", originalMsg)
			errFatal(nil, "unpadded was not equal to orgiinal msg")
		}

	}

	chunkSize := len(msg) / (kappa)
	if len(msg)%kappa != 0 {
		errFatal(nil, "msg should have been padded")
	}

	// Create all shards
	for i, _ := range data {
		data[i] = make([]byte, chunkSize)
	}

	// populate the first kappa shards
	for i := 0; i < kappa; i++ {
		data[i] = msg[i*chunkSize : (i+1)*chunkSize]
	}

	err = enc.Encode(data)
	ifErrFatal(err, "encoding reedsolomon")

	ok, err := enc.Verify(data)
	ifErrFatal(err, "reedsolomon shard sizes not equal")
	if !ok {
		log.Fatal("[Error] Reed solomon codes not ok: ", ok)
	}

	// create merkle tree over data:
	tree, err := merkletree.New(data)
	ifErrFatal(err, "creating merkle tree")

	root := tree.Root()
	//fmt.Println("Root len: ", len(root))
	root32 := toByte32(root)

	// create proofs for all leafs
	proofs := make([]*merkletree.Proof, len(data))
	for i, _ := range proofs {
		// Problem, if data has two equal arrays then GenerateProof will return
		proofs[i], err = tree.GenerateProofUsingIndex(uint64(i), 0)
		ifErrFatal(err, "generating proof")
		// fmt.Println(i, proofs[i].Index, proofs[i], "\n", data[i], "\n\n")
		if proofs[i].Index != uint64(i) {
			fmt.Println(proofs[i].Index, i)
			errFatal(nil, "Proof index not the same as index")
		}
	}

	// gossip (kappa+parity)/d data chuncks (with proofs) to each neighbour.
	var chunksToEach = (kappa + parity) / len(nodeCtx.neighbors)
	if (kappa+parity)%len(nodeCtx.neighbors) != 0 {
		log.Printf("chunks %d, doesnt divide evenly among %d neighbours", (kappa + parity), len(nodeCtx.neighbors))
	}

	msgs := make([]Msg, len(nodeCtx.neighbors))

	ii := 0
	total_chunks := 0
	for i := 0; i < len(nodeCtx.neighbors)*chunksToEach; i += chunksToEach {
		chunks := data[i : i+chunksToEach]
		//fmt.Println("\n\n", chunks)
		total_chunks += len(chunks)
		proofs := proofs[i : i+chunksToEach]
		msgs[ii] = Msg{"IDAGossipMsg", IDAGossipMsg{typ, chunks, proofs, root32}, nodeCtx.self.Priv.Pub}
		ii += 1
	}
	// log.Println("Creating: Len of chunks ", total_chunks, chunksToEach, len(msgs))

	// send each msg to node
	for i, msgToNode := range msgs {
		// fmt.Println("neig", nodeCtx.committee.Members[nodeCtx.neighbors[i]])
		// fmt.Println("neigg", nodeCtx.neighbors)
		// fmt.Println("neiggg", nodeCtx.neighbors[i])
		var addr string = nodeCtx.committee.Members[nodeCtx.neighbors[i]].IP
		dialAndSend(addr, msgToNode)
	}
	return root32
}

/*
func getLenOfChunks(msgs []IDAGossipMsg) int {
	var totalChunks int
	for _, msg := range msgs {
		totalChunks += len(msg.Chunks)
	}
	return totalChunks
}
handleIDAGossipMsg
*/

func handleIDAGossipMsg(
	idaMsg IDAGossipMsg,
	nodeCtx *NodeCtx) bool {
	// handles IDAGossipMsg returns true if msg is reconstructed into reconstructed messages, false otherwise

	// check if we allready have enough chunks to recreate
	if l := nodeCtx.idaMsgs.getLenOfChunks(idaMsg.MerkleRoot); l >= default_kappa {
		return false
	}

	// check that IDA messages was correct
	if len(idaMsg.Chunks) != len(idaMsg.Proofs) {
		errr(nil, "number of proofs not matching amount of chunks")
		return false
	}

	for i, _ := range idaMsg.Chunks {
		root := make([]byte, 32)
		for i, elem := range idaMsg.MerkleRoot {
			root[i] = elem
		}
		verified, err := merkletree.VerifyProof(idaMsg.Chunks[i], false, idaMsg.Proofs[i], [][]byte{root})
		fail := ifErr(err, "merkletree.Verifyproof")
		if fail || !verified {
			errr(nil, "chunk could not be verified")
			return false
		}
	}

	// check if merkleRoot is new
	nodeCtx.idaMsgs.mux.Lock()
	if ok := nodeCtx.idaMsgs._isArr(idaMsg.MerkleRoot); !ok {
		nodeCtx.idaMsgs._add(idaMsg.MerkleRoot, idaMsg)
		nodeCtx.idaMsgs.mux.Unlock()
		gossipSend(idaMsg, nodeCtx)
	} else {
		nodeCtx.idaMsgs.mux.Unlock()
		// check if the message is unique
		for _, newProof := range idaMsg.Proofs {
			for _, existingMsg := range nodeCtx.idaMsgs.getMsgs(idaMsg.MerkleRoot) {
				for _, existingProof := range existingMsg.Proofs {
					// check if index is equal
					if newProof.Index == existingProof.Index {
						//log.Printf("Found existing proof with same index\n")
						return false
					}
				}
			}
		}

		// add to list
		nodeCtx.idaMsgs.mux.Lock()
		nodeCtx.idaMsgs._add(idaMsg.MerkleRoot, idaMsg)
		// check if we have enough chunks to recreate
		if nodeCtx.idaMsgs._getLenOfChunks(idaMsg.MerkleRoot) >= default_kappa {
			// recreate data array and fill it with known chunks
			data := make([][]byte, default_kappa+default_parity)
			for _, elem := range nodeCtx.idaMsgs._getMsgs(idaMsg.MerkleRoot) {
				for i, chunk := range elem.Chunks {
					// gather leaf node position from proof
					index := elem.Proofs[i].Index

					// check if that location in data is not allready filled
					// doesnt really matter tho
					// if data[index] != nil {
					// 	errFatal(nil, "there was two equal chunks")
					// }

					data[index] = chunk
				}
			}

			enc, err := reedsolomon.New(default_kappa, default_parity)
			ifErrFatal(err, "reedsolomon encoder creation")

			// now we can recreate the message
			err = enc.Reconstruct(data)
			if err != nil {
				fmt.Println(data)
				fmt.Println(len(data))
				for i, d := range data {
					fmt.Println(i, bytesToString(d))
				}
				fmt.Println("Len of chunks: ", nodeCtx.idaMsgs._getLenOfChunks(idaMsg.MerkleRoot))
				ifErrFatal(err, "Could not reconstruct data")
			}

			// add IDAMsg to check that we don't allready have a msg for this root
			ok := nodeCtx.reconstructedIdaMsgs.safeAdd(idaMsg.MerkleRoot, data)
			nodeCtx.idaMsgs.mux.Unlock()

			if ok {
				// now the first default_kappa elements of data is the message! :)
				// log.Println("Message succesfully recreated and added")

				// send success message to coordinator
				msg := Msg{"IDASuccess", idaMsg.MerkleRoot, nodeCtx.self.Priv.Pub}
				go dialAndSend(coord+":8080", msg)
				go gossipSend(idaMsg, nodeCtx)

				return true
			}
		} else {
			nodeCtx.idaMsgs.mux.Unlock()
		}

		go gossipSend(idaMsg, nodeCtx)

	}
	return false
}

func gossipSend(msg IDAGossipMsg, nodeCtx *NodeCtx) {
	// If we do not have enough chunks then gossip the message to all neighbours
	msgs := make([]Msg, len(nodeCtx.neighbors))

	for i := range msgs {
		msgs[i] = Msg{"IDAGossipMsg", msg, nodeCtx.self.Priv.Pub}
	}

	// send each msg to node
	for i, msg := range msgs {
		// fmt.Println("neig", nodeCtx.committee.Members[nodeCtx.neighbors[i]])
		// fmt.Println("neigg", nodeCtx.neighbors)
		// fmt.Println("neiggg", nodeCtx.neighbors[i])
		var addr string = nodeCtx.committee.Members[nodeCtx.neighbors[i]].IP
		//log.Printf("addr: %s\n", addr)
		go dialAndSend(addr, msg)
	}
}
