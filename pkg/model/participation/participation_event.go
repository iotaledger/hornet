package participation

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
	// ParticipationEventIDLength defines the length of a participation event ID.
	ParticipationEventIDLength = blake2b.Size256

	ReferendumNameMaxLength           = 255
	ReferendumAdditionalInfoMaxLength = 500
)

// ParticipationEventID is the ID of a participation.
type ParticipationEventID = [ParticipationEventIDLength]byte

var (
	NullParticipationEventID = ParticipationEventID{}

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

// ParticipationEvent
type ParticipationEvent struct {
	Name                   string
	milestoneIndexCommence uint32
	milestoneIndexStart    uint32
	milestoneIndexEnd      uint32
	Payload                serializer.Serializable
	AdditionalInfo         string
}

// ID returns the ID of the event.
func (r *ParticipationEvent) ID() (ParticipationEventID, error) {
	data, err := r.Serialize(serializer.DeSeriModeNoValidation)
	if err != nil {
		return ParticipationEventID{}, err
	}

	return blake2b.Sum256(data), nil
}

func (r *ParticipationEvent) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode) (int, error) {
	return serializer.NewDeserializer(data).
		ReadString(&r.Name, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize participation name: %w", err)
		}, ReferendumNameMaxLength).
		ReadNum(&r.milestoneIndexCommence, func(err error) error {
			return fmt.Errorf("unable to deserialize participation commence milestone: %w", err)
		}).
		ReadNum(&r.milestoneIndexStart, func(err error) error {
			return fmt.Errorf("unable to deserialize participation start milestone: %w", err)
		}).
		ReadNum(&r.milestoneIndexEnd, func(err error) error {
			return fmt.Errorf("unable to deserialize participation end milestone: %w", err)
		}).
		ReadPayload(func(seri serializer.Serializable) { r.Payload = seri }, deSeriMode, func(ty uint32) (serializer.Serializable, error) {
			switch ty {
			case BallotPayloadTypeID:
			default:
				return nil, fmt.Errorf("invalid participation payload type ID %d: %w", ty, iotago.ErrUnsupportedPayloadType)
			}
			return PayloadSelector(ty)
		}, func(err error) error {
			return fmt.Errorf("unable to deserialize payload's inner payload: %w", err)
		}).
		ReadString(&r.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to deserialize participation additional info: %w", err)
		}, ReferendumAdditionalInfoMaxLength).
		ConsumedAll(func(leftOver int, err error) error {
			return fmt.Errorf("%w: unable to deserialize participation: %d bytes are still available", err, leftOver)
		}).
		Done()
}

func (r *ParticipationEvent) Serialize(deSeriMode serializer.DeSerializationMode) ([]byte, error) {

	//TODO: validate text lengths
	return serializer.NewSerializer().
		WriteString(r.Name, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize participation name: %w", err)
		}).
		WriteNum(r.milestoneIndexCommence, func(err error) error {
			return fmt.Errorf("unable to serialize participation commence milestone: %w", err)
		}).
		WriteNum(r.milestoneIndexStart, func(err error) error {
			return fmt.Errorf("unable to serialize participation start milestone: %w", err)
		}).
		WriteNum(r.milestoneIndexEnd, func(err error) error {
			return fmt.Errorf("unable to serialize participation end milestone: %w", err)
		}).
		WritePayload(r.Payload, deSeriMode, func(err error) error {
			return fmt.Errorf("unable to serialize participation inner payload: %w", err)
		}).
		WriteString(r.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to serialize participation additional info: %w", err)
		}).
		Serialize()
}

func (r *ParticipationEvent) MarshalJSON() ([]byte, error) {
	jReferendum := &jsonParticipationEvent{
		Name:                   r.Name,
		MilestoneIndexCommence: r.milestoneIndexCommence,
		MilestoneIndexStart:    r.milestoneIndexStart,
		MilestoneIndexEnd:      r.milestoneIndexEnd,
		AdditionalInfo:         r.AdditionalInfo,
	}

	jsonPayload, err := r.Payload.MarshalJSON()
	if err != nil {
		return nil, err
	}
	rawMsgJsonPayload := json.RawMessage(jsonPayload)
	jReferendum.Payload = &rawMsgJsonPayload

	return json.Marshal(jReferendum)
}

func (r *ParticipationEvent) UnmarshalJSON(bytes []byte) error {
	jReferendum := &jsonParticipationEvent{}
	if err := json.Unmarshal(bytes, jReferendum); err != nil {
		return err
	}
	seri, err := jReferendum.ToSerializable()
	if err != nil {
		return err
	}
	*r = *seri.(*ParticipationEvent)
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

// jsonParticipationEvent defines the json representation of a ParticipationEvent.
type jsonParticipationEvent struct {
	Name                   string           `json:"name"`
	MilestoneIndexCommence uint32           `json:"milestoneIndexCommence"`
	MilestoneIndexStart    uint32           `json:"milestoneIndexStart"`
	MilestoneIndexEnd      uint32           `json:"milestoneIndexEnd"`
	Payload                *json.RawMessage `json:"payload"`
	AdditionalInfo         string           `json:"additionalInfo"`
}

func (j *jsonParticipationEvent) ToSerializable() (serializer.Serializable, error) {
	r := &ParticipationEvent{
		Name:                   j.Name,
		milestoneIndexCommence: j.MilestoneIndexCommence,
		milestoneIndexStart:    j.MilestoneIndexStart,
		milestoneIndexEnd:      j.MilestoneIndexEnd,
		AdditionalInfo:         j.AdditionalInfo,
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

func (r *ParticipationEvent) BallotQuestions() []*Question {
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

func (r *ParticipationEvent) Status(atIndex milestone.Index) string {
	if atIndex < r.CommenceMilestoneIndex() {
		return "upcoming"
	}
	if r.IsCountingParticipation(atIndex) {
		return "holding"
	}
	if r.IsAcceptingParticipation(atIndex) {
		return "commencing"
	}
	return "ended"
}

func (r *ParticipationEvent) CommenceMilestoneIndex() milestone.Index {
	return milestone.Index(r.milestoneIndexCommence)
}

func (r *ParticipationEvent) StartMilestoneIndex() milestone.Index {
	return milestone.Index(r.milestoneIndexStart)
}

func (r *ParticipationEvent) EndMilestoneIndex() milestone.Index {
	return milestone.Index(r.milestoneIndexEnd)
}

func (r *ParticipationEvent) ShouldAcceptParticipation(forIndex milestone.Index) bool {
	return forIndex > r.CommenceMilestoneIndex() && forIndex <= r.EndMilestoneIndex()
}

func (r *ParticipationEvent) IsAcceptingParticipation(atIndex milestone.Index) bool {
	return atIndex >= r.CommenceMilestoneIndex() && atIndex < r.EndMilestoneIndex()
}

func (r *ParticipationEvent) ShouldCountParticipation(forIndex milestone.Index) bool {
	return forIndex > r.StartMilestoneIndex() && forIndex <= r.EndMilestoneIndex()
}

func (r *ParticipationEvent) IsCountingParticipation(atIndex milestone.Index) bool {
	return atIndex >= r.StartMilestoneIndex() && atIndex < r.EndMilestoneIndex()
}
