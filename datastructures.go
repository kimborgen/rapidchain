package main

// only data structures that are common in multiple, disjoint files, should belong here

type Node_InitialMessageToCoordinator struct {
	ID uint
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

// generic msg. typ indicates which struct to decode msg to.
type Msg struct {
	typ string
	msg interface{}
}
