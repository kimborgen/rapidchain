package main

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

// generic msg. typ indicates which struct to decode msg to.
type Msg struct {
	Typ    string
	Msg    interface{}
	FromID uint // TODO add signature aswell
}
