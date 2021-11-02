package participation

import (
	"encoding/json"
	"fmt"

	"github.com/iotaledger/hive.go/serializer"
)

const (
	MinParticipationCount = 1
	MaxParticipationCount = 255
)

var (
	participationsArrayRules = &serializer.ArrayRules{
		Min:            MinParticipationCount,
		Max:            MaxParticipationCount,
		ValidationMode: serializer.ArrayValidationModeNone,
	}
)

// Participations holds the participation for multiple events
type Participations struct {
	Participations serializer.Serializables
}

func (p *Participations) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode) (int, error) {
	return serializer.NewDeserializer(data).
		ReadSliceOfObjects(func(seri serializer.Serializables) { p.Participations = seri }, deSeriMode, serializer.SeriLengthPrefixTypeAsByte, serializer.TypeDenotationNone, func(_ uint32) (serializer.Serializable, error) {
			// there is no real selector, so we always return a fresh Participation
			return &Participation{}, nil
		}, participationsArrayRules, func(err error) error {
			return fmt.Errorf("unable to deserialize votes: %w", err)
		}).
		Done()
}

func (p *Participations) Serialize(deSeriMode serializer.DeSerializationMode) ([]byte, error) {
	return serializer.NewSerializer().
		WriteSliceOfObjects(p.Participations, deSeriMode, serializer.SeriLengthPrefixTypeAsByte, nil, func(err error) error {
			return fmt.Errorf("unable to serialize votes: %w", err)
		}).
		Serialize()
}

func (p *Participations) MarshalJSON() ([]byte, error) {
	j := &jsonParticipations{}

	j.Participations = make([]*json.RawMessage, len(p.Participations))
	for i, vote := range p.Participations {
		jsonVote, err := vote.MarshalJSON()
		if err != nil {
			return nil, err
		}
		rawJSONVote := json.RawMessage(jsonVote)
		j.Participations[i] = &rawJSONVote
	}

	return json.Marshal(j)
}

func (p *Participations) UnmarshalJSON(bytes []byte) error {
	j := &jsonParticipations{}
	if err := json.Unmarshal(bytes, j); err != nil {
		return err
	}
	seri, err := j.ToSerializable()
	if err != nil {
		return err
	}
	*p = *seri.(*Participations)
	return nil
}

// jsonParticipations defines the JSON representation of Participations.
type jsonParticipations struct {
	Participations []*json.RawMessage `json:"participations"`
}

func (j *jsonParticipations) ToSerializable() (serializer.Serializable, error) {
	payload := &Participations{}

	votes := make(serializer.Serializables, len(j.Participations))
	for i, ele := range j.Participations {
		vote := &Answer{}

		rawJSON, err := ele.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("pos %d: %w", i, err)
		}

		if err := json.Unmarshal(rawJSON, vote); err != nil {
			return nil, fmt.Errorf("pos %d: %w", i, err)
		}

		votes[i] = vote
	}
	payload.Participations = votes

	return payload, nil
}
