package participation

import (
	"encoding/json"
	"fmt"

	"github.com/iotaledger/hive.go/serializer/v2"
)

const (
	// BallotPayloadTypeID defines the ballot payload's type ID.
	BallotPayloadTypeID uint32 = 0

	BallotMinQuestionsCount = 1
	BallotMaxQuestionsCount = 10
)

var (
	questionsArrayRules = &serializer.ArrayRules{
		Min:            BallotMinQuestionsCount,
		Max:            BallotMaxQuestionsCount,
		ValidationMode: serializer.ArrayValidationModeNone,
		Guards: serializer.SerializableGuard{
			ReadGuard: func(ty uint32) (serializer.Serializable, error) {
				return &Question{}, nil
			},
			WriteGuard: func(seri serializer.Serializable) error {
				switch seri.(type) {
				case *Question:
					return nil
				default:
					return ErrSerializationUnknownType
				}
			},
		},
	}
)

type Questions []*Question

func (q Questions) ToSerializables() serializer.Serializables {
	seris := make(serializer.Serializables, len(q))
	for i, x := range q {
		seris[i] = x
	}
	return seris
}

func (q *Questions) FromSerializables(seris serializer.Serializables) {
	*q = make(Questions, len(seris))
	for i, seri := range seris {
		(*q)[i] = seri.(*Question)
	}
}

// Ballot can be used to define a voting participation with variable questions.
type Ballot struct {
	// Questions are the questions of the ballot and their possible answers.
	Questions Questions
}

func (q *Ballot) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) (int, error) {
	return serializer.NewDeserializer(data).
		Skip(serializer.TypeDenotationByteSize, func(err error) error {
			return fmt.Errorf("unable to skip ballot payload ID during deserialization: %w", err)
		}).
		ReadSliceOfObjects(&q.Questions, deSeriMode, deSeriCtx, serializer.SeriLengthPrefixTypeAsByte, serializer.TypeDenotationNone, questionsArrayRules, func(err error) error {
			return fmt.Errorf("unable to deserialize participation questions: %w", err)
		}).
		Done()
}

func (q *Ballot) Serialize(deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) ([]byte, error) {
	return serializer.NewSerializer().
		WriteNum(BallotPayloadTypeID, func(err error) error {
			return fmt.Errorf("%w: unable to serialize ballot payload ID", err)
		}).
		WriteSliceOfObjects(&q.Questions, deSeriMode, deSeriCtx, serializer.SeriLengthPrefixTypeAsByte, questionsArrayRules, func(err error) error {
			return fmt.Errorf("unable to serialize participation questions: %w", err)
		}).
		Serialize()
}

func (q *Ballot) MarshalJSON() ([]byte, error) {
	j := &jsonBallot{
		Type: int(BallotPayloadTypeID),
	}
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
	j := &jsonBallot{
		Type: int(BallotPayloadTypeID),
	}
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
	// Type is the type of the event.
	Type int `json:"type"`
	// Questions are the questions of the ballot and their possible answers.
	Questions []*json.RawMessage `json:"questions"`
}

func (j *jsonBallot) ToSerializable() (serializer.Serializable, error) {
	payload := &Ballot{}

	questions := make(Questions, len(j.Questions))
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
