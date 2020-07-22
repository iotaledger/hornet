package utils

import (
	"math/rand"
	"time"

	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	seededRand    = rand.New(rand.NewSource(time.Now().UnixNano()))
	randLock      = &syncutils.Mutex{}
	charsetTrytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZ9"
)

// RandomInsecure returns a random int in the range of min to max.
// the result is not cryptographically secure.
// RandomInsecure is inclusive max value.
func RandomInsecure(min int, max int) int {
	// Rand needs to be locked: https://github.com/golang/go/issues/3611
	randLock.Lock()
	defer randLock.Unlock()
	return seededRand.Intn(max+1-min) + min
}

// RandomTrytesInsecure returns random Trytes with the given length.
// the result is not cryptographically secure.
// DO NOT USE this function to generate a seed.
func RandomTrytesInsecure(length int) trinary.Trytes {
	// Rand needs to be locked: https://github.com/golang/go/issues/3611
	randLock.Lock()
	defer randLock.Unlock()

	trytes := make([]byte, length)
	for i := range trytes {
		trytes[i] = charsetTrytes[seededRand.Intn(len(charsetTrytes))]
	}
	return trinary.Trytes(trytes)
}
