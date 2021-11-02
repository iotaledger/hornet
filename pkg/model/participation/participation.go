package participation

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/iotaledger/hive.go/serializer"
)

// Participation holds the participation for an event and the optional answer to a ballot
type Participation struct {
	ParticipationEventID ParticipationEventID
	Answers              []byte
}

func (p *Participation) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode) (int, error) {
	return serializer.NewDeserializer(data).
		ReadArrayOf32Bytes(&p.ParticipationEventID, func(err error) error {
			return fmt.Errorf("unable to deserialize participation ID in vote: %w", err)
		}).
		ReadVariableByteSlice(&p.Answers, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize answers in vote: %w", err)
		}).
		Done()
}

func (p *Participation) Serialize(deSeriMode serializer.DeSerializationMode) ([]byte, error) {
	return serializer.NewSerializer().
		WriteBytes(p.ParticipationEventID[:], func(err error) error {
			return fmt.Errorf("unable to serialize participation ID in vote: %w", err)
		}).
		WriteVariableByteSlice(p.Answers, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize answers in vote: %w", err)
		}).
		Serialize()
}

func (p *Participation) MarshalJSON() ([]byte, error) {
	j := &jsonParticipation{}
	j.ParticipationEventID = hex.EncodeToString(p.ParticipationEventID[:])
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
	ParticipationEventID string `json:"participationEventId"`
	Answers              string `json:"answers"`
}

func (j *jsonParticipation) ToSerializable() (serializer.Serializable, error) {
	vote := &Participation{
		ParticipationEventID: ParticipationEventID{},
		Answers:              []byte{},
	}

	referendumIDBytes, err := hex.DecodeString(j.ParticipationEventID)
	if err != nil {
		return nil, fmt.Errorf("unable to decode participation ID from JSON in vote: %w", err)
	}
	copy(vote.ParticipationEventID[:], referendumIDBytes)

	answersBytes, err := hex.DecodeString(j.Answers)
	if err != nil {
		return nil, fmt.Errorf("unable to decode answers from JSON in vote: %w", err)
	}
	vote.Answers = make([]byte, len(answersBytes))
	copy(vote.Answers, answersBytes)

	return vote, nil
}
