package participation_test

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer"
	"github.com/iotaledger/hive.go/testutil"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandString(strLen int) string {
	b := make([]byte, strLen)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func RandEventID() participation.EventID {
	eventID := participation.EventID{}
	copy(eventID[:], testutil.RandBytes(participation.EventIDLength))
	return eventID
}

func RandValidEventWithBallot() (*participation.Event, []byte) {
	return RandEventWithBallot(participation.EventNameMaxLength, participation.EventAdditionalInfoMaxLength)
}

func RandEventWithBallot(nameLen int, additionalInfoLen int) (*participation.Event, []byte) {
	commence := uint32(rand.Intn(1000))
	start := commence + uint32(rand.Intn(1000))
	end := start + uint32(rand.Intn(1000))

	ballot, _ := RandBallot(1 + rand.Intn(participation.BallotMaxQuestionsCount-1))
	return RandEvent(nameLen, additionalInfoLen, commence, start, end, ballot)
}

func RandEventWithoutPayload() (*participation.Event, []byte) {
	return RandEvent(participation.EventNameMaxLength, participation.EventAdditionalInfoMaxLength, 1, 2, 3, nil)
}

func RandEvent(nameLen int, additionalInfoLen int, commence uint32, start uint32, end uint32, payload serializer.Serializable) (*participation.Event, []byte) {
	e := &participation.Event{
		Name:                   RandString(nameLen),
		MilestoneIndexCommence: commence,
		MilestoneIndexStart:    start,
		MilestoneIndexEnd:      end,
		Payload:                payload,
		AdditionalInfo:         RandString(additionalInfoLen),
	}

	ms := marshalutil.New()
	ms.WriteUint8(uint8(len(e.Name)))
	ms.WriteBytes([]byte(e.Name))
	ms.WriteUint32(e.MilestoneIndexCommence)
	ms.WriteUint32(e.MilestoneIndexStart)
	ms.WriteUint32(e.MilestoneIndexEnd)
	if e.Payload != nil {
		p, err := payload.Serialize(serializer.DeSeriModePerformValidation)
		if err != nil {
			panic(err)
		}
		ms.WriteUint32(uint32(len(p)))
		ms.WriteBytes(p)
	} else {
		ms.WriteUint32(0)
	}
	ms.WriteUint16(uint16(len(e.AdditionalInfo)))
	ms.WriteBytes([]byte(e.AdditionalInfo))

	return e, ms.Bytes()
}

func TestEvent_Deserialize(t *testing.T) {
	eventWithBallot, eventWithBallotData := RandValidEventWithBallot()
	longAdditionalInfo, longAdditionalInfoData := RandEventWithBallot(participation.EventNameMaxLength, participation.EventAdditionalInfoMaxLength+1)
	emptyEvent, emptyEventData := RandEventWithoutPayload()
	startBeforeCommence, startBeforeCommenceData := RandEvent(10, 10, 1, 1, 2, nil)
	endBeforeStart, endBeforeStartData := RandEvent(10, 10, 1, 2, 2, nil)

	tests := []struct {
		name   string
		data   []byte
		target *participation.Event
		err    error
	}{
		{"ok ballot", eventWithBallotData, eventWithBallot, nil},
		{"not enough data", eventWithBallotData[:len(eventWithBallotData)-1], eventWithBallot, serializer.ErrDeserializationNotEnoughData},
		{"too long additional info", longAdditionalInfoData, longAdditionalInfo, serializer.ErrDeserializationLengthInvalid},
		{"no payload", emptyEventData, emptyEvent, participation.ErrPayloadEmpty},
		{"invalid start", startBeforeCommenceData, startBeforeCommence, participation.ErrInvalidMilestoneSequence},
		{"invalid end", endBeforeStartData, endBeforeStart, participation.ErrInvalidMilestoneSequence},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &participation.Event{}
			bytesRead, err := u.Deserialize(tt.data, serializer.DeSeriModePerformValidation)
			if tt.err != nil {
				assert.True(t, errors.Is(err, tt.err))
				return
			}
			assert.Equal(t, len(tt.data), bytesRead)
			assert.EqualValues(t, tt.target, u)
		})
	}
}

func TestEvent_Serialize(t *testing.T) {
	eventWithBallot, eventWithBallotData := RandValidEventWithBallot()
	longName, longNameData := RandEventWithBallot(participation.EventNameMaxLength+1, participation.EventAdditionalInfoMaxLength)
	longAdditionalInfo, longAdditionalInfoData := RandEventWithBallot(participation.EventNameMaxLength, participation.EventAdditionalInfoMaxLength+1)
	emptyEvent, emptyEventData := RandEventWithoutPayload()
	startBeforeCommence, startBeforeCommenceData := RandEvent(10, 10, 1, 1, 2, nil)
	endBeforeStart, endBeforeStartData := RandEvent(10, 10, 1, 2, 2, nil)

	tests := []struct {
		name   string
		source *participation.Event
		target []byte
		err    error
	}{
		{"ok ballot", eventWithBallot, eventWithBallotData, nil},
		{"too long text", longName, longNameData, participation.ErrSerializationStringLengthInvalid},
		{"too long additional info", longAdditionalInfo, longAdditionalInfoData, participation.ErrSerializationStringLengthInvalid},
		{"no payload", emptyEvent, emptyEventData, participation.ErrPayloadEmpty},
		{"invalid start", startBeforeCommence, startBeforeCommenceData, participation.ErrInvalidMilestoneSequence},
		{"invalid end", endBeforeStart, endBeforeStartData, participation.ErrInvalidMilestoneSequence},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.source.Serialize(serializer.DeSeriModePerformValidation)
			if tt.err != nil {
				assert.True(t, errors.Is(err, tt.err))
				return
			}
			assert.EqualValues(t, tt.target, data)
		})
	}
}
