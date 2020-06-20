package main

import (
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

func genGenesisBlock(flagArgs *FlagArgs, users *[]PrivKey) *Block {

	per := uint(float64(flagArgs.totalCoins) / float64(flagArgs.nUsers))

	// genesis transactions
	genesisTx := Transaction{}
	genesisTx.Outputs = make([]OutTx, len(*users))
	for i, u := range *users {
		tx := OutTx{}
		tx.Value = per
		tx.N = uint(i)
		tx.PubKey = u.Pub
		genesisTx.Outputs[i] = tx
	}
	genesisTx.setHash()

	genesisBlock := Block{}
	genesisBlock.PreviousHash = [32]byte{}
	genesisBlock.Signatures = nil
	// since there is only one transaction set merkle root to hash of gensisTx
	genesisBlock.BlockHeader = BlockHeader{}
	genesisBlock.BlockHeader.IdaRoot = genesisTx.Hash
	genesisBlock.Transactions = []Transaction{genesisTx}
	// since the only thing in this block is the genesis tx, use that hash
	genesisBlock.Hash = genesisTx.Hash

	//fmt.Println("Block: ", genesisBlock)
	return &genesisBlock
}

func txGenerator(flagArgs *FlagArgs, allNodes []NodeAllInfo, users *[]PrivKey, gensisBlock *Block) {
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
	genTxHash := gensisBlock.Transactions[0].Hash
	outputs := gensisBlock.Transactions[0].Outputs
	for i := range outputs {
		set.add(genTxHash, &outputs[i])
		userSets[outputs[i].PubKey.Bytes].add(genTxHash, &outputs[i])
	}

	i := 0
	time.Sleep(3 * time.Second)
	for {
		if i > 3 {
			return
		}
		go _txGenerator(flagArgs, &allNodes, users, set, &userSets)
		dur := time.Second / time.Duration(flagArgs.tps)
		// fmt.Println("Sleeping for ", dur)
		time.Sleep(dur)
		i++
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

	// pick random user to send transaction to, aka output of tx
	rndTo := rand.Intn(len(*users))
	userTo := (*users)[rndTo].Pub

	// create transaction
	totalInputValue := uint(0)
	t := Transaction{}
	inputs := make([]InTx, len(*outputs))
	for i, o := range *outputs {
		// create sig of txID || n || pubkey
		outTx := userSet.getAndRemove(o.txID, o.n)
		// todo remove from overall set aswell
		totalInputValue += outTx.Value
		hashToSig := hash(byteSliceAppend(o.txID[:], getBytes(o.n), outTx.PubKey.Bytes[:]))
		sig := user.sign(hashToSig)
		inputs[i] = InTx{o.txID, o.n, sig}
	}

	// only one or two outputs (if there is some value left over, then send it back to user)
	txOutputs := []OutTx{}
	txOutputs = append(txOutputs, OutTx{valueToSend, 0, userTo})
	if totalInputValue != valueToSend {
		// there is a rest that we must send back to user
		rest := totalInputValue - valueToSend
		if rest <= 0 {
			errFatal(nil, "rest was not positive")
		}
		txOutputs = append(txOutputs, OutTx{rest, 1, user.Pub})
	}
	t.Inputs = inputs
	t.Outputs = txOutputs
	t.setHash()
	t.Sig = user.sign(t.Hash)

	// pick random node to send tx to
	rndNode := rand.Intn(len(*allNodes))
	node := (*allNodes)[rndNode]

	// send transaction
	msg := Msg{"transaction", t, user.Pub}
	go dialAndSend(node.IP, msg)

	// manually add the output to userTo
	(*userSets)[userTo.Bytes].add(t.Hash, &txOutputs[0])

	log.Println("Sent tx")
}
