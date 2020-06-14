package main

import (
	"log"
	"math/rand"
)

func ifErr(e interface{}, msg string) bool {
	if e != nil {
		log.Printf("[Error] %s with error %s", msg, e)
		return true
	}
	return false
}

func ifErrFatal(e interface{}, msg string) bool {
	if e != nil {
		log.Fatalf("[Fatal] msg(%s) with error(%s)", msg, e)
		panic(e)
		return true
	}
	return false
}

func errr(e interface{}, msg string) {
	log.Printf("[Error] %s with error %s", msg, e)
}

func errFatal(e interface{}, msg string) {
	log.Fatalf("[Fatal] %s with error: %s", msg, e)
	panic(e)
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
