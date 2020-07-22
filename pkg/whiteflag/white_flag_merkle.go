package whiteflag

import (
	"crypto"
	"math/bits"

	"github.com/gohornet/hornet/pkg/model/hornet"
)

// Domain separation prefixes
const (
	LeafHashPrefix = 0
	NodeHashPrefix = 1
)

// Hasher implements the RFC6962 tree hashing algorithm.
type Hasher struct {
	crypto.Hash
}

// NewHasher creates a new Hasher on the passed in hash function.
func NewHasher(h crypto.Hash) *Hasher {
	return &Hasher{Hash: h}
}

// EmptyRoot returns a special case for an empty tree.
func (t *Hasher) EmptyRoot() []byte {
	return t.New().Sum(nil)
}

// TreeHash computes the Merkle tree hash of the provided hashes.
func (t *Hasher) TreeHash(tailHashes []hornet.Hash) []byte {
	if len(tailHashes) == 0 {
		return t.EmptyRoot()
	}
	if len(tailHashes) == 1 {
		return t.HashLeaf(tailHashes[0])
	}

	k := largestPowerOfTwo(len(tailHashes))
	return t.HashNode(t.TreeHash(tailHashes[:k]), t.TreeHash(tailHashes[k:]))
}

// HashLeaf returns the Merkle tree leaf hash of the input hash.
func (t *Hasher) HashLeaf(hash hornet.Hash) []byte {
	h := t.New()
	h.Write([]byte{LeafHashPrefix})
	h.Write(hash)
	return h.Sum(nil)
}

// HashNode returns the inner Merkle tree node hash of the two child nodes l and r.
func (t *Hasher) HashNode(l, r []byte) []byte {
	h := t.New()
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
