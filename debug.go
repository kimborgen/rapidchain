package main

import (
	"fmt"
	"time"
)

func debug(nodeCtx *NodeCtx) {
	// this will only run if you are the debug node

	time.Sleep(2 * time.Second)
	fmt.Println("\n\n")
	b1 := hash([]byte("lol dette er string"))
	//b2 := hash([]byte("Hvorfor er jeg ikke ute å drikker nå"))
	closest := txFindClosestCommittee(nodeCtx, b1)

	fmt.Println(nodeCtx.committeeList)

	fmt.Println("\n", closest)

	fmt.Println("\n\n")
}
