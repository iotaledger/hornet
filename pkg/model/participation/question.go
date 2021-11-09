package participation

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/serializer"
)

const (
	QuestionTextMaxLength           = 255
	QuestionAdditionalInfoMaxLength = 500

	MinAnswersCount = 2
	MaxAnswersCount = 10
)

var (
	answersArrayRules = &serializer.ArrayRules{
		Min:            MinAnswersCount,
		Max:            MaxAnswersCount,
		ValidationMode: serializer.ArrayValidationModeNone,
	}

	ErrDuplicateAnswerValue = errors.New("duplicate answer value found")
)

// Question defines a single question inside a Ballot that can have multiple Answers.
type Question struct {
	Text           string
	Answers        serializer.Serializables
	AdditionalInfo string
}

func (q *Question) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode) (int, error) {
	return serializer.NewDeserializer(data).
		ReadString(&q.Text, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize participation question text: %w", err)
		}, QuestionTextMaxLength).
		ReadSliceOfObjects(func(seri serializer.Serializables) { q.Answers = seri }, deSeriMode, serializer.SeriLengthPrefixTypeAsByte, serializer.TypeDenotationNone, func(_ uint32) (serializer.Serializable, error) {
			// there is no real selector, so we always return a fresh Answer
			return &Answer{}, nil
		}, answersArrayRules, func(err error) error {
			return fmt.Errorf("unable to deserialize participation question answers: %w", err)
		}).
		ReadString(&q.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to deserialize participation question additional info: %w", err)
		}, QuestionAdditionalInfoMaxLength).
		AbortIf(func(err error) error {
			if deSeriMode.HasMode(serializer.DeSeriModePerformValidation) {
				seenValues := make(map[uint8]struct{})
				for _, s := range q.Answers {
					switch a := s.(type) {
					case *Answer:
						if _, found := seenValues[a.Value]; found {
							return ErrDuplicateAnswerValue
						}
						seenValues[a.Value] = struct{}{}
					default:
						return errors.New("invalid answer type")
					}
				}
			}
			return nil
		}).
		Done()
}

func (q *Question) Serialize(deSeriMode serializer.DeSerializationMode) ([]byte, error) {
	return serializer.NewSerializer().
		AbortIf(func(err error) error {
			if deSeriMode.HasMode(serializer.DeSeriModePerformValidation) {
				if len(q.Text) > QuestionTextMaxLength {
					return fmt.Errorf("%w: text too long. Max allowed %d", ErrSerializationStringLengthInvalid, QuestionTextMaxLength)
				}
				if len(q.AdditionalInfo) > QuestionAdditionalInfoMaxLength {
					return fmt.Errorf("%w: additional info too long. Max allowed %d", ErrSerializationStringLengthInvalid, QuestionAdditionalInfoMaxLength)
				}
				if err := answersArrayRules.CheckBounds(uint(len(q.Answers))); err != nil {
					return fmt.Errorf("unable to serialize question answers: %w", err)
				}
				seenValues := make(map[uint8]struct{})
				for _, s := range q.Answers {
					switch a := s.(type) {
					case *Answer:
						if _, found := seenValues[a.Value]; found {
							return ErrDuplicateAnswerValue
						}
						seenValues[a.Value] = struct{}{}
					default:
						return errors.New("invalid answer type")
					}
				}
			}
			return nil
		}).
		WriteString(q.Text, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize participation question text: %w", err)
		}).
		WriteSliceOfObjects(q.Answers, deSeriMode, serializer.SeriLengthPrefixTypeAsByte, nil, func(err error) error {
			return fmt.Errorf("unable to serialize participation question answers: %w", err)
		}).
		WriteString(q.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to serialize participation question additional info: %w", err)
		}).
		Serialize()
}

func (q *Question) MarshalJSON() ([]byte, error) {
	jQuestion := &jsonQuestion{
		Text:           q.Text,
		AdditionalInfo: q.AdditionalInfo,
	}

	jQuestion.Answers = make([]*json.RawMessage, len(q.Answers))
	for i, answer := range q.Answers {
		jsonAnswer, err := answer.MarshalJSON()
		if err != nil {
			return nil, err
		}
		rawJSONAnswer := json.RawMessage(jsonAnswer)
		jQuestion.Answers[i] = &rawJSONAnswer
	}

	return json.Marshal(jQuestion)
}

func (q *Question) UnmarshalJSON(bytes []byte) error {
	jQuestion := &jsonQuestion{}
	if err := json.Unmarshal(bytes, jQuestion); err != nil {
		return err
	}
	seri, err := jQuestion.ToSerializable()
	if err != nil {
		return err
	}
	*q = *seri.(*Question)
	return nil
}

// jsonQuestion defines the json representation of a Question.
type jsonQuestion struct {
	Text           string             `json:"text"`
	Answers        []*json.RawMessage `json:"answers"`
	AdditionalInfo string             `json:"additionalInfo"`
}

func (j *jsonQuestion) ToSerializable() (serializer.Serializable, error) {
	payload := &Question{
		Text:           j.Text,
		AdditionalInfo: j.AdditionalInfo,
	}

	answers := make(serializer.Serializables, len(j.Answers))
	for i, ele := range j.Answers {
		answer := &Answer{}

		rawJSON, err := ele.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("pos %d: %w", i, err)
		}

		if err := json.Unmarshal(rawJSON, answer); err != nil {
			return nil, fmt.Errorf("pos %d: %w", i, err)
		}

		answers[i] = answer
	}
	payload.Answers = answers

	return payload, nil
}

// QuestionAnswers returns the possible answers for a Question
func (q *Question) QuestionAnswers() []*Answer {
	answers := make([]*Answer, len(q.Answers))
	for i := range q.Answers {
		answers[i] = q.Answers[i].(*Answer)
	}
	return answers
}

// answerValueForByte checks if the given value is a valid answer and maps any other values to AnswerValueInvalid
func (q *Question) answerValueForByte(byteValue byte) uint8 {
	if byteValue == 0 {
		return 0
	}
	for i := range q.Answers {
		a := q.Answers[i].(*Answer)
		if a.Value == byteValue {
			return a.Value
		}
	}
	return AnswerValueInvalid
}
