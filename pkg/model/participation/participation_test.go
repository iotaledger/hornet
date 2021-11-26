package participation_test

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer"
	"github.com/iotaledger/iota.go/v2/tpkg"
)

func RandParticipation(answerCount int) (*participation.Participation, []byte) {
	return RandParticipationWithEventID(RandEventID(), answerCount)
}

func RandParticipationWithEventID(eventID participation.EventID, answerCount int) (*participation.Participation, []byte) {
	answers := tpkg.RandBytes(answerCount)
	if answerCount == 0 {
		answers = []byte{} // RandBytes returns nil if empty
	}

	p := &participation.Participation{
		EventID: eventID,
		Answers: answers,
	}

	ms := marshalutil.New()
	ms.WriteBytes(p.EventID[:])
	ms.WriteUint8(uint8(answerCount))
	ms.WriteBytes(p.Answers)

	return p, ms.Bytes()
}

func TestParticipation_Deserialize(t *testing.T) {
	validParticipation, validParticipationData := RandParticipation(1)
	emptyParticipation, emptyParticipationData := RandParticipation(0)
	maxParticipation, maxParticipationData := RandParticipation(participation.BallotMaxQuestionsCount)
	tooManyParticipation, tooManyParticipationData := RandParticipation(participation.BallotMaxQuestionsCount + 1)

	tests := []struct {
		name   string
		data   []byte
		target *participation.Participation
		err    error
	}{
		{"ok", validParticipationData, validParticipation, nil},
		{"not enough data", validParticipationData[:len(validParticipationData)-1], validParticipation, serializer.ErrDeserializationNotEnoughData},
		{"no answers", emptyParticipationData, emptyParticipation, nil},
		{"max answers", maxParticipationData, maxParticipation, nil},
		{"too many answers", tooManyParticipationData, tooManyParticipation, participation.ErrParticipationTooManyAnswers},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &participation.Participation{}
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

func TestParticipation_Serialize(t *testing.T) {
	validParticipation, validParticipationData := RandParticipation(1)
	emptyParticipation, emptyParticipationData := RandParticipation(0)
	maxParticipation, maxParticipationData := RandParticipation(participation.BallotMaxQuestionsCount)
	tooManyParticipation, tooManyParticipationData := RandParticipation(participation.BallotMaxQuestionsCount + 1)

	tests := []struct {
		name   string
		source *participation.Participation
		target []byte
		err    error
	}{
		{"ok", validParticipation, validParticipationData, nil},
		{"no answers", emptyParticipation, emptyParticipationData, nil},
		{"max answers", maxParticipation, maxParticipationData, nil},
		{"too many answers", tooManyParticipation, tooManyParticipationData, participation.ErrParticipationTooManyAnswers},
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
