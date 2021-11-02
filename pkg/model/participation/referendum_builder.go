package participation

import (
	"fmt"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/serializer"
)

// NewReferendumBuilder creates a new ReferendumBuilder.
func NewReferendumBuilder(name string, milestoneCommence milestone.Index, milestoneBeginHolding milestone.Index, milestoneEnd milestone.Index, additionalInfo string) *ReferendumBuilder {
	return &ReferendumBuilder{
		r: &ParticipationEvent{
			Name:                   name,
			milestoneIndexCommence: uint32(milestoneCommence),
			milestoneIndexStart:    uint32(milestoneBeginHolding),
			milestoneIndexEnd:      uint32(milestoneEnd),
			AdditionalInfo:         additionalInfo,
		},
	}
}

// ReferendumBuilder is used to easily build up a ParticipationEvent.
type ReferendumBuilder struct {
	r   *ParticipationEvent
	err error
}

// Payload sets the payload to embed within the message.
func (rb *ReferendumBuilder) Payload(seri serializer.Serializable) *ReferendumBuilder {
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
	rb.r.Payload = seri
	return rb
}

// Build builds the ParticipationEvent.
func (rb *ReferendumBuilder) Build() (*ParticipationEvent, error) {
	if rb.err != nil {
		return nil, rb.err
	}

	if _, err := rb.r.Serialize(serializer.DeSeriModePerformValidation); err != nil {
		return nil, fmt.Errorf("unable to build participation: %w", err)
	}
	return rb.r, nil
}
