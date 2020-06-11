package main

import (
	"flag"
	"fmt"
)

func main() {
	// Program starts here. This function will spawn the x RC instances.

	// defaults in defaults.go
	funcPtr := flag.String("func", default_func, "coordinator or node")
	vCPUs := flag.Uint("vpcus", default_vCPUs, "amount of VCPUs available")
	instancesPerVCPUPtr := flag.Uint("instances", default_instances, "Instances per VCPU")
	nPtr := flag.Uint("n", default_n, "Total amount of nodes")
	mPtr := flag.Uint("m", default_m, "Number of committees")
	totalFPtr := flag.Uint("totalF", default_totalF, "Total adversary tolerance in the form of the divisor (1/x)")
	committeeFPtr := flag.Uint("committeeF", default_committeeF, "Committee adversary tolerance in the form of the divisor (1/x)")

	fmt.Printf("%d", maxId)

	flag.Parse()

	if *funcPtr == "coordinator" {
		launchCoordinator(*nPtr, *mPtr, *totalFPtr, *committeeFPtr)
	} else {
		launchNodes(*vCPUs, *instancesPerVCPUPtr)
	}

}

func launchNodes(vCPUs, instancesPerVCPU uint) {
	for i := uint(0); i < vCPUs*instancesPerVCPU; i++ {
		go launchNode()
	}

	for {

	}
}
