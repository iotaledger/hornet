package participation_test

import (
	"math/rand"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/testutil"
	iotago "github.com/iotaledger/iota.go/v3"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var protoParas = &iotago.ProtocolParameters{
	TokenSupply: 2_779_530_283_277_761,
}

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

func RandValidEventWithStaking() (*participation.Event, []byte) {
	return RandEventWithStaking(participation.EventNameMaxLength, participation.EventAdditionalInfoMaxLength)
}

func RandEventWithStaking(nameLen int, additionalInfoLen int) (*participation.Event, []byte) {
	commence := uint32(rand.Intn(1000))
	start := commence + uint32(rand.Intn(1000))
	end := start + uint32(rand.Intn(1000))

	staking, _ := RandStaking(participation.StakingTextMaxLength, participation.StakingSymbolMaxLength, uint32(1+rand.Intn(10000)), uint32(1+rand.Intn(10000)), participation.StakingAdditionalInfoMaxLength)
	return RandEvent(nameLen, additionalInfoLen, commence, start, end, staking)
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
		p, err := payload.Serialize(serializer.DeSeriModePerformValidation, nil)
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
	eventWithStaking, eventWithStakingData := RandValidEventWithStaking()
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
		{"ok staking", eventWithStakingData, eventWithStaking, nil},
		{"not enough data", eventWithBallotData[:len(eventWithBallotData)-1], eventWithBallot, serializer.ErrDeserializationNotEnoughData},
		{"too long additional info", longAdditionalInfoData, longAdditionalInfo, serializer.ErrDeserializationLengthInvalid},
		{"no payload", emptyEventData, emptyEvent, participation.ErrPayloadEmpty},
		{"invalid start", startBeforeCommenceData, startBeforeCommence, participation.ErrInvalidMilestoneSequence},
		{"invalid end", endBeforeStartData, endBeforeStart, participation.ErrInvalidMilestoneSequence},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &participation.Event{}
			bytesRead, err := u.Deserialize(tt.data, serializer.DeSeriModePerformValidation, nil)
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
	eventWithStaking, eventWithStakingData := RandValidEventWithStaking()
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
		{"ok staking", eventWithStaking, eventWithStakingData, nil},
		{"too long text", longName, longNameData, serializer.ErrStringTooLong},
		{"too long additional info", longAdditionalInfo, longAdditionalInfoData, serializer.ErrStringTooLong},
		{"no payload", emptyEvent, emptyEventData, participation.ErrPayloadEmpty},
		{"invalid start", startBeforeCommence, startBeforeCommenceData, participation.ErrInvalidMilestoneSequence},
		{"invalid end", endBeforeStart, endBeforeStartData, participation.ErrInvalidMilestoneSequence},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.source.Serialize(serializer.DeSeriModePerformValidation, nil)
			if tt.err != nil {
				assert.True(t, errors.Is(err, tt.err))
				return
			}
			assert.EqualValues(t, tt.target, data)
		})
	}
}

func RandBallotEventWithIndexes(commence uint32, start uint32, end uint32) *participation.Event {
	ballot, _ := RandBallot(1 + rand.Intn(participation.BallotMaxQuestionsCount-1))
	event, _ := RandEvent(participation.EventNameMaxLength, participation.EventAdditionalInfoMaxLength, commence, start, end, ballot)
	return event
}

func RandStakingEvent(nominator uint32, denominator uint32, duration uint32) *participation.Event {
	eb := participation.NewEventBuilder(
		RandString(100),
		5,
		6,
		milestone.Index(6+duration),
		RandString(100),
	)

	eb.Payload(&participation.Staking{
		Text:           RandString(10),
		Symbol:         RandString(3),
		Numerator:      nominator,
		Denominator:    denominator,
		AdditionalInfo: RandString(100),
	})

	event, err := eb.Build()
	if err != nil {
		panic(err)
	}
	return event
}

func TestEventBallotCanOverflow(t *testing.T) {
	require.True(t, RandBallotEventWithIndexes(1, 5, 10000000).BallotCanOverflow(protoParas))
	require.False(t, RandBallotEventWithIndexes(1, 5, 10).BallotCanOverflow(protoParas))
}

func TestEventStakingCanOverflow(t *testing.T) {
	require.False(t, RandStakingEvent(6_636, 1, 1).StakingCanOverflow(protoParas))
	require.True(t, RandStakingEvent(6_637, 1, 1).StakingCanOverflow(protoParas))

	require.True(t, RandStakingEvent(6_636, 1, 2).StakingCanOverflow(protoParas))

	require.False(t, RandStakingEvent(6_636, 10, 10).StakingCanOverflow(protoParas))
	require.True(t, RandStakingEvent(6_636, 10, 11).StakingCanOverflow(protoParas))

	require.False(t, RandStakingEvent(1, 1, 6_636).StakingCanOverflow(protoParas))
	require.True(t, RandStakingEvent(1, 1, 6_637).StakingCanOverflow(protoParas))

	require.True(t, RandStakingEvent(1, 10, 66_367).StakingCanOverflow(protoParas))

	require.False(t, RandStakingEvent(1, 1_000_000, 777_600).StakingCanOverflow(protoParas))
}
