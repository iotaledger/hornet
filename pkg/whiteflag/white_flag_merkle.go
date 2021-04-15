package whiteflag

import (
	"crypto"
	"encoding"
	"math/bits"
)

// Domain separation prefixes
const (
	LeafHashPrefix = 0
	NodeHashPrefix = 1
)

// Hasher implements the hashing algorithm described in the IOTA protocol RFC-12.
type Hasher struct {
	hash crypto.Hash
}

// NewHasher creates a new Hasher using the provided hash function.
func NewHasher(h crypto.Hash) *Hasher {
	return &Hasher{hash: h}
}

// Size returns the length, in bytes, of a digest resulting from the given hash function.
func (t *Hasher) Size() int {
	return t.hash.Size()
}

// EmptyRoot returns a special case for an empty tree.
// This is equivalent to Hash(nil).
func (t *Hasher) EmptyRoot() []byte {
	return t.hash.New().Sum(nil)
}

// Hash computes the Merkle tree hash of the provided data encodings.
func (t *Hasher) Hash(data []encoding.BinaryMarshaler) ([]byte, error) {
	if len(data) == 0 {
		return t.EmptyRoot(), nil
	}
	if len(data) == 1 {
		return t.hashLeaf(data[0])
	}

	k := largestPowerOfTwo(len(data))
	l, err := t.Hash(data[:k])
	if err != nil {
		return nil, err
	}
	r, err := t.Hash(data[k:])
	if err != nil {
		return nil, err
	}
	return t.hashNode(l, r), nil
}

// hashLeaf returns the Merkle tree leaf hash of data.
func (t *Hasher) hashLeaf(data encoding.BinaryMarshaler) ([]byte, error) {
	b, err := data.MarshalBinary()
	if err != nil {
		return nil, err
	}
	h := t.hash.New()
	h.Write([]byte{LeafHashPrefix})
	h.Write(b)
	return h.Sum(nil), nil
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
