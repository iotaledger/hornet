package participation_test

import (
	"math/rand"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer"
)

func RandParticipations(participationCount int) (*participation.Participations, []byte) {

	p := &participation.Participations{
		Participations: serializer.Serializables{},
	}

	var pBytes [][]byte
	for i := 0; i < participationCount; i++ {
		q, bytes := RandParticipation(rand.Intn(participation.BallotMaxQuestionsCount))
		p.Participations = append(p.Participations, q)
		pBytes = append(pBytes, bytes)
	}

	ms := marshalutil.New()
	ms.WriteUint8(uint8(len(pBytes)))
	for _, bytes := range pBytes {
		ms.WriteBytes(bytes)
	}

	return p, ms.Bytes()
}

func TestParticipations_Deserialize(t *testing.T) {
	validParticipations, validParticipationsData := RandParticipations(1)
	maxParticipations, maxParticipationsData := RandParticipations(255)
	emptyParticipations, emptyParticipationsData := RandParticipations(0)

	tests := []struct {
		name   string
		data   []byte
		target *participation.Participations
		err    error
	}{
		{"ok", validParticipationsData, validParticipations, nil},
		{"not enough data", validParticipationsData[:len(validParticipationsData)-1], validParticipations, serializer.ErrDeserializationNotEnoughData},
		{"max participations", maxParticipationsData, maxParticipations, nil},
		{"empty participations", emptyParticipationsData, emptyParticipations, serializer.ErrArrayValidationMinElementsNotReached},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &participation.Participations{}
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

func TestParticipations_Serialize(t *testing.T) {
	validParticipations, validParticipationsData := RandParticipations(1)
	maxParticipations, maxParticipationsData := RandParticipations(255)
	emptyParticipations, emptyParticipationsData := RandParticipations(0)
	tooManyParticipations, tooManyParticipationsData := RandParticipations(256)

	tests := []struct {
		name   string
		source *participation.Participations
		target []byte
		err    error
	}{
		{"ok", validParticipations, validParticipationsData, nil},
		{"max participations", maxParticipations, maxParticipationsData, nil},
		{"empty participations", emptyParticipations, emptyParticipationsData, serializer.ErrArrayValidationMinElementsNotReached},
		{"too many participations", tooManyParticipations, tooManyParticipationsData, serializer.ErrArrayValidationMaxElementsExceeded},
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
