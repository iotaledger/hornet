package hornet

import (
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

// GetNullMessageID returns the ID of the genesis message.
func GetNullMessageID() MessageID {
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
	return MessageID(b[:])
}

// ToHex converts the MessageIDs to their hex string representation.
func (m MessageIDs) ToHex() []string {
	var results []string
	for _, msgID := range m {
		results = append(results, msgID.ToHex())
	}
	return results
}

// ToSliceOfSlices converts the MessageIDs to a slice of byte slices.
func (m MessageIDs) ToSliceOfSlices() [][]byte {
	var results [][]byte
	for _, msgID := range m {
		// this is necessary, otherwise we create a pointer to the loop variable
		tmp := msgID
		results = append(results, tmp)
	}
	return results
}

// ToSliceOfArrays converts the MessageIDs to a slice of byte arrays.
func (m MessageIDs) ToSliceOfArrays() iotago.MessageIDs {
	var results iotago.MessageIDs
	for _, msgID := range m {
		results = append(results, msgID.ToArray())
	}
	return results
}

// RemoveDupsAndSortByLexicalOrder returns a new slice of MessageIDs sorted by lexical order and without duplicates.
func (m MessageIDs) RemoveDupsAndSortByLexicalOrder() MessageIDs {

	seen := make(map[string]struct{})
	orderedArray := make(iotago.LexicalOrderedByteSlices, len(m))

	uniqueElements := 0
	for _, v := range m {
		// this is necessary, otherwise we create a pointer to the loop variable
		tmp := v
		k := string(v)
		if _, has := seen[k]; has {
			continue
		}
		seen[k] = struct{}{}
		orderedArray[uniqueElements] = tmp
		uniqueElements++
	}
	orderedArray = orderedArray[:uniqueElements]
	sort.Sort(orderedArray)

	result := make(MessageIDs, uniqueElements)
	for i, v := range orderedArray {
		// this is necessary, otherwise we create a pointer to the loop variable
		tmp := v
		result[i] = tmp
	}

	return result
}

// MessageIDsFromSliceOfArrays creates slice of MessageIDs from a slice of arrays.
func MessageIDsFromSliceOfArrays(b iotago.MessageIDs) MessageIDs {
	result := make(MessageIDs, len(b))
	for i, msgID := range b {
		// this is necessary, otherwise we create a pointer to the loop variable
		tmp := msgID
		result[i] = tmp[:]
	}
	return result
}
