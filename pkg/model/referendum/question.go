package referendum

import (
	"encoding/json"
	"fmt"

	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	QuestionTextMaxLength           = 255
	QuestionAdditionalInfoMaxLength = 500

	MinAnswersCount = 2
	MaxAnswersCount = 10
)

var (
	answersArrayRules = &iotago.ArrayRules{
		Min:            MinAnswersCount,
		Max:            MaxAnswersCount,
		ValidationMode: iotago.ArrayValidationModeNoDuplicates,
	}
)

// Question
type Question struct {
	Text           string
	Answers        iotago.Serializables
	AdditionalInfo string
}

func (q *Question) Deserialize(data []byte, deSeriMode iotago.DeSerializationMode) (int, error) {
	return iotago.NewDeserializer(data).
		ReadString(&q.Text, iotago.SeriSliceLengthAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum question text: %w", err)
		}, QuestionTextMaxLength).
		ReadSliceOfObjects(func(seri iotago.Serializables) { q.Answers = seri }, deSeriMode, iotago.SeriSliceLengthAsByte, iotago.TypeDenotationNone, func(_ uint32) (iotago.Serializable, error) {
			// there is no real selector, so we always return a fresh Answer
			return &Answer{}, nil
		}, answersArrayRules, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum question answers: %w", err)
		}).
		ReadString(&q.AdditionalInfo, iotago.SeriSliceLengthAsUint16, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum question additional info: %w", err)
		}, QuestionAdditionalInfoMaxLength).
		Done()
}

func (q *Question) Serialize(deSeriMode iotago.DeSerializationMode) ([]byte, error) {
	if deSeriMode.HasMode(iotago.DeSeriModePerformValidation) {
		//TODO: this should be moved as an arrayRule parameter to WriteSliceOfObjects in iota.go
		if err := answersArrayRules.CheckBounds(uint(len(q.Answers))); err != nil {
			return nil, fmt.Errorf("unable to serialize question answers: %w", err)
		}
		//TODO: this should also check the NoDups rule
	}
	return iotago.NewSerializer().
		WriteString(q.Text, iotago.SeriSliceLengthAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize referendum question text: %w", err)
		}).
		WriteSliceOfObjects(q.Answers, deSeriMode, iotago.SeriSliceLengthAsByte, nil, func(err error) error {
			return fmt.Errorf("unable to serialize referendum question answers: %w", err)
		}).
		WriteString(q.AdditionalInfo, iotago.SeriSliceLengthAsUint16, func(err error) error {
			return fmt.Errorf("unable to serialize referendum question additional info: %w", err)
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

func (j *jsonQuestion) ToSerializable() (iotago.Serializable, error) {
	payload := &Question{
		Text:           j.Text,
		AdditionalInfo: j.AdditionalInfo,
	}

	answers := make(iotago.Serializables, len(j.Answers))
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
