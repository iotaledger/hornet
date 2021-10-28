package referendum

import (
	"fmt"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/serializer"
)

// NewReferendumBuilder creates a new ReferendumBuilder.
func NewReferendumBuilder(name string, milestoneStart milestone.Index, milestoneStartHolding milestone.Index, milestoneEnd milestone.Index, additionalInfo string) *ReferendumBuilder {
	return &ReferendumBuilder{
		r: &Referendum{
			Name:                       name,
			milestoneIndexStart:        uint32(milestoneStart),
			milestoneIndexStartHolding: uint32(milestoneStartHolding),
			milestoneIndexEnd:          uint32(milestoneEnd),
			AdditionalInfo:             additionalInfo,
		},
	}
}

// ReferendumBuilder is used to easily build up a Referendum.
type ReferendumBuilder struct {
	r   *Referendum
	err error
}

// Payload sets the payload to embed within the message.
func (rb *ReferendumBuilder) Payload(seri serializer.Serializable) *ReferendumBuilder {
	if rb.err != nil {
		return rb
	}
	switch seri.(type) {
	case *Questions:
	case nil:
	default:
		rb.err = fmt.Errorf("%w: unsupported type %T", ErrUnknownPayloadType, seri)
		return rb
	}
	rb.r.Payload = seri
	return rb
}

// Build builds the Referendum.
func (rb *ReferendumBuilder) Build() (*Referendum, error) {
	if rb.err != nil {
		return nil, rb.err
	}

	if _, err := rb.r.Serialize(serializer.DeSeriModePerformValidation); err != nil {
		return nil, fmt.Errorf("unable to build referendum: %w", err)
	}
	return rb.r, nil
}
