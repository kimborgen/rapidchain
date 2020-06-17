package main

import "sync"

// only data structures that are common in multiple, disjoint files, should belong here

type Node_InitialMessageToCoordinator struct {
	ID   uint
	Port int
}

type NodeAllInfo struct {
	ID          uint
	CommitteeID uint
	IP          string
	IsHonest    bool
}

type ResponseToNodes struct {
	Nodes            []NodeAllInfo
	InitalRandomness int
}

// Representation of a member beloning to the current committee of a node
type CommitteeMember struct {
	ID uint
	IP string
}

// Representation of a committee from the point of view of a node
type Committee struct {
	ID      uint
	Members map[uint]CommitteeMember
}

// Cheat data to know all nodes in the system
type AllInfo struct {
	self  NodeAllInfo
	nodes map[uint]NodeAllInfo
}

// block header
type BlockHeader struct {
	I        uint
	Root     [32]byte
	LeaderID uint
	Tag      string
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
	m   map[uint]map[uint]BlockHeader // iter -> fromID -> msg
	mux sync.Mutex
}

func (cMsg *ConsensusMsgs) init() {
	cMsg.m = make(map[uint]map[uint]BlockHeader)
}

func (cMsg *ConsensusMsgs) initIter(i uint) {
	cMsg.mux.Lock()
	defer cMsg.mux.Unlock()
	cMsg.m[i] = make(map[uint]BlockHeader)
}

func (cMsg *ConsensusMsgs) iterationExists(i uint) bool {
	cMsg.mux.Lock()
	defer cMsg.mux.Unlock()
	if cMsg.m[i] == nil {
		return true
	}
	return false
}

func (cMsg *ConsensusMsgs) blockHeaderExists(i, ID uint) bool {
	cMsg.mux.Lock()
	defer cMsg.mux.Unlock()
	_, ok := cMsg.m[i][ID]
	return ok
}

func (cMsg *ConsensusMsgs) add(i, ID uint, header BlockHeader) {
	cMsg.mux.Lock()
	cMsg.m[i][ID] = header
	cMsg.mux.Unlock()
}

func (cMsg *ConsensusMsgs) safeAdd(i, ID uint, header BlockHeader, errMsg string) {
	cMsg.mux.Lock()
	if cMsg.m[i] == nil {
		cMsg.m[i] = make(map[uint]BlockHeader)
	}
	if _, ok := cMsg.m[i][ID]; ok {
		errFatal(nil, errMsg)
	}
	cMsg.m[i][ID] = header
	cMsg.mux.Unlock()
}

func (cMsg *ConsensusMsgs) getBlockHeader(i, ID uint) BlockHeader {
	cMsg.mux.Lock()
	defer cMsg.mux.Unlock()
	return cMsg.m[i][ID]
}

func (cMsg *ConsensusMsgs) getBlockHeaders(i uint) []BlockHeader {
	cMsg.mux.Lock()
	defer cMsg.mux.Unlock()

	blockHeaders := make([]BlockHeader, len(cMsg.m[i]))
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
	neighbors     []uint
	self          NodeAllInfo
	allInfo       AllInfo
	idaMsgs       IdaMsgs
	blocks        Blocks
	consensusMsgs ConsensusMsgs
	channels      Channels
	i             CurrentIteration
}

// generic msg. typ indicates which struct to decode msg to.
type Msg struct {
	Typ    string
	Msg    interface{}
	FromID uint // TODO add signature aswell
}
