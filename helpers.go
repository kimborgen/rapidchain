package main

import (
	"bytes"
	"encoding/gob"
	"log"
	"math/big"
	"math/rand"
	"sort"
)

func ifErr(e interface{}, msg string) bool {
	if e != nil {
		log.Printf("[Error] msg(%s) error (%s)", msg, e)
		return true
	}
	return false
}

func ifErrFatal(e interface{}, msg string) bool {
	if e != nil {
		log.Fatalf("[Fatal] msg(%s) error(%s)", msg, e)
		panic(e)
		return true
	}
	return false
}

func errr(e interface{}, msg string) {
	log.Printf("[Error] msg(%s) error(%s)", msg, e)
}

func errFatal(e interface{}, msg string) {
	log.Fatalf("[Fatal] msg(%s) error: (%s)", msg, e)
	panic(e)
}

func notOkErr(ok bool, msg string) {
	if !ok {
		errFatal(ok, msg)
	}
}

func randIndexesWithoutReplacement(arrayLength, sampleSize int) []int {
	// Sample a list of sampleSize random indexes in an array of length arrayLength
	// record indexes here to prevent duplicates
	indexes := make(map[int]bool)

	// create n random indexes
	for i := 0; i < sampleSize; i++ {
		var r int
		for {
			r = int(rand.Int63n(int64(arrayLength)))
			if indexes[r] {
				continue
			}
			break
		}

		indexes[r] = true
	}

	keys := make([]int, 0, len(indexes))
	for k := range indexes {
		keys = append(keys, k)
	}

	return keys
}

func getBytes(thing interface{}) []byte {
	// return a byte array of anyting
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(thing)
	ifErrFatal(err, "getBytes enc")
	return buf.Bytes()
}

func byteSliceAppend(b ...[]byte) []byte {
	tmp := []byte{}
	for i := 0; i < len(b); i++ {
		tmp = append(tmp, b[i]...)
	}
	return tmp
}

func toByte32(b []byte) [32]byte {
	var a [32]byte
	for i := range a {
		a[i] = b[i]
	}
	return a
}

func toBigInt(b [32]byte) *big.Int {
	return new(big.Int).SetBytes(b[:])
}

func sortBigIntArr(a *[]*big.Int) {
	//sort big int arr inplace
	b := *a
	sort.Slice(b, func(i, j int) bool { return b[i].Cmp(b[j]) < 0 })
	a = &b
}
