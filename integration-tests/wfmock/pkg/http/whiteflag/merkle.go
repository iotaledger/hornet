package whiteflag

import (
	"crypto"
	"math/bits"

	"github.com/iotaledger/iota.go/encoding/t5b1"
	"github.com/iotaledger/iota.go/trinary"
	_ "golang.org/x/crypto/blake2b" // BLAKE2b_512 is the default hashing algorithm
)

// DefaultHasher is a BLAKE2 based Merkle tree.
var DefaultHasher = NewHasher(crypto.BLAKE2b_512)

// Domain separation prefixes
const (
	LeafHashPrefix = 0
	NodeHashPrefix = 1
)

// Hasher implements the hashing algorithm described in the IOTA protocol RFC-12.
type Hasher struct {
	hash crypto.Hash
}

// NewHasher creates a new Hashers based on the passed in hash function.
func NewHasher(h crypto.Hash) *Hasher {
	return &Hasher{hash: h}
}

// EmptyRoot returns a special case for an empty tree.
func (t *Hasher) EmptyRoot() []byte {
	return t.hash.New().Sum(nil)
}

// Hash computes the Merkle tree hash of the provided ternary hashes.
func (t *Hasher) Hash(hashes []trinary.Hash) []byte {
	if len(hashes) == 0 {
		return t.EmptyRoot()
	}
	if len(hashes) == 1 {
		return t.hashLeaf(hashes[0])
	}

	k := largestPowerOfTwo(len(hashes))
	return t.hashNode(t.Hash(hashes[:k]), t.Hash(hashes[k:]))
}

// hashLeaf returns the Merkle tree leaf hash of the provided ternary hash.
func (t *Hasher) hashLeaf(hash trinary.Hash) []byte {
	h := t.hash.New()
	h.Write([]byte{LeafHashPrefix})
	h.Write(t5b1.EncodeTrytes(hash))
	return h.Sum(nil)
}

// hashNode returns the inner Merkle tree node hash of the two child nodes l and r.
func (t *Hasher) hashNode(l, r []byte) []byte {
	h := t.hash.New()
	h.Write([]byte{NodeHashPrefix})
	h.Write(l)
	h.Write(r)
	return h.Sum(nil)
}

// largestPowerOfTwo returns the largest power of two less than n.
func largestPowerOfTwo(x int) int {
	if x < 2 {
		panic("invalid value")
	}
	return 1 << (bits.Len(uint(x-1)) - 1)
}
