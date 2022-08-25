package tipselect

import (
	"math/rand"
	"time"

	"github.com/iotaledger/hive.go/core/syncutils"
)

var (
	//nolint:gosec // we don't care about weak random numbers here
	seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
	randLock   = &syncutils.Mutex{}
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
