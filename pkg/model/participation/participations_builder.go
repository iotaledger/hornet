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

// AddParticipation adds the given participation to the participations.
func (b *ParticipationsBuilder) AddParticipation(entry *Participation) *ParticipationsBuilder {
	b.p.Participations = append(b.p.Participations, entry)
	return b
}

// Build builds the Participations.
func (b *ParticipationsBuilder) Build() (*Participations, error) {
	if _, err := b.p.Serialize(serializer.DeSeriModePerformValidation); err != nil {
		return nil, fmt.Errorf("unable to build participations: %w", err)
	}
	return b.p, nil
}
