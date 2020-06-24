package main

/*
Train of thoughts (delete later):

Discussion:
	Value is not know to C_out using standard bitcoin UTXO model

	Cross-tx must indicate that it is not supposed to be spendable in C_in after inclusion in C_in's blockchain
	The entire output should be owned and only spendable in C_out (aka OrigTxId committee)

	After cross-tx is included in the blockchain in the C_in committees (aka it has undergone consensus) it is included in a block that has votes
	C_out must be able to verify that the transactions sent back from C_in are valid. Otherwise, any leader (and member?) could just forge the transactions and potentially doublespend.
	The only way for C_out to verify the cross-tx is therefor to have mf+1 valid signatures on the block. To known that these signatures are valid. C_out must known the public keys of
	the members of C_in. The reconfiguration block solves this.

	The set of signatures using normal ECDSA is quite huge, but it have to be done (increasing the argument for BLS aggregate signtures), but the set only has to be
	sent with the batched transactions to that committee.

	The signatures only sign the GossipHash of the proposed block. So how would C_out know that the cross-tx belongs to the block
		Merkle proof of transactions?

	Problem statement conclusion: In any cross-tx protocol. The output committee MUST be able to verify that cross-tx was execectued in C_in.
	In other words, C_out must have Proof of Consensus in C_in.

	Reconfiguration block has CommittteID <-> PubKey pairs therefor the signature set is verifiable to anyone who has the reconfiguration block

	What could proof of consensus lock like?
		1. Simplest
			Data
				Entire block
				Signature set that signed the block
			C_out verify process
				Ensure that signatures belong to that committee using reconfiguration block
				Check that we have mf+1 valid signatures
				C_out now knows the validity of cross-tx since entire block is transmitted and therefor any cross-tx is easily found in that block.
		2. harder
			Data
				Block without transactions // transactions are verifiable trough merkle-root so this is ok
				Merkle proof of cross-tx
				Signature set that signed the block
			C_out verify process
				Same as 1. but where merkle-proof is also verified.
		3. hardest
			Data
				GossipHash [gh]
				MerkleRoot [mr]
				MerkleProof of crossTx [mp]
				Hash of everything else [ha]
				Signature set
			Note:
				This requires a hashing function where hash(ha, mr) = gh
				If we change the hasing process this is possible
			C_out verify process
				Same as 2. but where hash(ha,mr) = gh is enough to verify block. and signatures ofc

	Question: How would C_out include the proof for crossTx so that any member can recreate the blockchain from scratch.
		Include all data of 3. in the block? This will significantly expand size.


	If some cross-tx fails, how would we release the cross-tx outputs to owner?
		Maybe they should always be spendable?
			So a user could potentially spend cross_tx1 before cross_tx2 finnished. orig_tx would then be invalid.
		Timeout?
			Spendable after iteration i+10 for example or something.

	Since C_1 does not know the value or the public key/owner of the Input it cannot create an ouput
	Therefor, a cross-tx must have no Outputs.
	This will be the definer for a cross-tx
	The cross-tx is only spendable in C_1 because C_1 is the closest committee to TxID
	And the new cross-tx will not have a different TxID.
		This is still verifyied by the owner of the transaction since the inputs TxID and the transactions TxID is signed by the owner
		Therefor output committee cannot change the outputs or the original Tx because it is recorded in TxID
	You could have a cross-tx imidatly have an output to several or one of the actual outputs
		But this will change the original TxID, and therefor the user has not signed that new txid

	There is one or more(!) inputs in each cross-tx, but no outputs

	NOTE: If OrigTxId != nil, this is where the transaction belongs and not using TxId

	problem: The rest of the committee needs to know if the transaction has been split into cross-tx
	and sent, so next leaders don't do the same
		Possible solutions:
			- Add transaction to block, but add a bool to say this transaction is not valid
			- Add to a second "pending transactions" list that is added to the block
			- Add to a second "pending TxIDs" list that is added to the block
			- Set TxID to nil to indicate that it is not spendable.

Example

// if TxID == nil, then the tx is not valid.
Orig transacrtions from user_1 that is added to block
	TxID 		nil
	OrigTxId	a2c			// C_1
	Inputs
		0	// value 25
			TxId 			ef2		// C_3
			OrigInputTxId	nil
			N				2
			Sig				of ef2 and a2c
		1	// value 20
			TxId			zb4		// C_2
			OrigInputTxId	nil
			N 				1
			Sig				of ef2 and a2c
	Outputs
		0
			Value 	30
			N		0
			PubKey	user_2
		1
			Value 15
			N 		1
			PubKey  user_1 // rest back to self
	ProofOfConsensus nil

cross-tx 2 ommitted from example

Cross-tx 1 to C_3
	TxId 		nil
	OrigTxId	a2c		// C_1
	Inputs
		0	// value 25 this is know to C_3
			TxId 			nil		// C_3
			OrigInputTxID	ef2		// C_3
			N				2
			Sig				of ef2 and a2c
	Outputs			nil
	ProofOfConsensus nil

Response to C_1 from C_3
	Cross-tx1-r
		TxId		skr  // UNSIGNED!
		OrigTxId 	a2c	 // C_1
		Inputs
			0
				TxId 			nil		// C_3
				OrigInputTxId	ef2		// C_3
				N				2
				Sig				Of ef2 and a2c
		Outputs // Problem, hash of tx is not TxID, therefor wee need an identifier to see if it is a cross-tx
			0
				Value	25
				N		0
				PubKey  Owner
		Proof-Of-Consensus
			BlockRepresentation
				GossipHash
				IntermediateHash
				MerkleRoot
				MerkleProof of Cross-tx
			Signature set

cross-tx1-r should be put into the blockchain so it can be used by user if cross-tx2 fails.

When including a cross-tx in a block. A leader (and all nodes) check wheter or not this was the last tx to
fulfill the original Tx. If it was leader should make the final transaction (and if he does not, honest node should not accept the block).

Final transaction given that cross-tx1-r and cross-tx2-r have been recived (and possibly already added to block)

Final transaction
	TxID 		qxk		// unsigned
	OrigTxId	a2c
	Inputs
		0	// value 25
			TxId 			skr		// cross-tx1-r TxId unsigned
			OrigInputTxId	ef2
			N				0
			Sig				Sig of ef2 and a2c
		1	// value 20
			TxId			abc		// cross-tx2-r TxID unsigned
			OrigInputTxId	zb4
			N 				0
			Sig				Sig of zb4 and a2c
	Outputs
		0
			Value 	30
			N		0
			PubKey	user_2
		1
			Value 15
			N 		1
			PubKey  user_1 // rest back to self

Added to blockchain and done! abc and skr are validated because they are in the blockchain and its proofOfCosnensus
is validated. And all is good!

QED


So when leader is creating a block
1. pop all transactions from tx pool
2. all new transactions that require cross-tx
	2.1 Create new transactions (cross-tx) to each input committee
	2.2 Batch these cross-tx together and send to target committees
	2.3 Original transactions TxID moved to OriginalTxID to indicate it is not spendable
3. If there is an incomming cross-tx-r then check if it is the last cross-tx-r to fulfill orig tx
	3.1 If it is, also create final tx and add it to the block after cross-tx-r
4. Add all transactions, original (not-spendable) transactions, cross-tx transactions, and final transactions to block
5. ??
6. Profit! (given an incentive scheme :P)

*/
