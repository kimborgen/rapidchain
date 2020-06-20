package main

import (
	"encoding/binary"
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

type UTXOSet struct {
	set map[[32]byte]*OutTx
}

func (s *UTXOSet) init() {
	s.set = make(map[[32]byte]*OutTx)
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
	Sig  Sig
}

func (i *InTx) bytes() []byte {
	return getBytes(i)
}

type Transaction struct {
	Hash    [32]byte // hash of inputs and outputs
	Inputs  []InTx
	Outputs []OutTx
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

type Blocks struct {
	m   map[[32]byte][][]byte
	mux sync.Mutex
}

func (b *Blocks) init() {
	b.m = make(map[[32]byte][][]byte)
}

func (b *Blocks) isBlock(root [32]byte) bool {
	b.mux.Lock()
	defer b.mux.Unlock()
	_, ok := b.m[root]
	return ok
}

func (b *Blocks) add(root [32]byte, block [][]byte) {
	b.mux.Lock()
	b.m[root] = block
	b.mux.Unlock()
}

func (b *Blocks) safeAdd(root [32]byte, block [][]byte) bool {
	b.mux.Lock()
	defer b.mux.Unlock()
	if _, ok := b.m[root]; ok {
		return false
	}
	b.m[root] = block
	return true
}

func (b *Blocks) getBlock(root [32]byte) [][]byte {
	b.mux.Lock()
	defer b.mux.Unlock()
	return b.m[root]
}

func (b *Blocks) getLen() uint {
	b.mux.Lock()
	defer b.mux.Unlock()
	return uint(len(b.m))
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
	flagArgs      FlagArgs
	committee     Committee
	neighbors     [][32]byte
	self          SelfInfo
	allInfo       map[[32]byte]NodeAllInfo
	idaMsgs       IdaMsgs
	blocks        Blocks
	consensusMsgs ConsensusMsgs
	channels      Channels
	i             CurrentIteration
	routingTable  RoutingTable
}

// generic msg. typ indicates which struct to decode msg to.
type Msg struct {
	Typ     string
	Msg     interface{}
	FromPub *PubKey
}
