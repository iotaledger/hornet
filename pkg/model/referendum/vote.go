package referendum

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/blake2b"

	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	// ReferendumIDLength defines the length of a referendum ID.
	ReferendumIDLength = blake2b.Size256
)

// ReferendumID is the ID of a referendum.
type ReferendumID = [ReferendumIDLength]byte

// Vote
type Vote struct {
	ReferendumID ReferendumID
	Answers      []byte
}

func (v *Vote) Deserialize(data []byte, deSeriMode iotago.DeSerializationMode) (int, error) {
	return iotago.NewDeserializer(data).
		ReadArrayOf32Bytes(&v.ReferendumID, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum ID in vote: %w", err)
		}).
		ReadVariableByteSlice(&v.Answers, iotago.SeriSliceLengthAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize answers in vote: %w", err)
		}).
		Done()
}

func (v *Vote) Serialize(deSeriMode iotago.DeSerializationMode) ([]byte, error) {
	return iotago.NewSerializer().
		WriteBytes(v.ReferendumID[:], func(err error) error {
			return fmt.Errorf("unable to serialize referendum ID in vote: %w", err)
		}).
		WriteVariableByteSlice(v.Answers, iotago.SeriSliceLengthAsByte, func(err error) error {
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

func (j *jsonVote) ToSerializable() (iotago.Serializable, error) {
	vote := &Vote{
		ReferendumID: ReferendumID{},
		Answers:      []byte{},
	}

	referendumIDBytes, err := hex.DecodeString(j.ReferendumID)
	if err != nil {
		return nil, fmt.Errorf("unable to decode referendum ID from JSON in vote: %w", err)
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
