package hornet

import (
	"encoding/hex"
	"fmt"
)

// MilestoneMessageID is the binary representation of a MilestoneMessageID.
type Hash []byte

// Hex converts the binary MilestoneMessageID to its hex string representation.
func (h Hash) Hex() string {
	if len(h) == 32 {
		return hex.EncodeToString(h)
	}

	panic(fmt.Sprintf("Unknown hash length (%d)", len(h)))
}

// ID converts the binary MilestoneMessageID to an array representation.
func (h Hash) ID() (id [32]byte) {
	if len(h) == 32 {
		copy(id[:], h[:32])
	}

	panic(fmt.Sprintf("Unknown hash length (%d)", len(h)))
}

// Hashes is a slice of MilestoneMessageID.
type Hashes []Hash

// Hex converts the binary Hashes to their hex string representation.
func (h Hashes) Hex() []string {
	var results []string
	for _, hash := range h {
		results = append(results, hash.Hex())
	}
	return results
}
