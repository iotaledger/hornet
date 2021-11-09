package participation

import (
	"encoding/json"
	"fmt"

	"github.com/iotaledger/hive.go/serializer"
)

const (
	// BallotPayloadTypeID defines the ballot payload's type ID.
	BallotPayloadTypeID uint32 = 0

	MinQuestionsCount = 1
	MaxQuestionsCount = 10
)

var (
	questionsArrayRules = &serializer.ArrayRules{
		Min:            MinQuestionsCount,
		Max:            MaxQuestionsCount,
		ValidationMode: serializer.ArrayValidationModeNone,
	}
)

// Ballot can be used to define a voting participation with variable questions.
type Ballot struct {
	Questions serializer.Serializables
}

func (q *Ballot) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode) (int, error) {
	return serializer.NewDeserializer(data).
		Skip(serializer.TypeDenotationByteSize, func(err error) error {
			return fmt.Errorf("unable to skip ballot payload ID during deserialization: %w", err)
		}).
		ReadSliceOfObjects(func(seri serializer.Serializables) { q.Questions = seri }, deSeriMode, serializer.SeriLengthPrefixTypeAsByte, serializer.TypeDenotationNone, func(_ uint32) (serializer.Serializable, error) {
			// there is no real selector, so we always return a fresh Question
			return &Question{}, nil
		}, questionsArrayRules, func(err error) error {
			return fmt.Errorf("unable to deserialize participation questions: %w", err)
		}).
		Done()
}

func (q *Ballot) Serialize(deSeriMode serializer.DeSerializationMode) ([]byte, error) {
	return serializer.NewSerializer().
		AbortIf(func(err error) error {
			if deSeriMode.HasMode(serializer.DeSeriModePerformValidation) {
				if err := questionsArrayRules.CheckBounds(uint(len(q.Questions))); err != nil {
					return fmt.Errorf("unable to serialize participation questions: %w", err)
				}
			}
			return nil
		}).
		WriteNum(BallotPayloadTypeID, func(err error) error {
			return fmt.Errorf("%w: unable to serialize ballot payload ID", err)
		}).
		WriteSliceOfObjects(q.Questions, deSeriMode, serializer.SeriLengthPrefixTypeAsByte, nil, func(err error) error {
			return fmt.Errorf("unable to serialize participation questions: %w", err)
		}).
		Serialize()
}

func (q *Ballot) MarshalJSON() ([]byte, error) {
	j := &jsonBallot{}
	j.Questions = make([]*json.RawMessage, len(q.Questions))
	for i, question := range q.Questions {
		jsonQuestion, err := question.MarshalJSON()
		if err != nil {
			return nil, err
		}
		rawJSONQuestion := json.RawMessage(jsonQuestion)
		j.Questions[i] = &rawJSONQuestion
	}

	return json.Marshal(j)
}

func (q *Ballot) UnmarshalJSON(bytes []byte) error {
	j := &jsonBallot{}
	j.Type = int(BallotPayloadTypeID)
	if err := json.Unmarshal(bytes, j); err != nil {
		return err
	}
	seri, err := j.ToSerializable()
	if err != nil {
		return err
	}
	*q = *seri.(*Ballot)
	return nil
}

// jsonBallot defines the json representation of a Ballot.
type jsonBallot struct {
	Type      int                `json:"type"`
	Questions []*json.RawMessage `json:"questions"`
}

func (j *jsonBallot) ToSerializable() (serializer.Serializable, error) {
	payload := &Ballot{}

	questions := make(serializer.Serializables, len(j.Questions))
	for i, ele := range j.Questions {
		question := &Question{}

		rawJSON, err := ele.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("pos %d: %w", i, err)
		}

		if err := json.Unmarshal(rawJSON, question); err != nil {
			return nil, fmt.Errorf("pos %d: %w", i, err)
		}

		questions[i] = question
	}
	payload.Questions = questions

	return payload, nil
}
