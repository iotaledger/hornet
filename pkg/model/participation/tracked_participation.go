package participation

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/marshalutil"
	iotago "github.com/iotaledger/iota.go/v3"
)

// TrackedParticipation holds the information the node tracked for the participation.
type TrackedParticipation struct {
	// EventID is the ID of the event the participation is made for.
	EventID EventID
	// OutputID is the ID of the output the participation was made.
	OutputID *iotago.OutputID
	// MessageID is the ID of the message that included the transaction that created the output the participation was made.
	MessageID hornet.MessageID
	// Amount is the amount of tokens that were included in the output the participation was made.
	Amount uint64
	// StartIndex is the milestone index the participation started.
	StartIndex milestone.Index
	// EndIndex is the milestone index the participation ended. 0 if the participation is still active.
	EndIndex milestone.Index
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

func TrackedParticipationFromBytes(key []byte, value []byte) (*TrackedParticipation, error) {

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

func (t *TrackedParticipation) ValueBytes() []byte {
	m := marshalutil.New(48)
	m.WriteBytes(t.MessageID)           // 32 bytes
	m.WriteUint64(t.Amount)             // 8 bytes
	m.WriteUint32(uint32(t.StartIndex)) // 4 bytes
	m.WriteUint32(uint32(t.EndIndex))   // 4 bytes
	return m.Bytes()
}
