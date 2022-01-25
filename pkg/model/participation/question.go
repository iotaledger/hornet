package participation

import (
	"encoding/json"
	"fmt"

	"github.com/iotaledger/hive.go/serializer/v2"
)

const (
	QuestionTextMaxLength           = 255
	QuestionAdditionalInfoMaxLength = 500

	QuestionMinAnswersCount = 2
	QuestionMaxAnswersCount = 10
)

var (
	answersArrayRules = &serializer.ArrayRules{
		Min:            QuestionMinAnswersCount,
		Max:            QuestionMaxAnswersCount,
		ValidationMode: serializer.ArrayValidationModeNoDuplicates,
		UniquenessSliceFunc: func(next []byte) []byte {
			return next[:1] // Check answer value for uniqueness
		},
		Guards: serializer.SerializableGuard{
			ReadGuard: func(ty uint32) (serializer.Serializable, error) {
				return &Answer{}, nil
			},
			WriteGuard: func(seri serializer.Serializable) error {
				switch seri.(type) {
				case *Answer:
					return nil
				default:
					return ErrSerializationUnknownType
				}
			},
		},
	}
)

type Answers []*Answer

func (a Answers) ToSerializables() serializer.Serializables {
	seris := make(serializer.Serializables, len(a))
	for i, x := range a {
		seris[i] = x
	}
	return seris
}

func (a *Answers) FromSerializables(seris serializer.Serializables) {
	*a = make(Answers, len(seris))
	for i, seri := range seris {
		(*a)[i] = seri.(*Answer)
	}
}

// Question defines a single question inside a Ballot that can have multiple Answers.
type Question struct {
	// Text is the text of the question.
	Text string
	// Answers are the possible answers to the question.
	Answers Answers
	// AdditionalInfo is an additional description text about the question.
	AdditionalInfo string
}

func (q *Question) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) (int, error) {
	return serializer.NewDeserializer(data).
		ReadString(&q.Text, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize participation question text: %w", err)
		}, QuestionTextMaxLength).
		ReadSliceOfObjects(&q.Answers, deSeriMode, deSeriCtx, serializer.SeriLengthPrefixTypeAsByte, serializer.TypeDenotationNone, answersArrayRules, func(err error) error {
			return fmt.Errorf("unable to deserialize participation question answers: %w", err)
		}).
		ReadString(&q.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to deserialize participation question additional info: %w", err)
		}, QuestionAdditionalInfoMaxLength).
		Done()
}

func (q *Question) Serialize(deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) ([]byte, error) {
	return serializer.NewSerializer().
		WriteString(q.Text, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize participation question text: %w", err)
		}, QuestionTextMaxLength).
		WriteSliceOfObjects(&q.Answers, deSeriMode, deSeriCtx, serializer.SeriLengthPrefixTypeAsByte, answersArrayRules, func(err error) error {
			return fmt.Errorf("unable to serialize participation question answers: %w", err)
		}).
		WriteString(q.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to serialize participation question additional info: %w", err)
		}, QuestionAdditionalInfoMaxLength).
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
	// Text is the text of the question.
	Text string `json:"text"`
	// Answers are the possible answers to the question.
	Answers []*json.RawMessage `json:"answers"`
	// AdditionalInfo is an additional description text about the question.
	AdditionalInfo string `json:"additionalInfo"`
}

func (j *jsonQuestion) ToSerializable() (serializer.Serializable, error) {
	payload := &Question{
		Text:           j.Text,
		AdditionalInfo: j.AdditionalInfo,
	}

	answers := make(Answers, len(j.Answers))
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
		answers[i] = q.Answers[i]
	}
	return answers
}

// answerValueForByte checks if the given value is a valid answer and maps any other values to AnswerValueInvalid
func (q *Question) answerValueForByte(byteValue byte) uint8 {
	if byteValue == 0 {
		return 0
	}
	for i := range q.Answers {
		a := q.Answers[i]
		if a.Value == byteValue {
			return a.Value
		}
	}
	return AnswerValueInvalid
}
