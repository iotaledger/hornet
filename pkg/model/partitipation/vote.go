package partitipation

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/iotaledger/hive.go/serializer"
)

// Vote holds the vote for a partitipation and the answer to each question
type Vote struct {
	ReferendumID ReferendumID
	Answers      []byte
}

func (v *Vote) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode) (int, error) {
	return serializer.NewDeserializer(data).
		ReadArrayOf32Bytes(&v.ReferendumID, func(err error) error {
			return fmt.Errorf("unable to deserialize partitipation ID in vote: %w", err)
		}).
		ReadVariableByteSlice(&v.Answers, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize answers in vote: %w", err)
		}).
		Done()
}

func (v *Vote) Serialize(deSeriMode serializer.DeSerializationMode) ([]byte, error) {
	return serializer.NewSerializer().
		WriteBytes(v.ReferendumID[:], func(err error) error {
			return fmt.Errorf("unable to serialize partitipation ID in vote: %w", err)
		}).
		WriteVariableByteSlice(v.Answers, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize answers in vote: %w", err)
		}).
		Serialize()
}

func (v *Vote) MarshalJSON() ([]byte, error) {
	jVote := &jsonVote{}
	jVote.ReferendumID = hex.EncodeToString(v.ReferendumID[:])
	jVote.Answers = hex.EncodeToString(v.Answers)
	return json.Marshal(jVote)
}

func (v *Vote) UnmarshalJSON(bytes []byte) error {
	jVote := &jsonVote{}
	if err := json.Unmarshal(bytes, jVote); err != nil {
		return err
	}
	seri, err := jVote.ToSerializable()
	if err != nil {
		return err
	}
	*v = *seri.(*Vote)
	return nil
}

// jsonVote defines the JSON representation of a Vote.
type jsonVote struct {
	ReferendumID string `json:"referendumID"`
	Answers      string `json:"answers"`
}

func (j *jsonVote) ToSerializable() (serializer.Serializable, error) {
	vote := &Vote{
		ReferendumID: ReferendumID{},
		Answers:      []byte{},
	}

	referendumIDBytes, err := hex.DecodeString(j.ReferendumID)
	if err != nil {
		return nil, fmt.Errorf("unable to decode partitipation ID from JSON in vote: %w", err)
	}
	copy(vote.ReferendumID[:], referendumIDBytes)

	answersBytes, err := hex.DecodeString(j.Answers)
	if err != nil {
		return nil, fmt.Errorf("unable to decode answers from JSON in vote: %w", err)
	}
	vote.Answers = make([]byte, len(answersBytes))
	copy(vote.Answers, answersBytes)

	return vote, nil
}
