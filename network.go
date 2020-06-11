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

func reciveMsg(conn net.Conn, obj interface{}) {
	dec := gob.NewDecoder(conn)
	err := dec.Decode(obj)
	ifErrFatal(err, "decoding")
}
