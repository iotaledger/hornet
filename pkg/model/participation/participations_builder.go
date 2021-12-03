package participation

import (
	"fmt"

	"github.com/iotaledger/hive.go/serializer/v2"
)

// NewParticipationsBuilder creates a new ParticipationsBuilder.
func NewParticipationsBuilder() *ParticipationsBuilder {
	return &ParticipationsBuilder{
		p: &ParticipationPayload{
			Participations: Participations{},
		},
	}
}

// ParticipationsBuilder is used to easily build up ParticipationPayload.
type ParticipationsBuilder struct {
	p *ParticipationPayload
}

// AddParticipation adds the given participation to the participations.
func (b *ParticipationsBuilder) AddParticipation(entry *Participation) *ParticipationsBuilder {
	b.p.Participations = append(b.p.Participations, entry)
	return b
}

// Build builds the ParticipationPayload.
func (b *ParticipationsBuilder) Build() (*ParticipationPayload, error) {
	if _, err := b.p.Serialize(serializer.DeSeriModePerformValidation, nil); err != nil {
		return nil, fmt.Errorf("unable to build participations: %w", err)
	}
	return b.p, nil
}
