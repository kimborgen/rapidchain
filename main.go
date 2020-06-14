package main

import (
	"encoding/gob"
	"flag"
)

func main() {
	// Program starts here. This function will spawn the x RC instances.

	// defaults in defaults.go
	functionPtr := flag.String("function", default_function, "coordinator or node")
	vCPUs := flag.Uint("vpcus", default_vCPUs, "amount of VCPUs available")
	instancesPerVCPUPtr := flag.Uint("instances", default_instances, "Instances per VCPU")
	nPtr := flag.Uint("n", default_n, "Total amount of nodes")
	mPtr := flag.Uint("m", default_m, "Number of committees")
	totalFPtr := flag.Uint("totalF", default_totalF, "Total adversary tolerance in the form of the divisor (1/x)")
	committeeFPtr := flag.Uint("committeeF", default_committeeF, "Committee adversary tolerance in the form of the divisor (1/x)")
	dPtr := flag.Uint("d", default_d, "d neighbours")
	BPtr := flag.Uint("B", default_B, "block size in bytes")

	flag.Parse()

	var flagArgs FlagArgs

	flagArgs.function = *functionPtr
	flagArgs.vCPUs = *vCPUs
	flagArgs.instances = *instancesPerVCPUPtr
	flagArgs.n = *nPtr
	flagArgs.m = *mPtr
	flagArgs.totalF = *totalFPtr
	flagArgs.committeeF = *committeeFPtr
	flagArgs.d = *dPtr
	flagArgs.B = *BPtr

	// register structs with gob
	gob.Register(IDAGossipMsg{})
	gob.Register([32]uint8{})

	if *functionPtr == "coordinator" {
		launchCoordinator(*nPtr, *mPtr, *totalFPtr, *committeeFPtr)
	} else {
		launchNodes(&flagArgs)
	}

}

func launchNodes(flagArgs *FlagArgs) {
	for i := uint(0); i < flagArgs.vCPUs*flagArgs.instances; i++ {
		go launchNode(flagArgs)
	}

	for {

	}
}
