package main

import (
	"fmt"
	"log"

	"github.com/klauspost/reedsolomon"
	"github.com/wealdtech/go-merkletree"
)

type IDAGossipMsg struct {
	Chunks     [][]byte
	Proofs     []*merkletree.Proof
	MerkleRoot [32]byte
}

func IDAGossip(
	flagArgs *FlagArgs,
	currentCommittee *Committee,
	currentNeighbours *[]uint,
	selfID uint,
	B *[]byte) [32]byte {
	// Initiates the IDA gossip process
	block := *B
	// initate some static variables
	// TODO: dynamicly create these
	var phi float64 = default_phi
	var kappa int = default_kappa
	var parity int = int(float64(kappa) * phi)
	if parity != default_parity {
		errFatal(nil, "parity not equal to default")
	}
	log.Println("Paritiy: ", parity)

	// build reed solomon chunks
	enc, err := reedsolomon.New(kappa, parity)
	ifErrFatal(err, "createing reedsolomon")

	data := make([][]byte, kappa+parity)

	// make sure that block can be divided evenly among the kappa data shards.
	if len(block)%kappa != 0 {
		errFatal(nil, fmt.Sprintf("block size %d could not be divided over %d kappa chunks", len(block), kappa))
	}

	// Create all shards
	chunkSize := len(block) / kappa
	for i, _ := range data {
		data[i] = make([]byte, chunkSize)
	}

	// populate the first kappa shards
	for i, _ := range data[:kappa] {
		data[i] = block[i*chunkSize : (i+1)*chunkSize]
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
	fmt.Println("Root len: ", len(root))
	var root32 [32]byte
	for i, elem := range root {
		root32[i] = elem
	}

	// create proofs for all leafs
	proofs := make([]*merkletree.Proof, len(data))
	for i, _ := range proofs {
		proofs[i], err = tree.GenerateProof(data[i], 0)
		if proofs[i].Index != uint64(i) {
			fmt.Println(proofs[i].Index, i)
			errFatal(nil, "Proof index not the same as index")
		}
	}

	// gossip (kappa+parity)/d data chuncks (with proofs) to each neighbour.
	var chunksToEach = (kappa + parity) / len(*currentNeighbours)
	if (kappa+parity)%len(*currentNeighbours) != 0 {
		log.Printf("chunks %d, doesnt divide evenly among %d neighbours", (kappa + parity), len(*currentNeighbours))
	}

	msgs := make([]Msg, len(*currentNeighbours))

	ii := 0
	total_chunks := 0
	for i := 0; i < len(*currentNeighbours)*chunksToEach; i += chunksToEach {
		chunks := data[i : i+chunksToEach]
		//fmt.Println("\n\n", chunks)
		total_chunks += len(chunks)
		proofs := proofs[i : i+chunksToEach]
		msgs[ii] = Msg{"IDAGossipMsg", IDAGossipMsg{chunks, proofs, root32}, selfID}
		ii += 1
	}
	log.Println("Creating: Len of chunks ", total_chunks, chunksToEach, len(msgs))

	// send each msg to node
	for i, msg := range msgs {
		var addr string = currentCommittee.Members[(*currentNeighbours)[i]].IP
		dialAndSend(addr, msg)
	}
	return root32
}

func getLenOfChunks(msgs []IDAGossipMsg) int {
	var totalChunks int
	for _, msg := range msgs {
		totalChunks += len(msg.Chunks)
	}
	return totalChunks
}

func handleIDAGossipMsg(
	idaMsg IDAGossipMsg,
	idaMsgs *map[[32]byte][]IDAGossipMsg,
	currentCommittee *Committee,
	currentNeighbours *[]uint,
	self *NodeAllInfo) ([32]byte, [][]byte) {
	// check if we allready have enough chunks to recreate
	//fmt.Println("Len of IDAMessages: ", len((*idaMsgs)[idaMsg.MerkleRoot]))
	//log.Println("Len of chunks: ", getLenOfChunks((*idaMsgs)[idaMsg.MerkleRoot]))
	if l := getLenOfChunks((*idaMsgs)[idaMsg.MerkleRoot]); l >= default_kappa {
		// log.Printf("#IDAchunks allready enough %d of required %d\n", l, default_kappa)
		return [32]byte{}, nil
	}

	// check that IDA messages was correct
	if len(idaMsg.Chunks) != len(idaMsg.Proofs) {
		errr(nil, "number of proofs not matching amount of chunks")
		return [32]byte{}, nil
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
			return [32]byte{}, nil
		}
	}

	// check if merkleRoot is new
	if _, ok := (*idaMsgs)[idaMsg.MerkleRoot]; !ok {
		(*idaMsgs)[idaMsg.MerkleRoot] = []IDAGossipMsg{idaMsg}
		gossipSend(idaMsg, currentCommittee, currentNeighbours, self.ID)
	} else {
		arr := (*idaMsgs)[idaMsg.MerkleRoot]
		// check if the message is unique
		for _, newProof := range idaMsg.Proofs {
			for _, existingMsg := range arr {
				for _, existingProof := range existingMsg.Proofs {
					// check if index is equal
					if newProof.Index == existingProof.Index {
						//log.Printf("Found existing proof with same index\n")
						return [32]byte{}, nil
					}
				}
			}
		}

		// add to list
		(*idaMsgs)[idaMsg.MerkleRoot] = append((*idaMsgs)[idaMsg.MerkleRoot], idaMsg)

		// check if we have enough chunks to recreate
		if getLenOfChunks((*idaMsgs)[idaMsg.MerkleRoot]) >= default_kappa {
			// recreate data array and fill it with known chunks
			data := make([][]byte, default_kappa+default_parity)
			for _, elem := range (*idaMsgs)[idaMsg.MerkleRoot] {
				for i, chunk := range elem.Chunks {
					// gather leaf node position from proof
					index := elem.Proofs[i].Index

					// check if that location in data is not allready filled
					if data[index] != nil {
						errFatal(nil, "there was two equal chunks")
					}

					data[index] = chunk
				}
			}

			enc, err := reedsolomon.New(default_kappa, default_parity)
			ifErrFatal(err, "reedsolomon encoder creation")

			// now we can recreate the message
			err = enc.Reconstruct(data)
			ifErrFatal(err, "Could not reconstruct data")

			// now the first default_kappa elements of data is the message! :)
			log.Println("Message succesfully recreated!")

			// send success message to coordinator
			msg := Msg{"IDASuccess", idaMsg.MerkleRoot, self.ID}
			dialAndSend("127.0.0.1:8080", msg)
			gossipSend(idaMsg, currentCommittee, currentNeighbours, self.ID)
			return idaMsg.MerkleRoot, data
		}

		gossipSend(idaMsg, currentCommittee, currentNeighbours, self.ID)

	}
	return [32]byte{}, nil
}

func gossipSend(msg IDAGossipMsg, currentCommittee *Committee, currentNeighbours *[]uint, selfID uint) {
	// If we do not have enough chunks then gossip the message to all neighbours
	msgs := make([]Msg, len(*currentNeighbours))

	for i := range msgs {
		msgs[i] = Msg{"IDAGossipMsg", msg, selfID}
	}

	// send each msg to node
	for i, msg := range msgs {
		var addr string = currentCommittee.Members[(*currentNeighbours)[i]].IP
		//log.Printf("addr: %s\n", addr)
		go dialAndSend(addr, msg)
	}
}
