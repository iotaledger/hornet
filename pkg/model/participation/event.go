package participation

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/pkg/errors"

	// import implementation
	"golang.org/x/crypto/blake2b"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// EventIDLength defines the length of a participation event ID.
	EventIDLength = blake2b.Size256

	EventNameMaxLength           = 255
	EventAdditionalInfoMaxLength = 2000
)

// EventID is the ID of an event.
type EventID [EventIDLength]byte

var (
	NullEventID = EventID{}

	ErrUnknownPayloadType               = errors.New("unknown payload type")
	ErrInvalidMilestoneSequence         = errors.New("milestone are not monotonically increasing")
	ErrPayloadEmpty                     = errors.New("payload cannot be empty")
	ErrSerializationStringLengthInvalid = errors.New("invalid string length")
	ErrSerializationUnknownType         = errors.New("invalid type")

	eventPayloadRules = &serializer.ArrayRules{
		Guards: serializer.SerializableGuard{
			ReadGuard: PayloadSelector,
			WriteGuard: func(seri serializer.Serializable) error {
				switch seri.(type) {
				case *Ballot, *Staking:
					return nil
				default:
					return ErrUnknownPayloadType
				}
			},
		},
	}
)

func (eventID EventID) ToHex() string {
	return iotago.EncodeHex(eventID[:])
}

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
	// Name is the name of the event.
	Name string
	// MilestoneIndexCommence is the milestone index the commencing period starts.
	MilestoneIndexCommence uint32
	// MilestoneIndexStart is the milestone index the holding period starts.
	MilestoneIndexStart uint32
	// MilestoneIndexEnd is the milestone index the event ends.
	MilestoneIndexEnd uint32
	// Payload is the payload of the event (ballot/staking).
	Payload serializer.Serializable
	// AdditionalInfo is an additional description text about the event.
	AdditionalInfo string
}

// ID returns the ID of the event.
func (e *Event) ID() (EventID, error) {
	data, err := e.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return EventID{}, err
	}

	return blake2b.Sum256(data), nil
}

func (e *Event) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) (int, error) {
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
		ReadPayload(func(seri serializer.Serializable) { e.Payload = seri }, deSeriMode, deSeriCtx, eventPayloadRules.Guards.ReadGuard, func(err error) error {
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

func (e *Event) Serialize(deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) ([]byte, error) {
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
			}
			return nil
		}).
		WriteString(e.Name, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize event name: %w", err)
		}, EventNameMaxLength).
		WriteNum(e.MilestoneIndexCommence, func(err error) error {
			return fmt.Errorf("unable to serialize event commence milestone: %w", err)
		}).
		WriteNum(e.MilestoneIndexStart, func(err error) error {
			return fmt.Errorf("unable to serialize event start milestone: %w", err)
		}).
		WriteNum(e.MilestoneIndexEnd, func(err error) error {
			return fmt.Errorf("unable to serialize event end milestone: %w", err)
		}).
		WritePayload(e.Payload, deSeriMode, deSeriCtx, eventPayloadRules.Guards.WriteGuard, func(err error) error {
			return fmt.Errorf("unable to serialize event inner payload: %w", err)
		}).
		WriteString(e.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to serialize event additional info: %w", err)
		}, EventAdditionalInfoMaxLength).
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
	// Name is the name of the event.
	Name string `json:"name"`
	// MilestoneIndexCommence is the milestone index the commencing period starts.
	MilestoneIndexCommence uint32 `json:"milestoneIndexCommence"`
	// MilestoneIndexStart is the milestone index the holding period starts.
	MilestoneIndexStart uint32 `json:"milestoneIndexStart"`
	// MilestoneIndexEnd is the milestone index the event ends.
	MilestoneIndexEnd uint32 `json:"milestoneIndexEnd"`
	// Payload is the payload of the event (ballot/staking).
	Payload *json.RawMessage `json:"payload"`
	// AdditionalInfo is an additional description text about the event.
	AdditionalInfo string `json:"additionalInfo"`
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

// Ballot returns the Ballot payload if this participation is for a Ballot event.
func (e *Event) Ballot() *Ballot {
	switch payload := e.Payload.(type) {
	case *Ballot:
		return payload
	default:
		return nil
	}
}

// BallotQuestions returns the questions contained in the Ballot payload if this participation contains a Ballot.
func (e *Event) BallotQuestions() Questions {
	switch payload := e.Payload.(type) {
	case *Ballot:
		return payload.Questions
	default:
		return nil
	}
}

// Staking returns the staking payload if this participation is for a Staking event.
func (e *Event) Staking() *Staking {
	switch payload := e.Payload.(type) {
	case *Staking:
		return payload
	default:
		return nil
	}
}

// Status returns a human-readable status of the event. Possible values are "upcoming", "commencing", "holding" and "ended".
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

// BallotCanOverflow returns whether a Ballot event can overflow.
func (e *Event) BallotCanOverflow(protoParas *iotago.ProtocolParameters) bool {
	ballot := e.Ballot()
	if ballot == nil {
		return false
	}

	// Check if total-supply / denominator * number of milestones can overflow uint64
	maxWeightPerMilestone := uint64(protoParas.TokenSupply) / uint64(BallotDenominator)
	maxNumberOfMilestones := math.MaxUint64 / maxWeightPerMilestone

	return uint64(e.MilestoneIndexEnd-e.MilestoneIndexStart) > maxNumberOfMilestones
}

// StakingCanOverflow returns whether a Staking event can overflow.
func (e *Event) StakingCanOverflow(protoParas *iotago.ProtocolParameters) bool {
	staking := e.Staking()
	if staking == nil {
		return false
	}

	// Check if numerator * total-supply can overflow uint64
	maxNumerator := math.MaxUint64 / uint64(protoParas.TokenSupply)
	if uint64(staking.Numerator) > maxNumerator {
		return true
	}

	// Check if total-supply * numerator/denominator * number of milestones can overflow uint64
	maxRewardPerMilestone := staking.rewardsPerMilestone(protoParas.TokenSupply)
	maxNumberOfMilestones := math.MaxUint64 / maxRewardPerMilestone

	return uint64(e.MilestoneIndexEnd-e.MilestoneIndexStart) > maxNumberOfMilestones
}

func (s *Staking) rewardsPerMilestone(amount uint64) uint64 {
	return amount * uint64(s.Numerator) / uint64(s.Denominator)
}
