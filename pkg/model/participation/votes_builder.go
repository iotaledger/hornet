package participation

import (
	"fmt"

	"github.com/iotaledger/hive.go/serializer"
)

// NewVotesBuilder creates a new VotesBuilder.
func NewVotesBuilder() *VotesBuilder {
	return &VotesBuilder{
		v: &Participations{
			Participations: serializer.Serializables{},
		},
	}
}

// VotesBuilder is used to easily build up Participations.
type VotesBuilder struct {
	v *Participations
}

// AddVote adds the given vote to the votes.
func (vb *VotesBuilder) AddVote(entry *Participation) *VotesBuilder {
	vb.v.Participations = append(vb.v.Participations, entry)
	return vb
}

// Build builds the Participations.
func (vb *VotesBuilder) Build() (*Participations, error) {
	if _, err := vb.v.Serialize(serializer.DeSeriModePerformValidation); err != nil {
		return nil, fmt.Errorf("unable to build question: %w", err)
	}
	return vb.v, nil
}
