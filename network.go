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

func dialAndSendToCoordinator(identifier string, _msg interface{}) {
	msg := Msg{identifier, _msg, nil}
	dialAndSend(coord+":8080", msg)
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
	for _, v := range nodeCtx.committee.Members {
		go dialAndSend(v.IP, msg)
	}
	go dialAndSend(nodeCtx.self.IP, msg)
}
