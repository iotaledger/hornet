package participation_test

import (
	"math/rand"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2"
)

func RandValidAnswer() (*participation.Answer, []byte) {
	value := uint8(1 + rand.Intn(254)) // Value between 1 and 254
	return RandAnswer(value, participation.AnswerTextMaxLength, participation.AnswerAdditionalInfoMaxLength)
}

func RandAnswer(value uint8, textLength int, additionalTextLength int) (*participation.Answer, []byte) {
	a := &participation.Answer{
		Value:          value,
		Text:           RandString(textLength),
		AdditionalInfo: RandString(additionalTextLength),
	}

	ms := marshalutil.New()
	ms.WriteByte(a.Value)
	ms.WriteUint8(uint8(len(a.Text)))
	ms.WriteBytes([]byte(a.Text))
	ms.WriteUint16(uint16(len(a.AdditionalInfo)))
	ms.WriteBytes([]byte(a.AdditionalInfo))

	return a, ms.Bytes()
}

func TestAnswer_Deserialize(t *testing.T) {
	randAnswer, randAnswerData := RandValidAnswer()
	longAdditionalInfoAnswer, longAdditionalInfoAnswerData := RandAnswer(1, participation.AnswerTextMaxLength, participation.AnswerAdditionalInfoMaxLength+1)
	skippedValue, skippedValueData := RandAnswer(0, participation.AnswerTextMaxLength, participation.AnswerAdditionalInfoMaxLength)
	invalidValue, invalidValueData := RandAnswer(255, participation.AnswerTextMaxLength, participation.AnswerAdditionalInfoMaxLength)

	tests := []struct {
		name   string
		data   []byte
		target *participation.Answer
		err    error
	}{
		{"ok", randAnswerData, randAnswer, nil},
		{"not enough data", randAnswerData[:len(randAnswerData)-1], randAnswer, serializer.ErrDeserializationNotEnoughData},
		{"too long additional info", longAdditionalInfoAnswerData, longAdditionalInfoAnswer, serializer.ErrDeserializationLengthInvalid},
		{"using skipped value", skippedValueData, skippedValue, participation.ErrSerializationReservedValue},
		{"using invalid value", invalidValueData, invalidValue, participation.ErrSerializationReservedValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &participation.Answer{}
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

func TestAnswer_Serialize(t *testing.T) {
	randAnswer, randAnswerData := RandValidAnswer()
	longTextAnswer, longTextAnswerData := RandAnswer(1, participation.AnswerTextMaxLength+1, participation.AnswerAdditionalInfoMaxLength)
	longAdditionalInfoAnswer, longAdditionalInfoAnswerData := RandAnswer(1, participation.AnswerTextMaxLength, participation.AnswerAdditionalInfoMaxLength+1)
	skippedValue, skippedValueData := RandAnswer(0, participation.AnswerTextMaxLength, participation.AnswerAdditionalInfoMaxLength)
	invalidValue, invalidValueData := RandAnswer(255, participation.AnswerTextMaxLength, participation.AnswerAdditionalInfoMaxLength)

	tests := []struct {
		name   string
		source *participation.Answer
		target []byte
		err    error
	}{
		{"ok", randAnswer, randAnswerData, nil},
		{"too long text", longTextAnswer, longTextAnswerData, serializer.ErrStringTooLong},
		{"too long additional info", longAdditionalInfoAnswer, longAdditionalInfoAnswerData, serializer.ErrStringTooLong},
		{"using skipped value", skippedValue, skippedValueData, participation.ErrSerializationReservedValue},
		{"using invalid value", invalidValue, invalidValueData, participation.ErrSerializationReservedValue},
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
