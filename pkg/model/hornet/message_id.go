package hornet

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

// BlockID is the ID of a Block.
type BlockID []byte

// BlockIDs is a slice of BlockID.
type BlockIDs []BlockID

// LexicalOrderedBlockIDs are BlockIDs ordered in lexical order.
type LexicalOrderedBlockIDs BlockIDs

func (l LexicalOrderedBlockIDs) Len() int {
	return len(l)
}

func (l LexicalOrderedBlockIDs) Less(i, j int) bool {
	return bytes.Compare(l[i], l[j]) < 0
}

func (l LexicalOrderedBlockIDs) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

// ToHex converts the BlockID to its hex representation.
func (m BlockID) ToHex() string {
	return iotago.EncodeHex(m)
}

// ToArray converts the BlockID to an array.
func (m BlockID) ToArray() iotago.BlockID {
	var blockID iotago.BlockID
	copy(blockID[:], m)
	return blockID
}

// ToMapKey converts the BlockID to a string that can be used as a map key.
func (m BlockID) ToMapKey() string {
	return string(m)
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (m BlockID) MarshalBinary() ([]byte, error) {
	return m, nil
}

// IsNullBlockID returns the true if it is the genesis block ID.
func (m BlockID) IsNullBlockID() bool {
	return bytes.Equal(m[:], NullBlockID()[:])
}

// NullBlockID returns the ID of the genesis block.
func NullBlockID() BlockID {
	nullBlockID := make(BlockID, iotago.BlockIDLength)
	return nullBlockID
}

// BlockIDFromHex creates a BlockID from a hex string representation.
func BlockIDFromHex(hexString string) (BlockID, error) {

	b, err := iotago.DecodeHex(hexString)
	if err != nil {
		return nil, err
	}

	if len(b) != iotago.BlockIDLength {
		return nil, fmt.Errorf("unknown blockID length (%d)", len(b))
	}

	return BlockID(b), nil
}

// BlockIDFromMapKey creates a BlockID from a map key representation.
func BlockIDFromMapKey(mapKey string) BlockID {
	if len(mapKey) != iotago.BlockIDLength {
		panic(fmt.Sprintf("unknown blockID length (%d)", len(mapKey)))
	}

	return BlockID(mapKey)
}

// BlockIDFromSlice creates a BlockID from a byte slice.
func BlockIDFromSlice(b []byte) BlockID {

	if len(b) != iotago.BlockIDLength {
		panic(fmt.Sprintf("unknown blockID length (%d)", len(b)))
	}

	return BlockID(b)
}

// BlockIDFromArray creates a BlockID from a byte array.
func BlockIDFromArray(b iotago.BlockID) BlockID {
	return append(BlockID{}, b[:]...)
}

// ToHex converts the BlockIDs to their hex string representation.
func (m BlockIDs) ToHex() []string {
	results := make([]string, len(m))
	for i, blockID := range m {
		results[i] = blockID.ToHex()
	}
	return results
}

// ToSliceOfSlices converts the BlockIDs to a slice of byte slices.
func (m BlockIDs) ToSliceOfSlices() [][]byte {
	results := make([][]byte, len(m))
	for i, blockID := range m {
		results[i] = blockID
	}
	return results
}

// ToSliceOfArrays converts the BlockIDs to a slice of byte arrays.
func (m BlockIDs) ToSliceOfArrays() iotago.BlockIDs {
	results := make(iotago.BlockIDs, len(m))
	for i, blockID := range m {
		results[i] = blockID.ToArray()
	}
	return results
}

// RemoveDupsAndSortByLexicalOrder returns a new slice of BlockIDs sorted by lexical order and without duplicates.
func (m BlockIDs) RemoveDupsAndSortByLexicalOrder() BlockIDs {
	// sort the blocks lexicographically
	sorted := make(serializer.LexicalOrderedByteSlices, len(m))
	for i, id := range m {
		sorted[i] = id
	}
	sort.Sort(sorted)

	var result BlockIDs
	var prev BlockID
	for i, id := range sorted {
		// only add to the result, if it its different from its predecessor
		if i == 0 || !bytes.Equal(prev, id) {
			result = append(result, id)
		}
		prev = id
	}
	return result
}

// BlockIDsFromHex creates a slice of BlockIDs from a slice of hex string representations.
func BlockIDsFromHex(hexStrings []string) (BlockIDs, error) {
	results := make(BlockIDs, len(hexStrings))

	for i, hexString := range hexStrings {
		blockID, err := BlockIDFromHex(hexString)
		if err != nil {
			return nil, err
		}
		results[i] = blockID
	}

	return results, nil
}

// BlockIDsFromSliceOfArrays creates a slice of BlockIDs from a slice of arrays.
func BlockIDsFromSliceOfArrays(b iotago.BlockIDs) BlockIDs {
	results := make(BlockIDs, len(b))
	for i, blockID := range b {
		// as blockID is reused between iterations, it must be copied
		results[i] = BlockIDFromArray(blockID)
	}
	return results
}

func BlockIDsFromSliceOfSlices(s [][]byte) BlockIDs {
	results := make(BlockIDs, len(s))
	for i, blockID := range s {
		// as blockID is reused between iterations, it must be copied
		results[i] = BlockIDFromSlice(blockID)
	}
	return results
}
