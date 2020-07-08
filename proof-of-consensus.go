package main

import (
	"fmt"
	"time"

	"github.com/kimborgen/go-merkletree"
)

func createMerkleTree(nodeCtx *NodeCtx, transactions []*Transaction) *merkletree.MerkleTree {
	data := make([][]byte, len(transactions))

	for i, tx := range transactions {
		tmp := tx.ifOrigRetOrigIfNotRetHash()
		data[i] = tmp[:]
	}

	tree, err := merkletree.New(data)
	ifErrFatal(err, "creating merkle tree")

	return tree
}

func addProofOfConsensus(nodeCtx *NodeCtx, t *Transaction, finalBlock *FinalBlock) {

	before := time.Now()
	t.ProofOfConsensus = new(ProofOfConsensus)
	t.ProofOfConsensus.GossipHash = finalBlock.ProposedBlock.GossipHash
	t.ProofOfConsensus.IntermediateHash = finalBlock.ProposedBlock.calculateHashExceptMerkleRoot()

	t.ProofOfConsensus.Signatures = finalBlock.Signatures

	// create a merkle tree based on transaction hashes

	indexOfT := -1

	tree := createMerkleTree(nodeCtx, finalBlock.ProposedBlock.Transactions)

	for i, tx := range finalBlock.ProposedBlock.Transactions {
		if tx.ifOrigRetOrigIfNotRetHash() == t.ifOrigRetOrigIfNotRetHash() {
			indexOfT = i
		}
	}

	if indexOfT == -1 {
		errFatal(nil, "index of transacton not found")
	}

	root := tree.Root()
	//fmt.Println("Root len: ", len(root))
	root32 := toByte32(root)

	if root32 != finalBlock.ProposedBlock.MerkleRoot {
		errFatal(nil, fmt.Sprintf("Proposedblock merkleroot: %s, was not equal to calculated merkleroot: %s", bytes32ToString(finalBlock.ProposedBlock.MerkleRoot), bytes32ToString(root32)))
	}

	proof, err := tree.GenerateProofUsingIndex(uint64(indexOfT), 0)
	ifErrFatal(err, "generating proof for tx in addProofOfConsensus")

	// fmt.Println("root: ", bytes32ToString(root32))
	// fmt.Println("proof: ", proof)

	t.ProofOfConsensus.MerkleRoot = root32
	t.ProofOfConsensus.MerkleProof = proof

	dur := time.Now().Sub(before)
	go dialAndSendToCoordinator("pocadd", dur)

	// fmt.Println("new tx PoC : ", t)
}
