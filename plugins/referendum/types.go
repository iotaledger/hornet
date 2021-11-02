package referendum

import "github.com/gohornet/hornet/pkg/model/milestone"

// ReferendumsResponse defines the response of a GET RouteReferendums REST API call.
type ReferendumsResponse struct {
	// The hex encoded partitipation IDs of the found referendums.
	ReferendumIDs []string `json:"referendumIds"`
}

// CreateReferendumResponse defines the response of a POST RouteReferendums REST API call.
type CreateReferendumResponse struct {
	// The hex encoded partitipation ID of the created partitipation.
	ReferendumID string `json:"referendumId"`
}

// TrackedVote holds the information for each tracked vote.
type TrackedVote struct {
	MessageID           string          `json:"messageId"`
	Amount              uint64          `json:"amount"`
	StartMilestoneIndex milestone.Index `json:"startMilestoneIndex"`
	EndMilestoneIndex   milestone.Index `json:"endMilestoneIndex"`
}

// OutputStatusResponse defines the response of a GET RouteOutputStatus REST API call.
type OutputStatusResponse struct {
	ReferendumVotes map[string]*TrackedVote `json:"referendumVotes"`
}
