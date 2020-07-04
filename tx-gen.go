package main

import (
	"fmt"
	"math/rand"
	"time"
)

func genUsers(flagArgs *FlagArgs) *[]PrivKey {
	users := make([]PrivKey, flagArgs.nUsers)
	for i := range users {
		privKey := PrivKey{}
		privKey.gen()
		users[i] = privKey
	}

	return &users
}

func genGenesisBlock(flagArgs *FlagArgs, committeeInfos []committeeInfo, users *[]PrivKey) []*FinalBlock {

	per := uint(float64(flagArgs.totalCoins) / float64(flagArgs.nUsers))

	ctx := new(NodeCtx)
	ctx.committeeList = make([][32]byte, len(committeeInfos))
	for i, c := range committeeInfos {
		ctx.committeeList[i] = c.id
	}

	finalBlocks := make([]*FinalBlock, len(committeeInfos))

	// genesis block, one per committtee
	for i := range committeeInfos {

		genesisTx := new(Transaction)
		genesisTx.Outputs = make([]*OutTx, len(*users))
		for j, u := range *users {
			tx := new(OutTx)
			tx.Value = per
			tx.N = uint(j)
			tx.PubKey = u.Pub
			genesisTx.Outputs[j] = tx
		}

		txHash := [32]byte{}
		// need to set a txHash that will point to the committee
		for {
			tmpp := make([]byte, 32)
			rand.Read(tmpp)
			tmp := hash(tmpp)

			if committeeInfos[i].id == txFindClosestCommittee(ctx, tmp) {
				fmt.Println("Closets committee ", bytes32ToString(txFindClosestCommittee(ctx, tmp)), bytes32ToString(committeeInfos[i].id))
				txHash = tmp
				break
			}
		}
		genesisTx.Hash = txHash

		fmt.Println(genesisTx)

		// genesisTx.setHash()
		genesisBlock := new(ProposedBlock)
		// since there is only one transaction set merkle root to hash of gensisTx
		genesisBlock.MerkleRoot = genesisTx.Hash
		genesisBlock.Transactions = []*Transaction{genesisTx}
		genesisBlock.CommitteeID = committeeInfos[i].id
		// since the only thing in this block is the genesis tx, use that hash
		// genesisBlock.GossipHash = genesisTx.Hash
		genesisBlock.GossipHash = txHash
		genesisFinalBlock := new(FinalBlock)
		genesisFinalBlock.ProposedBlock = genesisBlock
		finalBlocks[i] = genesisFinalBlock

		fmt.Println(i, genesisFinalBlock)
	}
	//fmt.Println("Block: ", genesisBlock)
	// fmt.Println(finalBlocks)
	return finalBlocks
}

func txGenerator(flagArgs *FlagArgs, allNodes []NodeAllInfo, users *[]PrivKey, gensisBlocks []*FinalBlock, finalBlockChan chan FinalBlock) {
	// Emulates users by continously generating transactions

	if flagArgs.tps == 0 {
		return
	}

	// make a UTXO set for each user, such that we can easily look up UTXO for each user
	userSets := make(map[[32]byte]*UTXOSet)
	for _, u := range *users {
		id := u.Pub.Bytes
		userSets[id] = new(UTXOSet)
		userSets[id].init()
	}

	// add gensis block output to the main UTXO set
	for _, b := range gensisBlocks {
		for _, out := range b.ProposedBlock.Transactions[0].Outputs {
			userSets[out.PubKey.Bytes].add(b.ProposedBlock.Transactions[0].Hash, out)
		}
	}

	// find committee id list and  add it to nodeCtx
	cMap := make(map[[32]byte]bool)
	for _, node := range allNodes {
		cMap[node.CommitteeID] = true
	}
	cList := make([][32]byte, len(cMap))
	ii := 0
	for k := range cMap {
		cList[ii] = k
		ii++
	}
	nodeCtx := new(NodeCtx)
	nodeCtx.committeeList = cList

	i := 0
	time.Sleep(3 * time.Second)
	rand.Seed(42)
	for {
		l := len(finalBlockChan)
		for i := 0; i < l; i++ {
			fmt.Println("Recived finalblock")
			finalBlock := <-finalBlockChan
			fmt.Println(finalBlock.ProposedBlock)

			for _, t := range finalBlock.ProposedBlock.Transactions {
				if t.Hash == [32]byte{} && t.OrigTxHash != [32]byte{} && t.Outputs == nil {
					fmt.Println("crosstx")
					// return "crosstx"
					continue
				} else if t.Hash == [32]byte{} && t.OrigTxHash != [32]byte{} && t.Outputs != nil {
					// return "originaltx"
					fmt.Println("originaltx")
					continue
				} else if t.Hash != [32]byte{} && t.OrigTxHash != [32]byte{} && txFindClosestCommittee(nodeCtx, t.OrigTxHash) != finalBlock.ProposedBlock.CommitteeID {
					// return "crosstxresponse"
					fmt.Println("crosstxresponse_C_in")
					continue
				} else if t.Hash != [32]byte{} && t.OrigTxHash != [32]byte{} && t.ProofOfConsensus != nil {
					// TODO ADD crosstxresponse_C_out or not
					fmt.Println("crosstxresponse_C_out")
					continue
				}

				fmt.Print("normal or final")
				fmt.Println(t)

				for _, out := range t.Outputs {
					userSets[out.PubKey.Bytes].add(t.ifOrigRetOrigIfNotRetHash(), out)
				}
			}
		}

		_txGenerator(flagArgs, &allNodes, users, userSets)

		// fmt.Println("Sleeping for ", dur)
		time.Sleep(time.Second / time.Duration(flagArgs.tps))
		i++
		// if i == 30 {
		// 	return
		// }
		// if i%10 == 0 {
		// 	tot := 0
		// 	for _, u := range userSets {
		// 		for _, t := range u.set {
		// 			for range t {
		// 				tot++
		// 			}
		// 		}
		// 	}
		// 	fmt.Println(tot)
		// }
	}
}

func _txGenerator(flagArgs *FlagArgs, allNodes *[]NodeAllInfo, users *[]PrivKey, userSets map[[32]byte]*UTXOSet) {
	//fmt.Println("_txGen")

	// pick random user to send transaction from
	rnd := rand.Intn(len(*users))
	user := (*users)[rnd]

	// pick a random value from the users total value
	totVal := userSets[user.Pub.Bytes].totalValue()
	if totVal == 0 {
		// no value in this user unfortuantly, so start again
		_txGenerator(flagArgs, allNodes, users, userSets)
		return
	}
	valueToSend := uint(rand.Intn(int(totVal)) + 1)

	// get all outputs required to fill that value
	outputs, ok := userSets[user.Pub.Bytes].getOutputsToFillValue(valueToSend)
	notOkErr(ok, "not enough output value, but this should not happen")

	// fmt.Println(outputs)

	// pick random user to send transaction to, aka output of tx
	rndTo := rand.Intn(len(*users))
	userTo := (*users)[rndTo].Pub

	// create transaction
	totalInputValue := uint(0)
	t := new(Transaction)
	inputs := make([]*InTx, len(outputs))
	for i, o := range outputs {
		outTx := userSets[user.Pub.Bytes]._getAndRemove(o.txID, o.n)

		totalInputValue += outTx.Value

		newInTx := new(InTx)
		newInTx.TxHash = o.txID
		newInTx.N = outTx.N
		inputs[i] = newInTx
	}

	// only one or two outputs (if there is some value left over, then send it back to user)
	txOutputs := []*OutTx{}
	newOutTx := new(OutTx)
	newOutTx.Value = valueToSend
	newOutTx.N = 0
	newOutTx.PubKey = userTo
	txOutputs = append(txOutputs, newOutTx)
	if totalInputValue != valueToSend {
		// there is a rest that we must send back to user
		rest := totalInputValue - valueToSend
		if rest <= 0 {
			errFatal(nil, "rest was not positive")
		}
		newOutput := new(OutTx)
		newOutput.Value = rest
		newOutput.N = 1
		newOutput.PubKey = user.Pub
		txOutputs = append(txOutputs, newOutput)
	}
	t.Inputs = inputs
	t.Outputs = txOutputs
	t.setHash()
	t.signInputs(&user)

	// fmt.Println("newTx", bytes32ToString(t.Hash), bytes32ToString(t.OrigTxHash), bytes32ToString(t.id()))
	// fmt.Println(bytes32ToString(t.Inputs[0].TxHash), bytes32ToString(t.Inputs[0].OrigTxHash), bytes32ToString(t.id()), t.Inputs[0].N, t.Inputs[0].Sig)
	// fmt.Println(bytes32ToString(t.Outputs[0].PubKey.Bytes), t.Outputs[0].Value, t.Outputs[0].N)

	fmt.Println("Sent tx: ", t)

	// pick random node to send tx to
	rndNode := rand.Intn(len(*allNodes))
	node := (*allNodes)[rndNode]

	// send transaction
	msg := Msg{"transaction", t, user.Pub}
	go dialAndSend(node.IP, msg)

	// add output to sets
	// for _, out := range t.Outputs {
	// 	userSets[out.PubKey.Bytes].add(t.Hash, out)
	// }

	// log.Println("Sent tx")
}
