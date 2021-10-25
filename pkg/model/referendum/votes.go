package referendum

import (
	"encoding/json"
	"fmt"

	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	MinVotesCount = 1
	MaxVotesCount = 255
)

var (
	votesArrayRules = &iotago.ArrayRules{
		Min:            MinVotesCount,
		Max:            MaxVotesCount,
		ValidationMode: iotago.ArrayValidationModeNone,
	}
)

// Votes holds the votes for multiple referendums
type Votes struct {
	Votes iotago.Serializables
}

func (v *Votes) Deserialize(data []byte, deSeriMode iotago.DeSerializationMode) (int, error) {
	return iotago.NewDeserializer(data).
		ReadSliceOfObjects(func(seri iotago.Serializables) { v.Votes = seri }, deSeriMode, iotago.SeriSliceLengthAsByte, iotago.TypeDenotationNone, func(_ uint32) (iotago.Serializable, error) {
			// there is no real selector, so we always return a fresh Vote
			return &Vote{}, nil
		}, votesArrayRules, func(err error) error {
			return fmt.Errorf("unable to deserialize votes: %w", err)
		}).
		Done()
}

func (v *Votes) Serialize(deSeriMode iotago.DeSerializationMode) ([]byte, error) {
	return iotago.NewSerializer().
		WriteSliceOfObjects(v.Votes, deSeriMode, iotago.SeriSliceLengthAsByte, nil, func(err error) error {
			return fmt.Errorf("unable to serialize votes: %w", err)
		}).
		Serialize()
}

func (v *Votes) MarshalJSON() ([]byte, error) {
	jVotes := &jsonVotes{}

	jVotes.Votes = make([]*json.RawMessage, len(v.Votes))
	for i, vote := range v.Votes {
		jsonVote, err := vote.MarshalJSON()
		if err != nil {
			return nil, err
		}
		rawJSONVote := json.RawMessage(jsonVote)
		jVotes.Votes[i] = &rawJSONVote
	}

	return json.Marshal(jVotes)
}

func (v *Votes) UnmarshalJSON(bytes []byte) error {
	jVotes := &jsonVotes{}
	if err := json.Unmarshal(bytes, jVotes); err != nil {
		return err
	}
	seri, err := jVotes.ToSerializable()
	if err != nil {
		return err
	}
	*v = *seri.(*Votes)
	return nil
}

// jsonVotes defines the JSON representation of Votes.
type jsonVotes struct {
	Votes []*json.RawMessage `json:"votes"`
}

func (j *jsonVotes) ToSerializable() (iotago.Serializable, error) {
	payload := &Votes{}

	votes := make(iotago.Serializables, len(j.Votes))
	for i, ele := range j.Votes {
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
	payload.Votes = votes

	return payload, nil
}
