package participation

import (
	"fmt"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/serializer/v2"
)

// NewEventBuilder creates a new EventBuilder.
func NewEventBuilder(name string, milestoneCommence milestone.Index, milestoneBeginHolding milestone.Index, milestoneEnd milestone.Index, additionalInfo string) *EventBuilder {
	return &EventBuilder{
		event: &Event{
			Name:                   name,
			MilestoneIndexCommence: uint32(milestoneCommence),
			MilestoneIndexStart:    uint32(milestoneBeginHolding),
			MilestoneIndexEnd:      uint32(milestoneEnd),
			AdditionalInfo:         additionalInfo,
		},
	}
}

// EventBuilder is used to easily build up a Event.
type EventBuilder struct {
	event *Event
	err   error
}

// Payload sets the payload to embed within the message.
func (rb *EventBuilder) Payload(seri serializer.Serializable) *EventBuilder {
	if rb.err != nil {
		return rb
	}
	switch seri.(type) {
	case *Ballot:
	case *Staking:
	case nil:
	default:
		rb.err = fmt.Errorf("%w: unsupported type %T", ErrUnknownPayloadType, seri)
		return rb
	}
	rb.event.Payload = seri
	return rb
}

// Build builds the Event.
func (rb *EventBuilder) Build() (*Event, error) {
	if rb.err != nil {
		return nil, rb.err
	}

	if _, err := rb.event.Serialize(serializer.DeSeriModePerformValidation, nil); err != nil {
		return nil, fmt.Errorf("unable to build participation: %w", err)
	}
	return rb.event, nil
}
