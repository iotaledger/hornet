package participation

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/pkg/errors"

	// import implementation
	"golang.org/x/crypto/blake2b"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/serializer"
	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	// EventIDLength defines the length of a participation event ID.
	EventIDLength = blake2b.Size256

	EventNameMaxLength           = 255
	EventAdditionalInfoMaxLength = 2000
)

// EventID is the ID of an event.
type EventID = [EventIDLength]byte

var (
	NullEventID = EventID{}

	ErrUnknownPayloadType               = errors.New("unknown payload type")
	ErrInvalidMilestoneSequence         = errors.New("milestone are not monotonically increasing")
	ErrPayloadEmpty                     = errors.New("payload cannot be empty")
	ErrSerializationStringLengthInvalid = errors.New("invalid string length")
)

// PayloadSelector implements SerializableSelectorFunc for payload types.
func PayloadSelector(payloadType uint32) (serializer.Serializable, error) {
	var seri serializer.Serializable
	switch payloadType {
	case BallotPayloadTypeID:
		seri = &Ballot{}
	case StakingPayloadTypeID:
		seri = &Staking{}
	default:
		return nil, fmt.Errorf("%w: type %d", ErrUnknownPayloadType, payloadType)
	}
	return seri, nil
}

// Event
type Event struct {
	Name                   string
	MilestoneIndexCommence uint32
	MilestoneIndexStart    uint32
	MilestoneIndexEnd      uint32
	Payload                serializer.Serializable
	AdditionalInfo         string
}

// ID returns the ID of the event.
func (e *Event) ID() (EventID, error) {
	data, err := e.Serialize(serializer.DeSeriModeNoValidation)
	if err != nil {
		return EventID{}, err
	}

	return blake2b.Sum256(data), nil
}

func (e *Event) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode) (int, error) {
	return serializer.NewDeserializer(data).
		ReadString(&e.Name, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize event name: %w", err)
		}, EventNameMaxLength).
		ReadNum(&e.MilestoneIndexCommence, func(err error) error {
			return fmt.Errorf("unable to deserialize event commence milestone: %w", err)
		}).
		ReadNum(&e.MilestoneIndexStart, func(err error) error {
			return fmt.Errorf("unable to deserialize event start milestone: %w", err)
		}).
		ReadNum(&e.MilestoneIndexEnd, func(err error) error {
			return fmt.Errorf("unable to deserialize event end milestone: %w", err)
		}).
		ReadPayload(func(seri serializer.Serializable) { e.Payload = seri }, deSeriMode, func(ty uint32) (serializer.Serializable, error) {
			switch ty {
			case BallotPayloadTypeID:
			case StakingPayloadTypeID:
			default:
				return nil, fmt.Errorf("invalid event payload type ID %d: %w", ty, ErrUnknownPayloadType)
			}
			return PayloadSelector(ty)
		}, func(err error) error {
			return fmt.Errorf("unable to deserialize payload's inner payload: %w", err)
		}).
		ReadString(&e.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to deserialize event additional info: %w", err)
		}, EventAdditionalInfoMaxLength).
		ConsumedAll(func(leftOver int, err error) error {
			return fmt.Errorf("%w: unable to deserialize event: %d bytes are still available", err, leftOver)
		}).
		AbortIf(func(err error) error {
			if deSeriMode.HasMode(serializer.DeSeriModePerformValidation) {
				if e.MilestoneIndexCommence >= e.MilestoneIndexStart {
					return fmt.Errorf("%w: unable to deserialize event, commence milestone needs to be before the start milestone: %d vs %d", ErrInvalidMilestoneSequence, e.MilestoneIndexCommence, e.MilestoneIndexStart)
				}
				if e.MilestoneIndexStart >= e.MilestoneIndexEnd {
					return fmt.Errorf("%w: unable to deserialize event, start milestone needs to be before the end milestone: %d vs %d", ErrInvalidMilestoneSequence, e.MilestoneIndexStart, e.MilestoneIndexEnd)
				}
				if e.Payload == nil {
					return fmt.Errorf("%w: unable to deserialize event, payload cannot be empty", ErrPayloadEmpty)
				}
			}
			return nil
		}).
		Done()
}

func (e *Event) Serialize(deSeriMode serializer.DeSerializationMode) ([]byte, error) {
	return serializer.NewSerializer().
		AbortIf(func(err error) error {
			if deSeriMode.HasMode(serializer.DeSeriModePerformValidation) {
				if e.MilestoneIndexCommence >= e.MilestoneIndexStart {
					return fmt.Errorf("%w: unable to serialize event, commence milestone needs to be before the start: %d vs %d", ErrInvalidMilestoneSequence, e.MilestoneIndexCommence, e.MilestoneIndexStart)
				}
				if e.MilestoneIndexStart >= e.MilestoneIndexEnd {
					return fmt.Errorf("%w: unable to serialize event, start milestone needs to be before the end: %d vs %d", ErrInvalidMilestoneSequence, e.MilestoneIndexStart, e.MilestoneIndexEnd)
				}
				if e.Payload == nil {
					return fmt.Errorf("%w: unable to serialize event, payload cannot be empty", ErrPayloadEmpty)
				}
				if len(e.Name) > EventNameMaxLength {
					return fmt.Errorf("%w: unable to serialize event, name too long. Max allowed %d", ErrSerializationStringLengthInvalid, EventNameMaxLength)
				}
				if len(e.AdditionalInfo) > EventAdditionalInfoMaxLength {
					return fmt.Errorf("%w: unable to serialize event, additional info too long. Max allowed %d", ErrSerializationStringLengthInvalid, EventAdditionalInfoMaxLength)
				}
			}
			return nil
		}).
		WriteString(e.Name, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize event name: %w", err)
		}).
		WriteNum(e.MilestoneIndexCommence, func(err error) error {
			return fmt.Errorf("unable to serialize event commence milestone: %w", err)
		}).
		WriteNum(e.MilestoneIndexStart, func(err error) error {
			return fmt.Errorf("unable to serialize event start milestone: %w", err)
		}).
		WriteNum(e.MilestoneIndexEnd, func(err error) error {
			return fmt.Errorf("unable to serialize event end milestone: %w", err)
		}).
		WritePayload(e.Payload, deSeriMode, func(err error) error {
			return fmt.Errorf("unable to serialize event inner payload: %w", err)
		}).
		WriteString(e.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to serialize event additional info: %w", err)
		}).
		Serialize()
}

func (e *Event) MarshalJSON() ([]byte, error) {
	j := &jsonEvent{
		Name:                   e.Name,
		MilestoneIndexCommence: e.MilestoneIndexCommence,
		MilestoneIndexStart:    e.MilestoneIndexStart,
		MilestoneIndexEnd:      e.MilestoneIndexEnd,
		AdditionalInfo:         e.AdditionalInfo,
	}

	jsonPayload, err := e.Payload.MarshalJSON()
	if err != nil {
		return nil, err
	}
	rawMsgJsonPayload := json.RawMessage(jsonPayload)
	j.Payload = &rawMsgJsonPayload

	return json.Marshal(j)
}

func (e *Event) UnmarshalJSON(bytes []byte) error {
	j := &jsonEvent{}
	if err := json.Unmarshal(bytes, j); err != nil {
		return err
	}
	seri, err := j.ToSerializable()
	if err != nil {
		return err
	}
	*e = *seri.(*Event)
	return nil
}

// selects the json object for the given type.
func jsonPayloadSelector(ty int) (iotago.JSONSerializable, error) {
	var obj iotago.JSONSerializable
	switch uint32(ty) {
	case BallotPayloadTypeID:
		obj = &jsonBallot{}
	case StakingPayloadTypeID:
		obj = &jsonStaking{}
	default:
		return nil, fmt.Errorf("unable to decode payload type from JSON: %w", ErrUnknownPayloadType)
	}
	return obj, nil
}

// jsonEvent defines the json representation of a Event.
type jsonEvent struct {
	Name                   string           `json:"name"`
	MilestoneIndexCommence uint32           `json:"milestoneIndexCommence"`
	MilestoneIndexStart    uint32           `json:"milestoneIndexStart"`
	MilestoneIndexEnd      uint32           `json:"milestoneIndexEnd"`
	Payload                *json.RawMessage `json:"payload"`
	AdditionalInfo         string           `json:"additionalInfo"`
}

func (j *jsonEvent) ToSerializable() (serializer.Serializable, error) {
	e := &Event{
		Name:                   j.Name,
		MilestoneIndexCommence: j.MilestoneIndexCommence,
		MilestoneIndexStart:    j.MilestoneIndexStart,
		MilestoneIndexEnd:      j.MilestoneIndexEnd,
		AdditionalInfo:         j.AdditionalInfo,
	}

	if j.Payload == nil {
		return nil, ErrPayloadEmpty
	}

	jsonPayload, err := iotago.DeserializeObjectFromJSON(j.Payload, jsonPayloadSelector)
	if err != nil {
		return nil, err
	}

	e.Payload, err = jsonPayload.ToSerializable()
	if err != nil {
		return nil, err
	}

	return e, nil
}

// Helpers

func (e *Event) payloadType() uint32 {
	switch e.Payload.(type) {
	case *Ballot:
		return BallotPayloadTypeID
	case *Staking:
		return StakingPayloadTypeID
	default:
		panic(ErrUnknownPayloadType)
	}
}

// BallotQuestions returns the questions contained in the Ballot payload if this participation contains a Ballot.
func (e *Event) BallotQuestions() []*Question {
	switch payload := e.Payload.(type) {
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

// Staking returns the staking payload if this participation is for a Staking event
func (e *Event) Staking() *Staking {
	switch payload := e.Payload.(type) {
	case *Staking:
		return payload
	default:
		return nil
	}
}

// Status returns a human-readable status of the event. Possible values are "upcoming", "commencing", "holding" and "ended"
func (e *Event) Status(atIndex milestone.Index) string {
	if atIndex < e.CommenceMilestoneIndex() {
		return "upcoming"
	}
	if e.IsCountingParticipation(atIndex) {
		return "holding"
	}
	if e.IsAcceptingParticipation(atIndex) {
		return "commencing"
	}
	return "ended"
}

// CommenceMilestoneIndex returns the milestone index the commencing phase of the participation starts.
func (e *Event) CommenceMilestoneIndex() milestone.Index {
	return milestone.Index(e.MilestoneIndexCommence)
}

// StartMilestoneIndex returns the milestone index the holding phase of the participation starts.
func (e *Event) StartMilestoneIndex() milestone.Index {
	return milestone.Index(e.MilestoneIndexStart)
}

// EndMilestoneIndex returns the milestone index the participation ends.
func (e *Event) EndMilestoneIndex() milestone.Index {
	return milestone.Index(e.MilestoneIndexEnd)
}

// ShouldAcceptParticipation returns true if the event should accept the participation for the given milestone index.
func (e *Event) ShouldAcceptParticipation(forIndex milestone.Index) bool {
	return forIndex > e.CommenceMilestoneIndex() && forIndex <= e.EndMilestoneIndex()
}

// IsAcceptingParticipation returns true if the event already commenced or started the holding phase for the given milestone index.
func (e *Event) IsAcceptingParticipation(atIndex milestone.Index) bool {
	return atIndex >= e.CommenceMilestoneIndex() && atIndex < e.EndMilestoneIndex()
}

// ShouldCountParticipation returns true if the event should count the participation for the given milestone index.
func (e *Event) ShouldCountParticipation(forIndex milestone.Index) bool {
	return forIndex > e.StartMilestoneIndex() && forIndex <= e.EndMilestoneIndex()
}

// IsCountingParticipation returns true if the event already started the holding phase for the given milestone index.
func (e *Event) IsCountingParticipation(atIndex milestone.Index) bool {
	return atIndex >= e.StartMilestoneIndex() && atIndex < e.EndMilestoneIndex()
}

func (e *Event) StakingCanOverflow() bool {
	staking := e.Staking()
	if staking == nil {
		return false
	}

	// Check if numerator * total-supply can overflow uint64
	maxNumerator := math.MaxUint64 / uint64(iotago.TokenSupply)
	if uint64(staking.Numerator) > maxNumerator {
		return true
	}

	// Check if total-supply * numerator/denominator * number of milestones can overflow uint64
	maxRewardPerMilestone := uint64(iotago.TokenSupply) * uint64(staking.Numerator) / uint64(staking.Denominator)
	maxNumberOfMilestones := math.MaxUint64 / maxRewardPerMilestone
	if uint64(e.MilestoneIndexEnd-e.MilestoneIndexStart) > maxNumberOfMilestones {
		return true
	}
	return false
}
