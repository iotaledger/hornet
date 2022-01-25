package participation

import (
	"encoding/json"
	"fmt"

	"github.com/iotaledger/hive.go/serializer/v2"
)

const (
	ParticipationsMinParticipationCount = 1
	ParticipationsMaxParticipationCount = 255
)

var (
	participationsArrayRules = &serializer.ArrayRules{
		Min:            ParticipationsMinParticipationCount,
		Max:            ParticipationsMaxParticipationCount,
		ValidationMode: serializer.ArrayValidationModeNoDuplicates,
		UniquenessSliceFunc: func(next []byte) []byte {
			return next[:EventIDLength] // Verify that the EventIDs are unique
		},
		Guards: serializer.SerializableGuard{
			ReadGuard: func(ty uint32) (serializer.Serializable, error) {
				return &Participation{}, nil
			},
			WriteGuard: func(seri serializer.Serializable) error {
				switch seri.(type) {
				case *Participation:
					return nil
				default:
					return ErrSerializationUnknownType
				}
			},
		},
	}
)

type Participations []*Participation

func (s Participations) ToSerializables() serializer.Serializables {
	seris := make(serializer.Serializables, len(s))
	for i, x := range s {
		seris[i] = x
	}
	return seris
}

func (s *Participations) FromSerializables(seris serializer.Serializables) {
	*s = make(Participations, len(seris))
	for i, seri := range seris {
		(*s)[i] = seri.(*Participation)
	}
}

// ParticipationPayload holds the participation for multiple events
type ParticipationPayload struct {
	// Participations holds the participation for multiple events.
	Participations Participations
}

func (p *ParticipationPayload) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) (int, error) {
	return serializer.NewDeserializer(data).
		ReadSliceOfObjects(&p.Participations, deSeriMode, deSeriCtx, serializer.SeriLengthPrefixTypeAsByte, serializer.TypeDenotationNone, participationsArrayRules, func(err error) error {
			return fmt.Errorf("unable to deserialize participations: %w", err)
		}).
		Done()
}

func (p *ParticipationPayload) Serialize(deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) ([]byte, error) {
	return serializer.NewSerializer().
		WriteSliceOfObjects(&p.Participations, deSeriMode, deSeriCtx, serializer.SeriLengthPrefixTypeAsByte, participationsArrayRules, func(err error) error {
			return fmt.Errorf("unable to serialize participations: %w", err)
		}).
		Serialize()
}

func (p *ParticipationPayload) MarshalJSON() ([]byte, error) {
	j := &jsonParticipationPayload{}

	j.Participations = make([]*json.RawMessage, len(p.Participations))
	for i, participation := range p.Participations {
		jsonParticipation, err := participation.MarshalJSON()
		if err != nil {
			return nil, err
		}
		rawJSONParticipation := json.RawMessage(jsonParticipation)
		j.Participations[i] = &rawJSONParticipation
	}

	return json.Marshal(j)
}

func (p *ParticipationPayload) UnmarshalJSON(bytes []byte) error {
	j := &jsonParticipationPayload{}
	if err := json.Unmarshal(bytes, j); err != nil {
		return err
	}
	seri, err := j.ToSerializable()
	if err != nil {
		return err
	}
	*p = *seri.(*ParticipationPayload)
	return nil
}

// jsonParticipationPayload defines the JSON representation of ParticipationPayload.
type jsonParticipationPayload struct {
	// Participations holds the participation for multiple events.
	Participations []*json.RawMessage `json:"participations"`
}

func (j *jsonParticipationPayload) ToSerializable() (serializer.Serializable, error) {
	payload := &ParticipationPayload{}

	participations := make(Participations, len(j.Participations))
	for i, ele := range j.Participations {
		participation := &Participation{}

		rawJSON, err := ele.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("pos %d: %w", i, err)
		}

		if err := json.Unmarshal(rawJSON, participation); err != nil {
			return nil, fmt.Errorf("pos %d: %w", i, err)
		}

		participations[i] = participation
	}
	payload.Participations = participations

	return payload, nil
}
