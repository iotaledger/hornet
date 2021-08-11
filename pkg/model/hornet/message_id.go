package hornet

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"

	iotago "github.com/iotaledger/iota.go/v2"
)

// MessageID is the ID of a Message.
type MessageID []byte

// MessageIDs is a slice of MessageID.
type MessageIDs []MessageID

// ToHex converts the MessageID to its hex representation.
func (m MessageID) ToHex() string {
	return hex.EncodeToString(m)
}

// ToArray converts the MessageID to an array.
func (m MessageID) ToArray() iotago.MessageID {
	var messageID iotago.MessageID
	copy(messageID[:], m)
	return messageID
}

// ToMapKey converts the MessageID to a string that can be used as a map key.
func (m MessageID) ToMapKey() string {
	return string(m)
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (m MessageID) MarshalBinary() ([]byte, error) {
	return m, nil
}

// NullMessageID returns the ID of the genesis message.
func NullMessageID() MessageID {
	nullMessageID := make(MessageID, 32)
	return nullMessageID
}

// MessageIDFromHex creates a MessageID from a hex string representation.
func MessageIDFromHex(hexString string) (MessageID, error) {

	b, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}

	if len(b) != iotago.MessageIDLength {
		return nil, fmt.Errorf("unknown messageID length (%d)", len(b))
	}

	return MessageID(b), nil
}

// MessageIDFromMapKey creates a MessageID from a map key representation.
func MessageIDFromMapKey(mapKey string) MessageID {
	if len(mapKey) != iotago.MessageIDLength {
		panic(fmt.Sprintf("unknown messageID length (%d)", len(mapKey)))
	}

	return MessageID(mapKey)
}

// MessageIDFromSlice creates a MessageID from a byte slice.
func MessageIDFromSlice(b []byte) MessageID {

	if len(b) != iotago.MessageIDLength {
		panic(fmt.Sprintf("unknown messageID length (%d)", len(b)))
	}

	return MessageID(b)
}

// MessageIDFromArray creates a MessageID from a byte array.
func MessageIDFromArray(b iotago.MessageID) MessageID {
	return append(MessageID{}, b[:]...)
}

// ToHex converts the MessageIDs to their hex string representation.
func (m MessageIDs) ToHex() []string {
	results := make([]string, len(m))
	for i, msgID := range m {
		results[i] = msgID.ToHex()
	}
	return results
}

// ToSliceOfSlices converts the MessageIDs to a slice of byte slices.
func (m MessageIDs) ToSliceOfSlices() [][]byte {
	results := make([][]byte, len(m))
	for i, msgID := range m {
		results[i] = msgID
	}
	return results
}

// ToSliceOfArrays converts the MessageIDs to a slice of byte arrays.
func (m MessageIDs) ToSliceOfArrays() iotago.MessageIDs {
	results := make(iotago.MessageIDs, len(m))
	for i, msgID := range m {
		results[i] = msgID.ToArray()
	}
	return results
}

// RemoveDupsAndSortByLexicalOrder returns a new slice of MessageIDs sorted by lexical order and without duplicates.
func (m MessageIDs) RemoveDupsAndSortByLexicalOrder() MessageIDs {
	// sort the messages lexicographically
	sorted := make(iotago.LexicalOrderedByteSlices, len(m))
	for i, id := range m {
		sorted[i] = id
	}
	sort.Sort(sorted)

	var result MessageIDs
	var prev MessageID
	for i, id := range sorted {
		// only add to the result, if it its different from its predecessor
		if i == 0 || !bytes.Equal(prev, id) {
			result = append(result, id)
		}
		prev = id
	}
	return result
}

// MessageIDsFromHex creates a slice of MessageIDs from a slice of hex string representations.
func MessageIDsFromHex(hexStrings []string) (MessageIDs, error) {
	results := make(MessageIDs, len(hexStrings))

	for i, hexString := range hexStrings {
		msgID, err := MessageIDFromHex(hexString)
		if err != nil {
			return nil, err
		}
		results[i] = msgID
	}

	return results, nil
}

// MessageIDsFromSliceOfArrays creates a slice of MessageIDs from a slice of arrays.
func MessageIDsFromSliceOfArrays(b iotago.MessageIDs) MessageIDs {
	results := make(MessageIDs, len(b))
	for i, msgID := range b {
		// as msgID is reused between iterations, it must be copied
		results[i] = MessageIDFromArray(msgID)
	}
	return results
}
