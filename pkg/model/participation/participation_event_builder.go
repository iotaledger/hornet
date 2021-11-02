package participation

import (
	"fmt"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/serializer"
)

// NewParticipationEventBuilder creates a new ParticipationEventBuilder.
func NewParticipationEventBuilder(name string, milestoneCommence milestone.Index, milestoneBeginHolding milestone.Index, milestoneEnd milestone.Index, additionalInfo string) *ParticipationEventBuilder {
	return &ParticipationEventBuilder{
		event: &ParticipationEvent{
			Name:                   name,
			milestoneIndexCommence: uint32(milestoneCommence),
			milestoneIndexStart:    uint32(milestoneBeginHolding),
			milestoneIndexEnd:      uint32(milestoneEnd),
			AdditionalInfo:         additionalInfo,
		},
	}
}

// ParticipationEventBuilder is used to easily build up a ParticipationEvent.
type ParticipationEventBuilder struct {
	event *ParticipationEvent
	err   error
}

// Payload sets the payload to embed within the message.
func (rb *ParticipationEventBuilder) Payload(seri serializer.Serializable) *ParticipationEventBuilder {
	if rb.err != nil {
		return rb
	}
	switch seri.(type) {
	case *Ballot:
	case nil:
	default:
		rb.err = fmt.Errorf("%w: unsupported type %T", ErrUnknownPayloadType, seri)
		return rb
	}
	rb.event.Payload = seri
	return rb
}

// Build builds the ParticipationEvent.
func (rb *ParticipationEventBuilder) Build() (*ParticipationEvent, error) {
	if rb.err != nil {
		return nil, rb.err
	}

	if _, err := rb.event.Serialize(serializer.DeSeriModePerformValidation); err != nil {
		return nil, fmt.Errorf("unable to build participation: %w", err)
	}
	return rb.event, nil
}
