package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"math/big"
	"sync"
)

// only data structures that are common in multiple, disjoint files, should belong here

type Node_InitialMessageToCoordinator struct {
	Pub  *PubKey
	Port int
}

type SelfInfo struct {
	Priv        *PrivKey
	CommitteeID [32]byte
	IP          string
	IsHonest    bool
	Debug       bool
}

type NodeAllInfo struct {
	Pub         *PubKey
	CommitteeID [32]byte
	IP          string
	IsHonest    bool
}

type PlaceHolder struct {
	t uint
}

type ResponseToNodes struct {
	Nodes            []NodeAllInfo
	GensisisBlock    *Block
	DebugNode        [32]byte
	InitalRandomness int
}

// Representation of a member beloning to the current committee of a node
type CommitteeMember struct {
	Pub *PubKey
	IP  string
}

// Representation of a committee from the point of view of a node
type Committee struct {
	ID       [32]byte
	BigIntID *big.Int
	Members  map[[32]byte]CommitteeMember
}

func (c *Committee) init(ID [32]byte) {
	c.ID = ID
	c.BigIntID = new(big.Int).SetBytes(ID[:])
	c.Members = make(map[[32]byte]CommitteeMember)
}

func (c *Committee) addMember(m CommitteeMember) {
	c.Members[m.Pub.Bytes] = m
}

func (c *Committee) safeAddMember(m CommitteeMember) bool {
	if _, ok := c.Members[m.Pub.Bytes]; !ok {
		return false
	}
	c.addMember(m)
	return true
}

// block header
type ConsensusBlockHeader struct {
	I         uint
	Root      [32]byte
	LeaderPub *PubKey
	Tag       string
}

// Recived transactions that have not been included in a block yet
type TxPool struct {
	pool map[[32]byte]*Transaction // TxHash -> Transaction
	mux  sync.Mutex
}

func (t *TxPool) init() {
	t.pool = make(map[[32]byte]*Transaction)
}

func (t *TxPool) add(tx *Transaction) {
	t.mux.Lock()
	t.pool[tx.Hash] = tx
	t.mux.Unlock()
}

func (t *TxPool) safeAdd(tx *Transaction) bool {
	// only add if there is no transaction with same has
	t.mux.Lock()
	defer t.mux.Unlock()
	if _, ok := t.pool[tx.Hash]; ok {
		return false
	}
	t.pool[tx.Hash] = tx
	return true
}

func (t *TxPool) get(txHash [32]byte) *Transaction {
	t.mux.Lock()
	defer t.mux.Unlock()
	return t.pool[txHash]
}

func (t *TxPool) remove(txHash [32]byte) {
	t.mux.Lock()
	defer t.mux.Unlock()
	delete(t.pool, txHash)
}

func (t *TxPool) pop(txHash [32]byte) (*Transaction, bool) {
	t.mux.Lock()
	defer t.mux.Unlock()
	tx, ok := t.pool[txHash]
	if ok {
		delete(t.pool, txHash)
	}
	return tx, ok
}

func (t *TxPool) popAll() []*Transaction {
	t.mux.Lock()
	defer t.mux.Unlock()
	txes := make([]*Transaction, len(t.pool))
	i := 0
	for _, tx := range t.pool {
		txes[i] = tx
		i++
	}
	t.pool = make(map[[32]byte]*Transaction)
	return txes
}

type UTXOSet struct {
	set map[[32]byte]map[uint]*OutTx // TxID -> Nonce -> OutTx
	mux sync.Mutex
}

func (s *UTXOSet) init() {
	s.set = make(map[[32]byte]map[uint]*OutTx)
}

func (s *UTXOSet) add(k [32]byte, oTx *OutTx) {
	s.mux.Lock()
	if len(s.set[k]) == 0 {
		s.set[k] = make(map[uint]*OutTx)
	}
	s.set[k][oTx.N] = oTx
	s.mux.Unlock()
}

func (s *UTXOSet) removeOutput(k [32]byte, N uint) {
	s.mux.Lock()
	delete(s.set[k], N)
	if len(s.set[k]) == 0 {
		delete(s.set, k)
	}
	s.mux.Unlock()
}

func (s *UTXOSet) get(k [32]byte, N uint) *OutTx {
	s.mux.Lock()
	defer s.mux.Unlock()
	if len(s.set[k]) == 0 {
		return nil
	}
	return s.set[k][N]
}

func (s *UTXOSet) getAndRemove(k [32]byte, N uint) *OutTx {
	s.mux.Lock()
	defer s.mux.Unlock()
	if len(s.set[k]) == 0 {
		return nil
	}
	ret := s.set[k][N]
	delete(s.set[k], N)
	if len(s.set[k]) == 0 {
		delete(s.set, k)
	}
	return ret
}

func (s *UTXOSet) getTxOutputsAsList(k [32]byte) *[]*OutTx {
	s.mux.Lock()
	defer s.mux.Unlock()
	if len(s.set[k]) == 0 {
		return nil
	}
	a := make([]*OutTx, len(s.set[k]))
	i := 0
	for _, v := range s.set[k] {
		a[i] = v
	}
	return &a
}

func (s *UTXOSet) getLenOfEntireSet() int {
	s.mux.Lock()
	defer s.mux.Unlock()
	l := 0
	for k := range s.set {
		l += len(s.set[k])
	}
	return l
}

type txIDNonceTuple struct {
	txID [32]byte
	n    uint
}

func (s *UTXOSet) totalValue() uint {
	// finds the total value that is in the UTXO set, usefull for each user to know their balance
	s.mux.Lock()
	defer s.mux.Unlock()

	var tot uint = 0
	for txID := range s.set {
		for nonce := range s.set[txID] {
			tot += s.set[txID][nonce].Value
		}
	}
	return tot
}

// only to be used if you own all UTXO's
func (s *UTXOSet) getOutputsToFillValue(value uint) (*[]txIDNonceTuple, bool) {
	s.mux.Lock()
	defer s.mux.Unlock()
	res := []txIDNonceTuple{}
	var remV int = int(value)
	for txID := range s.set {
		for nonce := range s.set[txID] {
			v := s.set[txID][nonce].Value
			remV -= int(v)
			if remV > 0 {
				// take entire output
				res = append(res, txIDNonceTuple{txID, nonce})
			} else {
				// take only the required amount
				res = append(res, txIDNonceTuple{txID, nonce})
				return &res, true
			}
		}
	}
	// did not find enough outputs to fill value
	return &res, false
}

type OutTx struct {
	Value  uint // value of UTXO
	N      uint // nonce/i in output list of tx
	PubKey *PubKey
}

func (o *OutTx) bytes() []byte {
	b1 := make([]byte, 8) //uint64 is 8 bytes
	binary.LittleEndian.PutUint64(b1, uint64(o.Value))
	b2 := make([]byte, 8)
	binary.LittleEndian.PutUint64(b2, uint64(o.N))
	b3 := o.PubKey.Bytes
	return byteSliceAppend(b1, b2, b3[:])
}

type InTx struct {
	TxID [32]byte // output in transaction
	N    uint     // nonce in output in transaction
	Sig  *Sig
}

func (i *InTx) bytes() []byte {
	return getBytes(i)
}

type Transaction struct {
	Hash    [32]byte // hash of inputs and outputs
	Inputs  []InTx
	Outputs []OutTx
	Sig     *Sig // sig of hash
}

func (t *Transaction) calculateHash() [32]byte {
	b := []byte{}
	for i := range t.Inputs {
		b = append(b, t.Inputs[i].bytes()...)
	}
	for i := range t.Outputs {
		b = append(b, t.Outputs[i].bytes()...)
	}
	return hash(b)
}

func (t *Transaction) setHash() {
	t.Hash = t.calculateHash()
}

func (t *Transaction) encode() []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(t)
	ifErrFatal(err, "transaction encode")
	return buf.Bytes()
}

func (t *Transaction) decode(b []byte) {
	buf := bytes.NewBuffer(b)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(t)
	ifErrFatal(err, "transaction decode")
}

type ConsensusSignature struct {
	IdaRoot   [32]byte
	Iteration uint
	State     string // echo, accept or pending
	Pub       *PubKey
	Sig       Sig // not hased
}

func (cs *ConsensusSignature) calculateHash() [32]byte {
	b := byteSliceAppend(cs.IdaRoot[:], getBytes(cs.Iteration), []byte(cs.State), cs.Pub.Bytes[:])
	return hash(b)
}

type BlockHeader struct {
	IdaRoot     [32]byte // merkle root of IDA gossip
	Iteration   uint
	CommitteeID [32]byte
	LeaderPub   *PubKey
	LeaderSig   Sig //not hashed
}

func (bh *BlockHeader) calculateHash() [32]byte {
	b := byteSliceAppend(bh.IdaRoot[:], getBytes(bh.Iteration), bh.CommitteeID[:], bh.LeaderPub.Bytes[:])
	return hash(b)
}

type Block struct {
	Hash         [32]byte // ID of this block
	PreviousHash [32]byte // ID of last block
	BlockHeader  BlockHeader
	Signatures   []ConsensusSignature
	Transactions []Transaction // not hashed
	LeaderSig    Sig           // not hashed
}

func (b *Block) calculateHash() [32]byte {
	bh := b.BlockHeader.calculateHash()
	bts := []byte{}
	for i := range b.Signatures {
		sigBytes := b.Signatures[i].calculateHash()
		sigBytes2 := sigBytes[:]
		bts = append(bts, sigBytes2...)
	}
	toH := byteSliceAppend(b.PreviousHash[:], bh[:], bts)
	return hash(toH)
}

func (b *Block) setHash() {
	b.Hash = b.calculateHash()
}

func (b *Block) encode() []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(b)
	ifErrFatal(err, "transaction encode")
	return buf.Bytes()
}

func (b *Block) decode(bArr []byte) {
	buf := bytes.NewBuffer(bArr)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(b)
	ifErrFatal(err, "transaction decode")
}

// Todo define, extend and create reconfiguration block
type ReconfigurationBlock struct {
	Hash        [32]byte
	CommitteeID [32]byte
	Members     map[[32]byte]*CommitteeMember
}

// routing table
type RoutingTable struct {
	l   []Committee // Your known commites, sorted by distance
	mux sync.Mutex
}

func (r *RoutingTable) init(length int) {
	r.mux.Lock()
	r.l = make([]Committee, length)
	r.mux.Unlock()
}

func (r *RoutingTable) addCommittee(i uint, ID [32]byte) {
	r.mux.Lock()
	r.l[i] = Committee{}
	r.l[i].init(ID)
	r.mux.Unlock()
}

func (r *RoutingTable) addMember(i uint, cm CommitteeMember) {
	r.mux.Lock()
	r.l[i].addMember(cm)
	r.mux.Unlock()
}

func (r *RoutingTable) get() []Committee {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.l
}

type KademliaFindNodeMsg struct {
	ID [32]byte
}

type KademliaFindNodeResponse struct {
	Committee Committee
}

type IdaMsgs struct {
	m   map[[32]byte][]IDAGossipMsg
	mux sync.Mutex
}

func (ida *IdaMsgs) init() {
	ida.m = make(map[[32]byte][]IDAGossipMsg)
}

func (ida *IdaMsgs) isArr(root [32]byte) bool {
	ida.mux.Lock()
	defer ida.mux.Unlock()
	_, ok := ida.m[root]
	return ok
}

func (ida *IdaMsgs) add(root [32]byte, m IDAGossipMsg) {
	ida.mux.Lock()
	if arr, ok := ida.m[root]; !ok {
		ida.m[root] = []IDAGossipMsg{m}
	} else {
		ida.m[root] = append(arr, m)
	}
	ida.mux.Unlock()
}

func (ida *IdaMsgs) getMsg(root [32]byte, index uint) IDAGossipMsg {
	ida.mux.Lock()
	defer ida.mux.Unlock()
	return ida.m[root][index]
}

func (ida *IdaMsgs) getMsgs(root [32]byte) []IDAGossipMsg {
	ida.mux.Lock()
	defer ida.mux.Unlock()
	return ida.m[root]
}

func (ida *IdaMsgs) getLenOfChunks(root [32]byte) int {
	ida.mux.Lock()
	defer ida.mux.Unlock()
	var totalChunks int
	for _, msg := range ida.m[root] {
		totalChunks += len(msg.Chunks)
	}
	return totalChunks
}

func (ida *IdaMsgs) getLen() uint {
	ida.mux.Lock()
	defer ida.mux.Unlock()
	return uint(len(ida.m))
}

type ReconstructedIdaMsgs struct {
	m   map[[32]byte][][]byte
	mux sync.Mutex
}

func (b *ReconstructedIdaMsgs) init() {
	b.m = make(map[[32]byte][][]byte)
}

func (b *ReconstructedIdaMsgs) keyExists(root [32]byte) bool {
	b.mux.Lock()
	defer b.mux.Unlock()
	_, ok := b.m[root]
	return ok
}

func (b *ReconstructedIdaMsgs) add(root [32]byte, block [][]byte) {
	b.mux.Lock()
	b.m[root] = block
	b.mux.Unlock()
}

func (b *ReconstructedIdaMsgs) safeAdd(root [32]byte, block [][]byte) bool {
	b.mux.Lock()
	defer b.mux.Unlock()
	if _, ok := b.m[root]; ok {
		return false
	}
	b.m[root] = block
	return true
}

func (b *ReconstructedIdaMsgs) get(root [32]byte) [][]byte {
	b.mux.Lock()
	defer b.mux.Unlock()
	return b.m[root]
}

func (b *ReconstructedIdaMsgs) pop(root [32]byte) [][]byte {
	b.mux.Lock()
	defer b.mux.Unlock()
	ret := b.m[root]
	delete(b.m, root)
	return ret
}

func (b *ReconstructedIdaMsgs) getLen() uint {
	b.mux.Lock()
	defer b.mux.Unlock()
	return uint(len(b.m))
}

func (b *ReconstructedIdaMsgs) getData(root [32]byte) []byte {
	b.mux.Lock()
	defer b.mux.Unlock()
	// get data, flatten it, and unpadd
	data := b.m[root][:default_kappa]
	bArr := []byte{}
	for _, chunk := range data {
		bArr = append(bArr, chunk...)
	}

	if isPadded(bArr) {
		return unPad(bArr)
	}
	return bArr
}

func (b *ReconstructedIdaMsgs) popData(root [32]byte) []byte {
	b.mux.Lock()
	defer b.mux.Unlock()
	data := b.m[root][:default_kappa]
	bArr := []byte{}
	for _, chunk := range data {
		bArr = append(bArr, chunk...)
	}
	delete(b.m, root)
	if isPadded(bArr) {
		return unPad(bArr)
	}
	return bArr
}

type ConsensusMsgs struct {
	m   map[uint]map[[32]byte]ConsensusBlockHeader // iter -> fromID -> msg
	mux sync.Mutex
}

func (cMsg *ConsensusMsgs) init() {
	cMsg.m = make(map[uint]map[[32]byte]ConsensusBlockHeader)
}

func (cMsg *ConsensusMsgs) initIter(i uint) {
	cMsg.mux.Lock()
	defer cMsg.mux.Unlock()
	cMsg.m[i] = make(map[[32]byte]ConsensusBlockHeader)
}

func (cMsg *ConsensusMsgs) iterationExists(i uint) bool {
	cMsg.mux.Lock()
	defer cMsg.mux.Unlock()
	if cMsg.m[i] == nil {
		return true
	}
	return false
}

func (cMsg *ConsensusMsgs) blockHeaderExists(i uint, ID [32]byte) bool {
	cMsg.mux.Lock()
	defer cMsg.mux.Unlock()
	_, ok := cMsg.m[i][ID]
	return ok
}

func (cMsg *ConsensusMsgs) add(i uint, ID [32]byte, header ConsensusBlockHeader) {
	cMsg.mux.Lock()
	cMsg.m[i][ID] = header
	cMsg.mux.Unlock()
}

func (cMsg *ConsensusMsgs) safeAdd(i uint, ID [32]byte, header ConsensusBlockHeader, errMsg string) {
	cMsg.mux.Lock()
	if cMsg.m[i] == nil {
		cMsg.m[i] = make(map[[32]byte]ConsensusBlockHeader)
	}

	cMsg.m[i][ID] = header
	cMsg.mux.Unlock()
}

func (cMsg *ConsensusMsgs) getBlockHeader(i uint, ID [32]byte) ConsensusBlockHeader {
	cMsg.mux.Lock()
	defer cMsg.mux.Unlock()
	return cMsg.m[i][ID]
}

func (cMsg *ConsensusMsgs) getBlockHeaders(i uint) []ConsensusBlockHeader {
	cMsg.mux.Lock()
	defer cMsg.mux.Unlock()

	blockHeaders := make([]ConsensusBlockHeader, len(cMsg.m[i]))
	iter := 0
	for _, v := range cMsg.m[i] {
		blockHeaders[iter] = v
		iter++
	}
	return blockHeaders
}

func (cMsg *ConsensusMsgs) getLen(i uint) uint {
	cMsg.mux.Lock()
	defer cMsg.mux.Unlock()
	return uint(len(cMsg.m[i]))
}

type Channels struct {
	echoChan chan bool
}

func (c *Channels) init(l int) {
	c.echoChan = make(chan bool, l)
}

type CurrentIteration struct {
	i   uint
	mux sync.Mutex
}

func (c *CurrentIteration) add() {
	c.mux.Lock()
	c.i++
	c.mux.Unlock()
}

func (c *CurrentIteration) getI() uint {
	c.mux.Lock()
	defer c.mux.Unlock()
	return c.i
}

// context for a node, to be passed everywhere, acts like a global var
type NodeCtx struct {
	flagArgs             FlagArgs
	committee            Committee  // current committee
	neighbors            [][32]byte // neighboring nodes
	self                 SelfInfo
	allInfo              map[[32]byte]NodeAllInfo // cheat variable for easy testing
	idaMsgs              IdaMsgs
	reconstructedIdaMsgs ReconstructedIdaMsgs
	consensusMsgs        ConsensusMsgs
	channels             Channels
	i                    CurrentIteration
	routingTable         RoutingTable
	committeeList        [][32]byte //list of all committee ids, to be replaced with reference block?
	txPool               TxPool
}

// generic msg. typ indicates which struct to decode msg to.
type Msg struct {
	Typ     string
	Msg     interface{}
	FromPub *PubKey
}
