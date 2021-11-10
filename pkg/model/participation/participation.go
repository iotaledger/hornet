package participation

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/iotaledger/hive.go/serializer"
)

var (
	ErrParticipationTooManyAnswers = errors.New("participation contains more answers than what a ballot can hold")
)

// Participation holds the participation for an event and the optional answer to a ballot
type Participation struct {
	EventID EventID
	Answers []byte
}

func (p *Participation) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode) (int, error) {
	return serializer.NewDeserializer(data).
		ReadArrayOf32Bytes(&p.EventID, func(err error) error {
			return fmt.Errorf("unable to deserialize eventID in participation: %w", err)
		}).
		ReadVariableByteSlice(&p.Answers, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize answers in participation: %w", err)
		}).
		AbortIf(func(err error) error {
			if deSeriMode.HasMode(serializer.DeSeriModePerformValidation) {
				if len(p.Answers) > BallotMaxQuestionsCount {
					return ErrParticipationTooManyAnswers
				}
			}
			return nil
		}).
		Done()
}

func (p *Participation) Serialize(deSeriMode serializer.DeSerializationMode) ([]byte, error) {
	return serializer.NewSerializer().
		AbortIf(func(err error) error {
			if deSeriMode.HasMode(serializer.DeSeriModePerformValidation) {
				if len(p.Answers) > BallotMaxQuestionsCount {
					return ErrParticipationTooManyAnswers
				}
			}
			return nil
		}).
		WriteBytes(p.EventID[:], func(err error) error {
			return fmt.Errorf("unable to serialize eventID in participation: %w", err)
		}).
		WriteVariableByteSlice(p.Answers, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize answers in participation: %w", err)
		}).
		Serialize()
}

func (p *Participation) MarshalJSON() ([]byte, error) {
	j := &jsonParticipation{}
	j.EventID = hex.EncodeToString(p.EventID[:])
	j.Answers = hex.EncodeToString(p.Answers)
	return json.Marshal(j)
}

func (p *Participation) UnmarshalJSON(bytes []byte) error {
	j := &jsonParticipation{}
	if err := json.Unmarshal(bytes, j); err != nil {
		return err
	}
	seri, err := j.ToSerializable()
	if err != nil {
		return err
	}
	*p = *seri.(*Participation)
	return nil
}

// jsonParticipation defines the JSON representation of a Participation.
type jsonParticipation struct {
	EventID string `json:"eventId"`
	Answers string `json:"answers"`
}

func (j *jsonParticipation) ToSerializable() (serializer.Serializable, error) {
	p := &Participation{
		EventID: EventID{},
		Answers: []byte{},
	}

	eventIDBytes, err := hex.DecodeString(j.EventID)
	if err != nil {
		return nil, fmt.Errorf("unable to decode event ID from JSON in participation: %w", err)
	}
	copy(p.EventID[:], eventIDBytes)

	answersBytes, err := hex.DecodeString(j.Answers)
	if err != nil {
		return nil, fmt.Errorf("unable to decode answers from JSON in participation: %w", err)
	}
	p.Answers = make([]byte, len(answersBytes))
	copy(p.Answers, answersBytes)

	return p, nil
}
