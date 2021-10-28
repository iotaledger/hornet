package referendum

import (
	"encoding/json"
	"errors"
	"fmt"

	// import implementation
	"golang.org/x/crypto/blake2b"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/serializer"
	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	// ReferendumIDLength defines the length of a referendum ID.
	ReferendumIDLength = blake2b.Size256

	ReferendumNameMaxLength           = 255
	ReferendumAdditionalInfoMaxLength = 500
)

// ReferendumID is the ID of a referendum.
type ReferendumID = [ReferendumIDLength]byte

var (
	NullReferendumID = ReferendumID{}

	ErrUnknownPayloadType = errors.New("unknown payload type")
)

// PayloadSelector implements SerializableSelectorFunc for payload types.
func PayloadSelector(payloadType uint32) (serializer.Serializable, error) {
	var seri serializer.Serializable
	switch payloadType {
	case BallotPayloadTypeID:
		seri = &Ballot{}
	default:
		return nil, fmt.Errorf("%w: type %d", ErrUnknownPayloadType, payloadType)
	}
	return seri, nil
}

// Referendum
type Referendum struct {
	Name                       string
	milestoneIndexStart        uint32
	milestoneIndexStartHolding uint32
	milestoneIndexEnd          uint32
	Payload                    serializer.Serializable
	AdditionalInfo             string
}

// ID returns the ID of the referendum.
func (r *Referendum) ID() (ReferendumID, error) {
	data, err := r.Serialize(serializer.DeSeriModeNoValidation)
	if err != nil {
		return ReferendumID{}, err
	}

	return blake2b.Sum256(data), nil
}

func (r *Referendum) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode) (int, error) {
	return serializer.NewDeserializer(data).
		ReadString(&r.Name, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum name: %w", err)
		}, ReferendumNameMaxLength).
		ReadNum(&r.milestoneIndexStart, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum start milestone: %w", err)
		}).
		ReadNum(&r.milestoneIndexStartHolding, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum start holding milestone: %w", err)
		}).
		ReadNum(&r.milestoneIndexEnd, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum end milestone: %w", err)
		}).
		ReadPayload(func(seri serializer.Serializable) { r.Payload = seri }, deSeriMode, func(ty uint32) (serializer.Serializable, error) {
			switch ty {
			case BallotPayloadTypeID:
			default:
				return nil, fmt.Errorf("invalid referendum payload type ID %d: %w", ty, iotago.ErrUnsupportedPayloadType)
			}
			return PayloadSelector(ty)
		}, func(err error) error {
			return fmt.Errorf("unable to deserialize payload's inner payload: %w", err)
		}).
		ReadString(&r.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum additional info: %w", err)
		}, ReferendumAdditionalInfoMaxLength).
		ConsumedAll(func(leftOver int, err error) error {
			return fmt.Errorf("%w: unable to deserialize referendum: %d bytes are still available", err, leftOver)
		}).
		Done()
}

func (r *Referendum) Serialize(deSeriMode serializer.DeSerializationMode) ([]byte, error) {

	//TODO: validate text lengths
	return serializer.NewSerializer().
		WriteString(r.Name, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize referendum name: %w", err)
		}).
		WriteNum(r.milestoneIndexStart, func(err error) error {
			return fmt.Errorf("unable to serialize referendum start milestone: %w", err)
		}).
		WriteNum(r.milestoneIndexStartHolding, func(err error) error {
			return fmt.Errorf("unable to serialize referendum start holding milestone: %w", err)
		}).
		WriteNum(r.milestoneIndexEnd, func(err error) error {
			return fmt.Errorf("unable to serialize referendum end milestone: %w", err)
		}).
		WritePayload(r.Payload, deSeriMode, func(err error) error {
			return fmt.Errorf("unable to serialize referendum inner payload: %w", err)
		}).
		WriteString(r.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to serialize referendum additional info: %w", err)
		}).
		Serialize()
}

func (r *Referendum) MarshalJSON() ([]byte, error) {
	jReferendum := &jsonReferendum{
		Name:                       r.Name,
		MilestoneIndexStart:        r.milestoneIndexStart,
		MilestoneIndexStartHolding: r.milestoneIndexStartHolding,
		MilestoneIndexEnd:          r.milestoneIndexEnd,
		AdditionalInfo:             r.AdditionalInfo,
	}

	jsonPayload, err := r.Payload.MarshalJSON()
	if err != nil {
		return nil, err
	}
	rawMsgJsonPayload := json.RawMessage(jsonPayload)
	jReferendum.Payload = &rawMsgJsonPayload

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

// selects the json object for the given type.
func jsonPayloadSelector(ty int) (iotago.JSONSerializable, error) {
	var obj iotago.JSONSerializable
	switch uint32(ty) {
	case BallotPayloadTypeID:
		obj = &jsonBallot{}
	default:
		return nil, fmt.Errorf("unable to decode payload type from JSON: %w", ErrUnknownPayloadType)
	}
	return obj, nil
}

// jsonReferendum defines the json representation of a Referendum.
type jsonReferendum struct {
	Name                       string           `json:"name"`
	MilestoneIndexStart        uint32           `json:"milestoneIndexStart"`
	MilestoneIndexStartHolding uint32           `json:"milestoneIndexStartHolding"`
	MilestoneIndexEnd          uint32           `json:"milestoneIndexEnd"`
	Payload                    *json.RawMessage `json:"payload"`
	AdditionalInfo             string           `json:"additionalInfo"`
}

func (j *jsonReferendum) ToSerializable() (serializer.Serializable, error) {
	r := &Referendum{
		Name:                       j.Name,
		milestoneIndexStart:        j.MilestoneIndexStart,
		milestoneIndexStartHolding: j.MilestoneIndexStartHolding,
		milestoneIndexEnd:          j.MilestoneIndexEnd,
		AdditionalInfo:             j.AdditionalInfo,
	}

	jsonPayload, err := iotago.DeserializeObjectFromJSON(j.Payload, jsonPayloadSelector)
	if err != nil {
		return nil, err
	}

	r.Payload, err = jsonPayload.ToSerializable()
	if err != nil {
		return nil, err
	}

	return r, nil
}

// Helpers

func (r *Referendum) BallotQuestions() []*Question {
	switch payload := r.Payload.(type) {
	case *Ballot:
		questions := make([]*Question, len(payload.Questions))
		for i := range payload.Questions {
			questions[i] = payload.Questions[i].(*Question)
		}
		return questions
	default:
		return nil
	}
}

func (r *Referendum) Status(atIndex milestone.Index) string {
	if atIndex < r.StartMilestoneIndex() {
		return "upcoming"
	}
	if r.IsCountingVotes(atIndex) {
		return "holding"
	}
	if r.IsAcceptingVotes(atIndex) {
		return "commencing"
	}
	return "ended"
}

func (r *Referendum) StartMilestoneIndex() milestone.Index {
	return milestone.Index(r.milestoneIndexStart)
}

func (r *Referendum) StartHoldingMilestoneIndex() milestone.Index {
	return milestone.Index(r.milestoneIndexStartHolding)
}

func (r *Referendum) EndMilestoneIndex() milestone.Index {
	return milestone.Index(r.milestoneIndexEnd)
}

func (r *Referendum) ShouldAcceptVotes(forIndex milestone.Index) bool {
	return forIndex > r.StartMilestoneIndex() && forIndex <= r.EndMilestoneIndex()
}

func (r *Referendum) IsAcceptingVotes(atIndex milestone.Index) bool {
	return atIndex >= r.StartMilestoneIndex() && atIndex < r.EndMilestoneIndex()
}

func (r *Referendum) ShouldCountVotes(forIndex milestone.Index) bool {
	return forIndex > r.StartHoldingMilestoneIndex() && forIndex <= r.EndMilestoneIndex()
}

func (r *Referendum) IsCountingVotes(atIndex milestone.Index) bool {
	return atIndex >= r.StartHoldingMilestoneIndex() && atIndex < r.EndMilestoneIndex()
}
