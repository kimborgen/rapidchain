package main

import (
	"encoding/gob"
	"net"
)

func dial(addr string) net.Conn {
	conn, err := net.Dial("tcp", addr)
	ifErrFatal(err, "dialing addr "+addr)
	return conn
}

func sendMsg(conn net.Conn, msg interface{}) {
	enc := gob.NewEncoder(conn)
	err := enc.Encode(msg)
	ifErrFatal(err, "encoding and sending")
}

func dialAndSend(addr string, msg interface{}) {
	conn := dial(addr)
	sendMsg(conn, msg)
	conn.Close()
}

func reciveMsg(conn net.Conn, obj interface{}) {
	dec := gob.NewDecoder(conn)
	err := dec.Decode(obj)
	ifErrFatal(err, "decoding")
}

func sendMsgToCommittee(msg Msg, committee *Committee) {
	for _, v := range (*committee).Members {
		go dialAndSend(v.IP, msg)
	}
}

func sendMsgToCommitteeAndSelf(msg Msg, nodeCtx *NodeCtx) {
	go dialAndSend(nodeCtx.self.IP, msg)
	for _, v := range nodeCtx.committee.Members {
		go dialAndSend(v.IP, msg)
	}
}
