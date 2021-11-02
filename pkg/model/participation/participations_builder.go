package participation

import (
	"fmt"

	"github.com/iotaledger/hive.go/serializer"
)

// NewParticipationsBuilder creates a new ParticipationsBuilder.
func NewParticipationsBuilder() *ParticipationsBuilder {
	return &ParticipationsBuilder{
		p: &Participations{
			Participations: serializer.Serializables{},
		},
	}
}

// ParticipationsBuilder is used to easily build up Participations.
type ParticipationsBuilder struct {
	p *Participations
}

// AddVote adds the given vote to the votes.
func (vb *ParticipationsBuilder) AddVote(entry *Participation) *ParticipationsBuilder {
	vb.p.Participations = append(vb.p.Participations, entry)
	return vb
}

// Build builds the Participations.
func (vb *ParticipationsBuilder) Build() (*Participations, error) {
	if _, err := vb.p.Serialize(serializer.DeSeriModePerformValidation); err != nil {
		return nil, fmt.Errorf("unable to build question: %w", err)
	}
	return vb.p, nil
}
