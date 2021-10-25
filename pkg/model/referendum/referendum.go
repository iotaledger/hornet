package referendum

import (
	"encoding/json"
	"fmt"

	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v2"

	// import implementation
	"golang.org/x/crypto/blake2b"
)

const (
	// ReferendumIDLength defines the length of a referendum ID.
	ReferendumIDLength = blake2b.Size256

	ReferendumNameMaxLength           = 255
	ReferendumAdditionalInfoMaxLength = 500

	MinQuestionsCount = 2
	MaxQuestionsCount = 10
)

// ReferendumID is the ID of a referendum.
type ReferendumID = [ReferendumIDLength]byte

var (
	questionsArrayRules = &iotago.ArrayRules{
		Min:            MinQuestionsCount,
		Max:            MaxQuestionsCount,
		ValidationMode: iotago.ArrayValidationModeNone,
	}
)

// Referendum
type Referendum struct {
	Name                  string
	MilestoneStart        milestone.Index
	MilestoneStartHolding milestone.Index
	MilestoneEnd          milestone.Index
	Questions             iotago.Serializables
	AdditionalInfo        string
}

// ID returns the ID of the referendum.
func (r *Referendum) ID() (ReferendumID, error) {
	data, err := r.Serialize(iotago.DeSeriModeNoValidation)
	if err != nil {
		return ReferendumID{}, err
	}

	return blake2b.Sum256(data), nil
}

func (r *Referendum) Deserialize(data []byte, deSeriMode iotago.DeSerializationMode) (int, error) {
	return iotago.NewDeserializer(data).
		ReadString(&r.Name, iotago.SeriSliceLengthAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum name: %w", err)
		}, ReferendumNameMaxLength).
		ReadNum(&r.MilestoneStart, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum start milestone: %w", err)
		}).
		ReadNum(&r.MilestoneStartHolding, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum start holding milestone: %w", err)
		}).
		ReadNum(&r.MilestoneEnd, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum end milestone: %w", err)
		}).
		ReadSliceOfObjects(func(seri iotago.Serializables) { r.Questions = seri }, deSeriMode, iotago.SeriSliceLengthAsByte, iotago.TypeDenotationNone, func(_ uint32) (iotago.Serializable, error) {
			// there is no real selector, so we always return a fresh Question
			return &Question{}, nil
		}, questionsArrayRules, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum questions: %w", err)
		}).
		ReadString(&r.AdditionalInfo, iotago.SeriSliceLengthAsUint16, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum additional info: %w", err)
		}, ReferendumAdditionalInfoMaxLength).
		Done()
}

func (r *Referendum) Serialize(deSeriMode iotago.DeSerializationMode) ([]byte, error) {
	return iotago.NewSerializer().
		WriteString(r.Name, iotago.SeriSliceLengthAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize referendum name: %w", err)
		}).
		WriteNum(r.MilestoneStart, func(err error) error {
			return fmt.Errorf("unable to serialize referendum start milestone: %w", err)
		}).
		WriteNum(r.MilestoneStartHolding, func(err error) error {
			return fmt.Errorf("unable to serialize referendum start holding milestone: %w", err)
		}).
		WriteNum(r.MilestoneEnd, func(err error) error {
			return fmt.Errorf("unable to serialize referendum end milestone: %w", err)
		}).
		WriteSliceOfObjects(r.Questions, deSeriMode, iotago.SeriSliceLengthAsByte, nil, func(err error) error {
			return fmt.Errorf("unable to serialize referendum questions: %w", err)
		}).
		WriteString(r.AdditionalInfo, iotago.SeriSliceLengthAsUint16, func(err error) error {
			return fmt.Errorf("unable to serialize referendum additional info: %w", err)
		}).
		Serialize()
}

func (r *Referendum) MarshalJSON() ([]byte, error) {
	jReferendum := &jsonReferendum{
		Name:                  r.Name,
		MilestoneStart:        r.MilestoneStart,
		MilestoneStartHolding: r.MilestoneStartHolding,
		MilestoneEnd:          r.MilestoneEnd,
		AdditionalInfo:        r.AdditionalInfo,
	}

	jReferendum.Questions = make([]*json.RawMessage, len(r.Questions))
	for i, question := range r.Questions {
		jsonQuestion, err := question.MarshalJSON()
		if err != nil {
			return nil, err
		}
		rawJSONQuestion := json.RawMessage(jsonQuestion)
		jReferendum.Questions[i] = &rawJSONQuestion
	}

	return json.Marshal(jReferendum)
}

func (r *Referendum) UnmarshalJSON(bytes []byte) error {
	jReferendum := &jsonReferendum{}
	if err := json.Unmarshal(bytes, jReferendum); err != nil {
		return err
	}
	seri, err := jReferendum.ToSerializable()
	if err != nil {
		return err
	}
	*r = *seri.(*Referendum)
	return nil
}

// jsonReferendum defines the json representation of a Referendum.
type jsonReferendum struct {
	Name                  string             `json:"name"`
	MilestoneStart        milestone.Index    `json:"milestoneStart"`
	MilestoneStartHolding milestone.Index    `json:"milestoneStartHolding"`
	MilestoneEnd          milestone.Index    `json:"milestoneEnd"`
	Questions             []*json.RawMessage `json:"questions"`
	AdditionalInfo        string             `json:"additionalInfo"`
}

func (j *jsonReferendum) ToSerializable() (iotago.Serializable, error) {
	payload := &Referendum{
		Name:                  j.Name,
		MilestoneStart:        j.MilestoneStart,
		MilestoneStartHolding: j.MilestoneStartHolding,
		MilestoneEnd:          j.MilestoneEnd,
		AdditionalInfo:        j.AdditionalInfo,
	}

	questions := make(iotago.Serializables, len(j.Questions))
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
