package hornet

import (
	"encoding/hex"
	"fmt"

	iotago "github.com/iotaledger/iota.go"
)

// GetNullMessageID returns the ID of the genesis message.
func GetNullMessageID() *MessageID {
	var nullMessageID MessageID
	return &nullMessageID
}

// MessageID is the array representation of a MessageID.
type MessageID iotago.MessageHash

// Hex converts the MessageID to its hex string representation.
func (h *MessageID) Hex() string {
	return hex.EncodeToString(h[:])
}

// Slice converts the MessageID array to a slice.
func (h *MessageID) Slice() []byte {
	return h[:]
}

// MapKey converts the MessageID to a string that can be used as a map key.
func (h *MessageID) MapKey() string {
	return string(h[:])
}

// MessageIDFromMapKey creates a MessageID from a hex string representation.
func MessageIDFromHex(hexString string) (*MessageID, error) {

	b, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}

	if len(b) != iotago.MessageHashLength {
		return nil, fmt.Errorf("unknown hash length (%d)", len(b))
	}

	var messageID MessageID
	copy(messageID[:], b)

	return &messageID, nil
}

// MessageIDFromMapKey creates a MessageID from a map key representation.
func MessageIDFromMapKey(mapKey string) *MessageID {

	if len(mapKey) != iotago.MessageHashLength {
		panic(fmt.Sprintf("unknown hash length (%d)", len(mapKey)))
	}

	var messageID MessageID
	copy(messageID[:], []byte(mapKey))

	return &messageID
}

// MessageIDFromBytes creates a MessageID from a byte slice.
func MessageIDFromBytes(b []byte) *MessageID {

	if len(b) != iotago.MessageHashLength {
		panic(fmt.Sprintf("unknown hash length (%d)", len(b)))
	}

	var messageID MessageID
	copy(messageID[:], b)

	return &messageID
}

// MessageIDs is a slice of MessageID.
type MessageIDs []*MessageID

// Hex converts the MessageIDs to their hex string representation.
func (h MessageIDs) Hex() []string {
	var results []string
	for _, hash := range h {
		results = append(results, hash.Hex())
	}
	return results
}
