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

// LexicalOrderedOutputs are BlockIDs ordered in lexical order.
type LexicalOrderedMessageIDs BlockIDs

func (l LexicalOrderedMessageIDs) Len() int {
	return len(l)
}

func (l LexicalOrderedMessageIDs) Less(i, j int) bool {
	return bytes.Compare(l[i], l[j]) < 0
}

func (l LexicalOrderedMessageIDs) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

// ToHex converts the BlockID to its hex representation.
func (m BlockID) ToHex() string {
	return iotago.EncodeHex(m)
}

// ToArray converts the BlockID to an array.
func (m BlockID) ToArray() iotago.BlockID {
	var messageID iotago.BlockID
	copy(messageID[:], m)
	return messageID
}

// ToMapKey converts the BlockID to a string that can be used as a map key.
func (m BlockID) ToMapKey() string {
	return string(m)
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (m BlockID) MarshalBinary() ([]byte, error) {
	return m, nil
}

// IsNullMessageID returns the true if it is the genesis message ID.
func (m BlockID) IsNullMessageID() bool {
	return bytes.Equal(m[:], NullMessageID()[:])
}

// NullMessageID returns the ID of the genesis message.
func NullMessageID() BlockID {
	nullMessageID := make(BlockID, 32)
	return nullMessageID
}

// MessageIDFromHex creates a BlockID from a hex string representation.
func MessageIDFromHex(hexString string) (BlockID, error) {

	b, err := iotago.DecodeHex(hexString)
	if err != nil {
		return nil, err
	}

	if len(b) != iotago.BlockIDLength {
		return nil, fmt.Errorf("unknown messageID length (%d)", len(b))
	}

	return BlockID(b), nil
}

// MessageIDFromMapKey creates a BlockID from a map key representation.
func MessageIDFromMapKey(mapKey string) BlockID {
	if len(mapKey) != iotago.BlockIDLength {
		panic(fmt.Sprintf("unknown messageID length (%d)", len(mapKey)))
	}

	return BlockID(mapKey)
}

// MessageIDFromSlice creates a BlockID from a byte slice.
func MessageIDFromSlice(b []byte) BlockID {

	if len(b) != iotago.BlockIDLength {
		panic(fmt.Sprintf("unknown messageID length (%d)", len(b)))
	}

	return BlockID(b)
}

// MessageIDFromArray creates a BlockID from a byte array.
func MessageIDFromArray(b iotago.BlockID) BlockID {
	return append(BlockID{}, b[:]...)
}

// ToHex converts the BlockIDs to their hex string representation.
func (m BlockIDs) ToHex() []string {
	results := make([]string, len(m))
	for i, msgID := range m {
		results[i] = msgID.ToHex()
	}
	return results
}

// ToSliceOfSlices converts the BlockIDs to a slice of byte slices.
func (m BlockIDs) ToSliceOfSlices() [][]byte {
	results := make([][]byte, len(m))
	for i, msgID := range m {
		results[i] = msgID
	}
	return results
}

// ToSliceOfArrays converts the BlockIDs to a slice of byte arrays.
func (m BlockIDs) ToSliceOfArrays() iotago.BlockIDs {
	results := make(iotago.BlockIDs, len(m))
	for i, msgID := range m {
		results[i] = msgID.ToArray()
	}
	return results
}

// RemoveDupsAndSortByLexicalOrder returns a new slice of BlockIDs sorted by lexical order and without duplicates.
func (m BlockIDs) RemoveDupsAndSortByLexicalOrder() BlockIDs {
	// sort the messages lexicographically
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

// MessageIDsFromHex creates a slice of BlockIDs from a slice of hex string representations.
func MessageIDsFromHex(hexStrings []string) (BlockIDs, error) {
	results := make(BlockIDs, len(hexStrings))

	for i, hexString := range hexStrings {
		msgID, err := MessageIDFromHex(hexString)
		if err != nil {
			return nil, err
		}
		results[i] = msgID
	}

	return results, nil
}

// MessageIDsFromSliceOfArrays creates a slice of BlockIDs from a slice of arrays.
func MessageIDsFromSliceOfArrays(b iotago.BlockIDs) BlockIDs {
	results := make(BlockIDs, len(b))
	for i, msgID := range b {
		// as msgID is reused between iterations, it must be copied
		results[i] = MessageIDFromArray(msgID)
	}
	return results
}

func MessageIDsFromSliceOfSlices(s [][]byte) BlockIDs {
	results := make(BlockIDs, len(s))
	for i, msgID := range s {
		// as msgID is reused between iterations, it must be copied
		results[i] = MessageIDFromSlice(msgID)
	}
	return results
}
