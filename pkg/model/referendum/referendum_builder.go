package referendum

import (
	"fmt"

	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v2"
)

// NewReferendumBuilder creates a new ReferendumBuilder.
func NewReferendumBuilder(name string, milestoneStart milestone.Index, milestoneStartHolding milestone.Index, milestoneEnd milestone.Index, additionalInfo string) *ReferendumBuilder {
	return &ReferendumBuilder{
		r: &Referendum{
			Name:                  name,
			MilestoneStart:        milestoneStart,
			MilestoneStartHolding: milestoneStartHolding,
			MilestoneEnd:          milestoneEnd,
			Questions:             iotago.Serializables{},
			AdditionalInfo:        additionalInfo,
		},
	}
}

// ReferendumBuilder is used to easily build up a Referendum.
type ReferendumBuilder struct {
	r *Referendum
}

// AddQuestion adds the given question to the referendum.
func (rb *ReferendumBuilder) AddQuestion(entry *Question) *ReferendumBuilder {
	rb.r.Questions = append(rb.r.Questions, entry)
	return rb
}

// Build builds the Referendum.
func (rb *ReferendumBuilder) Build() (*Referendum, error) {
	if _, err := rb.r.Serialize(iotago.DeSeriModePerformValidation); err != nil {
		return nil, fmt.Errorf("unable to build referendum: %w", err)
	}
	return rb.r, nil
}
