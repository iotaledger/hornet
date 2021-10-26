package referendum

import "github.com/gohornet/hornet/pkg/model/milestone"

// ReferendumsResponse defines the response of a GET RouteReferendums REST API call.
type ReferendumsResponse struct {
	// The hex encoded referendum IDs of the found referendums.
	ReferendumIDs []string `json:"referendumIds"`
}

// CreateReferendumResponse defines the response of a POST RouteReferendums REST API call.
type CreateReferendumResponse struct {
	// The hex encoded referendum ID of the created referendum.
	ReferendumID string `json:"referendumId"`
}

// AnswerStatus holds the current and accumulated vote for an answer.
type AnswerStatus struct {
	Current     uint64 `json:"current"`
	Accumulated uint64 `json:"accumulated"`
}

// QuestionStatus holds the answers for a question.
type QuestionStatus struct {
	Answers []AnswerStatus `json:"answers"`
}

// ReferendumStatusResponse defines the response of a GET RouteReferendumStatus REST API call.
type ReferendumStatusResponse struct {
	Questions []QuestionStatus `json:"questions"`
}

// OutputStatusResponse defines the response of a GET RouteOutputStatus REST API call.
type OutputStatusResponse struct {
	MessageID           string          `json:"messageId"`
	StartMilestoneIndex milestone.Index `json:"startMilestoneIndex"`
	EndMilestoneIndex   milestone.Index `json:"endMilestoneIndex"`
}
