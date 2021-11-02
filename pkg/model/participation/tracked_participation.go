package participation

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/marshalutil"
	iotago "github.com/iotaledger/iota.go/v2"
)

// TrackedParticipation holds the information the node tracked for the participation.
type TrackedParticipation struct {
	EventID    EventID
	OutputID   *iotago.UTXOInputID
	MessageID  hornet.MessageID
	Amount     uint64
	StartIndex milestone.Index
	EndIndex   milestone.Index
}

// ParseEventID helps to parse an EventID using marshalutil.
func ParseEventID(ms *marshalutil.MarshalUtil) (EventID, error) {
	bytes, err := ms.ReadBytes(EventIDLength)
	if err != nil {
		return NullEventID, err
	}
	o := EventID{}
	copy(o[:], bytes)
	return o, nil
}

func trackedParticipation(key []byte, value []byte) (*TrackedParticipation, error) {

	if len(key) != 67 {
		return nil, ErrInvalidPreviouslyTrackedParticipation
	}

	if len(value) != 48 {
		return nil, ErrInvalidPreviouslyTrackedParticipation
	}

	mKey := marshalutil.New(key)

	// Skip prefix
	if _, err := mKey.ReadByte(); err != nil {
		return nil, err
	}

	// Read EventID
	eventID, err := ParseEventID(mKey)
	if err != nil {
		return nil, err
	}

	// Read OutputID
	outputID, err := utxo.ParseOutputID(mKey)
	if err != nil {
		return nil, err
	}

	mValue := marshalutil.New(value)

	messageID, err := utxo.ParseMessageID(mValue)
	if err != nil {
		return nil, err
	}

	amount, err := mValue.ReadUint64()
	if err != nil {
		return nil, err
	}

	start, err := mValue.ReadUint32()
	if err != nil {
		return nil, err
	}

	end, err := mValue.ReadUint32()
	if err != nil {
		return nil, err
	}

	return &TrackedParticipation{
		EventID:    eventID,
		OutputID:   outputID,
		MessageID:  messageID,
		Amount:     amount,
		StartIndex: milestone.Index(start),
		EndIndex:   milestone.Index(end),
	}, nil
}

func (t *TrackedParticipation) valueBytes() []byte {
	m := marshalutil.New(48)
	m.WriteBytes(t.MessageID) // 32 bytes
	m.WriteUint64(t.Amount)
	m.WriteUint32(uint32(t.StartIndex)) // 4 bytes
	m.WriteUint32(uint32(t.EndIndex))   // 4 bytes
	return m.Bytes()
}
