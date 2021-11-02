package participation

import "github.com/gohornet/hornet/pkg/model/milestone"

// ParticipationEventsResponse defines the response of a GET RouteParticipationEvents REST API call.
type ParticipationEventsResponse struct {
	// The hex encoded participation IDs of the found referendums.
	ParticipationEventIDs []string `json:"participationEventIds"`
}

// CreateParticipationEventResponse defines the response of a POST RouteParticipationEvents REST API call.
type CreateParticipationEventResponse struct {
	// The hex encoded ID of the created participation event.
	ParticipationEventID string `json:"participationEventId"`
}

// TrackedParticipation holds the information for each tracked participation.
type TrackedParticipation struct {
	MessageID           string          `json:"messageId"`
	Amount              uint64          `json:"amount"`
	StartMilestoneIndex milestone.Index `json:"startMilestoneIndex"`
	EndMilestoneIndex   milestone.Index `json:"endMilestoneIndex"`
}

// OutputStatusResponse defines the response of a GET RouteOutputStatus REST API call.
type OutputStatusResponse struct {
	Participations map[string]*TrackedParticipation `json:"participations"`
}
