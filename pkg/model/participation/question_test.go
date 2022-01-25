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

func RandValidQuestion() (*participation.Question, []byte) {
	return RandQuestion(participation.QuestionTextMaxLength, participation.QuestionAdditionalInfoMaxLength, []byte{1, 2, 3, 4})
}

func RandQuestion(textLength int, additionalTextLength int, answerValues []uint8) (*participation.Question, []byte) {
	q := &participation.Question{
		Text:           RandString(textLength),
		Answers:        participation.Answers{},
		AdditionalInfo: RandString(additionalTextLength),
	}

	var answersBytes [][]byte
	for _, value := range answerValues {
		a, b := RandAnswer(value, rand.Intn(participation.AnswerTextMaxLength), rand.Intn(participation.AnswerAdditionalInfoMaxLength))
		answersBytes = append(answersBytes, b)
		q.Answers = append(q.Answers, a)
	}

	ms := marshalutil.New()
	ms.WriteUint8(uint8(len(q.Text)))
	ms.WriteBytes([]byte(q.Text))
	ms.WriteUint8(uint8(len(answerValues)))
	for _, bytes := range answersBytes {
		ms.WriteBytes(bytes)
	}
	ms.WriteUint16(uint16(len(q.AdditionalInfo)))
	ms.WriteBytes([]byte(q.AdditionalInfo))

	return q, ms.Bytes()
}

func TestQuestion_Deserialize(t *testing.T) {
	validQuestion, validQuestionData := RandQuestion(participation.QuestionTextMaxLength, participation.QuestionAdditionalInfoMaxLength, []uint8{1, 2})
	longAdditionalInfoQuestion, longAdditionalInfoQuestionData := RandQuestion(participation.QuestionTextMaxLength, participation.QuestionAdditionalInfoMaxLength+1, []uint8{1, 2})
	maxAnswersQuestion, maxAnswersQuestionData := RandQuestion(participation.QuestionTextMaxLength, participation.QuestionAdditionalInfoMaxLength, []uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	noAnswersQuestion, noAnswersQuestionData := RandQuestion(participation.QuestionTextMaxLength, participation.QuestionAdditionalInfoMaxLength, []uint8{})
	tooManyAnswersQuestion, tooManyAnswersQuestionData := RandQuestion(participation.QuestionTextMaxLength, participation.QuestionAdditionalInfoMaxLength, []uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})
	duplicateAnswerQuestion, duplicateAnswerQuestionData := RandQuestion(participation.QuestionTextMaxLength, participation.QuestionAdditionalInfoMaxLength, []uint8{1, 2, 2})
	tests := []struct {
		name   string
		data   []byte
		target *participation.Question
		err    error
	}{
		{"ok", validQuestionData, validQuestion, nil},
		{"not enough data", validQuestionData[:len(validQuestionData)-1], validQuestion, serializer.ErrDeserializationNotEnoughData},
		{"too long additional info", longAdditionalInfoQuestionData, longAdditionalInfoQuestion, serializer.ErrDeserializationLengthInvalid},
		{"max answers", maxAnswersQuestionData, maxAnswersQuestion, nil},
		{"no answers", noAnswersQuestionData, noAnswersQuestion, serializer.ErrArrayValidationMinElementsNotReached},
		{"too many answers", tooManyAnswersQuestionData, tooManyAnswersQuestion, serializer.ErrArrayValidationMaxElementsExceeded},
		{"duplicate answers", duplicateAnswerQuestionData, duplicateAnswerQuestion, serializer.ErrArrayValidationViolatesUniqueness},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &participation.Question{}
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

func TestQuestion_Serialize(t *testing.T) {
	validQuestion, validQuestionData := RandQuestion(participation.QuestionTextMaxLength, participation.QuestionAdditionalInfoMaxLength, []uint8{1, 2})
	longTextQuestion, longTextQuestionData := RandQuestion(participation.QuestionTextMaxLength+1, participation.QuestionAdditionalInfoMaxLength, []uint8{1, 2})
	longAdditionalInfoQuestion, longAdditionalInfoQuestionData := RandQuestion(participation.QuestionTextMaxLength, participation.QuestionAdditionalInfoMaxLength+1, []uint8{1, 2})
	maxAnswersQuestion, maxAnswersQuestionData := RandQuestion(participation.QuestionTextMaxLength, participation.QuestionAdditionalInfoMaxLength, []uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	noAnswersQuestion, noAnswersQuestionData := RandQuestion(participation.QuestionTextMaxLength, participation.QuestionAdditionalInfoMaxLength, []uint8{})
	tooManyAnswersQuestion, tooManyAnswersQuestionData := RandQuestion(participation.QuestionTextMaxLength, participation.QuestionAdditionalInfoMaxLength, []uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})
	duplicateAnswerQuestion, duplicateAnswerQuestionData := RandQuestion(participation.QuestionTextMaxLength, participation.QuestionAdditionalInfoMaxLength, []uint8{1, 2, 2})

	tests := []struct {
		name   string
		source *participation.Question
		target []byte
		err    error
	}{
		{"ok", validQuestion, validQuestionData, nil},
		{"too long text", longTextQuestion, longTextQuestionData, serializer.ErrStringTooLong},
		{"too long additional info", longAdditionalInfoQuestion, longAdditionalInfoQuestionData, serializer.ErrStringTooLong},
		{"max answers", maxAnswersQuestion, maxAnswersQuestionData, nil},
		{"no answers", noAnswersQuestion, noAnswersQuestionData, serializer.ErrArrayValidationMinElementsNotReached},
		{"too many answers", tooManyAnswersQuestion, tooManyAnswersQuestionData, serializer.ErrArrayValidationMaxElementsExceeded},
		{"duplicate answers", duplicateAnswerQuestion, duplicateAnswerQuestionData, serializer.ErrArrayValidationViolatesUniqueness},
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
