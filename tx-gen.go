package main

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
		tx.PubKey = u.pub()
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

func genTx() {

}

func txGenerator(flagArgs *FlagArgs, allNodes []NodeAllInfo) {
	// Emulates users by bootstrapping UTXO set by genesis block and continously generate transactions

	// establish system parameters

	// Generate identites
	//curve := elliptic.P256() // use the secp256r1 curce that has comparable security to bitcoins secp256k1 but may be backdoored
	//users := make([]ecdsa.PrivateKey, nUsers)
	//for i := range users {
	//
	//}

	// generate genesis block -> All above identites should have an UTXO with some value

	// send genesis block to all nodes

	// generate and send transaction loop
}
