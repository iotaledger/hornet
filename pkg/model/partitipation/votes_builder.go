package partitipation

import (
	"fmt"

	"github.com/iotaledger/hive.go/serializer"
)

// NewVotesBuilder creates a new VotesBuilder.
func NewVotesBuilder() *VotesBuilder {
	return &VotesBuilder{
		v: &Votes{
			Votes: serializer.Serializables{},
		},
	}
}

// VotesBuilder is used to easily build up Votes.
type VotesBuilder struct {
	v *Votes
}

// AddVote adds the given vote to the votes.
func (vb *VotesBuilder) AddVote(entry *Vote) *VotesBuilder {
	vb.v.Votes = append(vb.v.Votes, entry)
	return vb
}

// Build builds the Votes.
func (vb *VotesBuilder) Build() (*Votes, error) {
	if _, err := vb.v.Serialize(serializer.DeSeriModePerformValidation); err != nil {
		return nil, fmt.Errorf("unable to build question: %w", err)
	}
	return vb.v, nil
}
