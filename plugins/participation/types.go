package participation

import "github.com/gohornet/hornet/pkg/model/milestone"

// ParticipationEventsResponse defines the response of a GET RouteReferendums REST API call.
type ParticipationEventsResponse struct {
	// The hex encoded partitipation IDs of the found referendums.
	ParticipationEventIDs []string `json:"participationEventIds"`
}

// CreateReferendumResponse defines the response of a POST RouteReferendums REST API call.
type CreateReferendumResponse struct {
	// The hex encoded ID of the created partitipation event.
	ParticipationEventID string `json:"participationEventId"`
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
