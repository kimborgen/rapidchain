package main

import "math"

// Default variables to be used if flag is not set

const default_function string = "node"
const default_vCPUs uint = 1
const default_instances uint = 40
const default_n uint = 40
const default_m uint = 10 // number of comimttees
const default_totalF uint = 3
const default_committeeF uint = 2
const default_d uint = 3
const default_nUsers uint = default_n * 5
const default_totalCoins uint = default_nUsers * 10
const default_tps uint = default_m

var default_B uint = uint(math.Pow(2, 21)) // 2 mill

// TODO add these to flags and so on
const default_kappa = 128 //128
const default_phi = 0.63
const default_parity = 80 // 80?
const default_delta = 600 //ms

type FlagArgs struct {
	function   string
	vCPUs      uint
	instances  uint
	n          uint
	m          uint
	totalF     uint
	committeeF uint
	d          uint
	B          uint
	nUsers     uint
	totalCoins uint
	tps        uint
}
