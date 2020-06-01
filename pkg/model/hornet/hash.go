package hornet

import (
	"fmt"

	"github.com/iotaledger/iota.go/trinary"
)

var (
	// NullHashBytes is the binary hash of the genesis transaction.
	NullHashBytes = make(Hash, 49)
)

// Hash is the binary representation of a trinary Hash.
type Hash []byte

// Trytes converts the binary Hash to its trinary representation.
func (h Hash) Trytes() trinary.Trytes {
	if len(h) == 49 {
		return trinary.MustBytesToTrytes(h, 81)
	}
	if len(h) == 17 {
		return trinary.MustBytesToTrytes(h, 27)
	}
	panic(fmt.Sprintf("Unknown hash length (%d)", len(h)))
}

// Hashes is a slice of Hash.
type Hashes []Hash

// Trytes converts the binary Hashes to their trinary representation.
func (h Hashes) Trytes() []trinary.Trytes {
	var results []trinary.Trytes
	for _, hash := range h {
		results = append(results, hash.Trytes())
	}
	return results
}
