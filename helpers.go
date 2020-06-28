package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"fmt"
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
	for i := range b {
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

func padByteToBeDivisible(b []byte, div uint) []byte {
	blen := uint(len(b))
	toAdd := uint(0)
	for {
		if (blen+toAdd)%div != 0 {
			toAdd++
		} else {
			break
		}
	}
	if toAdd > 255 {
		errFatal(nil, "Had to add more bytes than can be represented by 1byte")
	}
	return pad(b, uint8(toAdd))
}

func pad(b []byte, bytesToAdd uint8) []byte {
	// pad bytes using PKCS#7 https://en.wikipedia.org/wiki/Padding_(cryptography)#PKCS7
	// (each padded byte indicate how many bytes are padded)
	newB := make([]byte, uint(len(b))+uint(bytesToAdd))
	padB := byte(bytesToAdd)
	for i, byt := range b {
		newB[i] = byt
	}
	for i := uint(len(b)); i < uint(len(b))+uint(bytesToAdd); i++ {
		newB[i] = padB
	}
	return newB
}

func isPadded(b []byte) bool {
	// figures out if given byte array is padded using PKCS#7
	padded := uint8(b[len(b)-1])
	pB := uint(padded)
	uB := uint(len(b))
	if uB <= pB {
		// array is shorter or same as indicated padded bytes therefor it is not padded
		return false
	}
	isP := true
	for i := uB - pB; i < uB; i++ {
		if uint8(b[i]) != padded {
			isP = false
			break
		}
	}
	return isP
}

func unPad(b []byte) []byte {
	notOkErr(isPadded(b), "byte array was not padded")
	padded := uint8(b[len(b)-1])
	return b[:uint(len(b))-uint(padded)]
}

func uintToByte(u uint) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(u))
	return b
}

func bytesToString(b []byte) string {
	return hex.EncodeToString(b)
}

func bytes32ToString(b [32]byte) string {
	return hex.EncodeToString(b[:])
}

// sort a list of [32]byte
func sortListOf32Byte(lst [][32]byte) [][32]byte {
	sort.Slice(lst, func(i, j int) bool {
		return toBigInt((lst)[i]).Cmp(toBigInt((lst)[j])) < 0
	})
	return lst
}

func sortListOfByte32SortHelper(lst []byte32sortHelper) []byte32sortHelper {
	sort.Slice(lst, func(i, j int) bool {
		return toBigInt(lst[i].toSort).Cmp(toBigInt(lst[j].toSort)) < 0
	})
	return lst
}

func byte32Operations(a [32]byte, operator string, b [32]byte) bool {
	ai := toBigInt(a)
	bi := toBigInt(b)
	switch operator {
	case "<":
		return ai.Cmp(bi) < 0
	case ">":
		return ai.Cmp(bi) > 0
	case "==":
		return ai.Cmp(bi) == 0
	case ">=":
		return ai.Cmp(bi) >= 0
	case "<=":
		return ai.Cmp(bi) <= 0
	default:
		errFatal(nil, fmt.Sprintf("unknown [32]byte operation %s", operator))
		return false
	}
}
