package main

import (
	"fmt"
	"log"
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

func genGenesisBlock(flagArgs *FlagArgs, committeeInfos []committeeInfo, users *[]PrivKey) []FinalBlock {

	per := uint(float64(flagArgs.totalCoins) / float64(flagArgs.nUsers))

	ctx := new(NodeCtx)
	ctx.committeeList = make([][32]byte, len(committeeInfos))
	for i, c := range committeeInfos {
		ctx.committeeList[i] = c.id
	}

	finalBlocks := make([]FinalBlock, len(committeeInfos))

	// genesis block, one per committtee
	for i := range committeeInfos {

		genesisTx := Transaction{}
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
			tmp := toByte32(tmpp)

			if committeeInfos[i].id == txFindClosestCommittee(ctx, tmp) {
				txHash = tmp
				break
			}
		}
		genesisTx.Hash = txHash
		// genesisTx.setHash()
		genesisBlock := new(ProposedBlock)
		// since there is only one transaction set merkle root to hash of gensisTx
		genesisBlock.MerkleRoot = genesisTx.Hash
		genesisBlock.Transactions = []*Transaction{&genesisTx}
		genesisBlock.CommitteeID = committeeInfos[i].id
		// since the only thing in this block is the genesis tx, use that hash
		// genesisBlock.GossipHash = genesisTx.Hash
		genesisBlock.GossipHash = txHash
		genesisFinalBlock := new(FinalBlock)
		genesisFinalBlock.ProposedBlock = *genesisBlock
		finalBlocks[i] = *genesisFinalBlock

	}
	//fmt.Println("Block: ", genesisBlock)
	// fmt.Println(finalBlocks)
	return finalBlocks
}

func txGenerator(flagArgs *FlagArgs, allNodes []NodeAllInfo, users *[]PrivKey, gensisBlocks []FinalBlock) {
	// Emulates users by continously generating transactions

	if flagArgs.tps == 0 {
		return
	}

	// generate and send transaction loop
	set := new(UTXOSet)
	set.init()

	// make a UTXO set for each user, such that we can easily look up UTXO for each user
	userSets := make(map[[32]byte]*UTXOSet)
	for _, u := range *users {
		id := u.Pub.Bytes
		userSets[id] = new(UTXOSet)
		userSets[id].init()
	}

	// add gensis block output to the main UTXO set
	for _, b := range gensisBlocks {
		for _, t := range b.ProposedBlock.Transactions[0].Outputs {
			set.add(b.ProposedBlock.GossipHash, t)
			userSets[t.PubKey.Bytes].add(b.ProposedBlock.GossipHash, t)
		}
	}

	i := 0
	time.Sleep(3 * time.Second)
	rand.Seed(42)
	for {
		go _txGenerator(flagArgs, &allNodes, users, set, &userSets)
		dur := time.Second / time.Duration(flagArgs.tps)
		// fmt.Println("Sleeping for ", dur)
		time.Sleep(dur)
		i++
		// if i == 30 {
		// 	return
		// }
	}
}

func _txGenerator(flagArgs *FlagArgs, allNodes *[]NodeAllInfo, users *[]PrivKey, set *UTXOSet, userSets *map[[32]byte]*UTXOSet) {
	//fmt.Println("_txGen")

	// pick random user to send transaction from
	rnd := rand.Intn(len(*users))
	user := (*users)[rnd]

	// pick a random value from the users total value
	userSet := (*userSets)[user.Pub.Bytes]
	totVal := userSet.totalValue()
	if totVal == 0 {
		// no value in this user unfortuantly, so start again
		_txGenerator(flagArgs, allNodes, users, set, userSets)
		return
	}
	valueToSend := uint(rand.Intn(int(totVal)) + 1)

	// get all outputs required to fill that value
	outputs, ok := userSet.getOutputsToFillValue(valueToSend)
	notOkErr(ok, "not enough output value, but this should not happen")

	fmt.Println(outputs)

	// pick random user to send transaction to, aka output of tx
	rndTo := rand.Intn(len(*users))
	userTo := (*users)[rndTo].Pub

	// create transaction
	totalInputValue := uint(0)
	t := new(Transaction)
	inputs := make([]*InTx, len(*outputs))
	for i, o := range *outputs {
		// create sig of txID || n || pubkey
		outTx := userSet.getAndRemove(o.txID, o.n)
		// todo remove from overall set aswell
		totalInputValue += outTx.Value

		newInTx := new(InTx)
		newInTx.TxHash = o.txID
		newInTx.OrigTxHash = [32]byte{}
		newInTx.N = o.n
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

	// pick random node to send tx to
	rndNode := rand.Intn(len(*allNodes))
	node := (*allNodes)[rndNode]

	// send transaction
	msg := Msg{"transaction", t, user.Pub}
	go dialAndSend(node.IP, msg)

	// manually add the output to userTo
	(*userSets)[userTo.Bytes].add(t.Hash, txOutputs[0])

	log.Println("Sent tx")
}
