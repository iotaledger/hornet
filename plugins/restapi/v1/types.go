package v1

import (
	"encoding/json"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

// okResponseEnvelope defines the ok response schema for node API responses.
type okResponseEnvelope struct {
	Data interface{} `json:"data"`
}

// infoResponse defines the response of a node info REST API call.
type infoResponse struct {
	// The name of the node software.
	Name string `json:"name"`
	// The semver version of the node software.
	Version string `json:"version"`
	// Whether the node is healthy.
	IsHealthy bool `json:"isHealthy"`
	// The used coordinator public key.
	CoordinatorPublicKey string `json:"coordinatorPublicKey"`
	// The latest known milestone message ID.
	LatestMilestoneMessageID string `json:"latestMilestoneMessageId"`
	// The latest known milestone index.
	LatestMilestoneIndex milestone.Index `json:"latestMilestoneIndex"`
	// The current solid milestone's message ID.
	LatestSolidMilestoneMessageID string `json:"solidMilestoneMessageId"`
	// The current solid milestone's index.
	LatestSolidMilestoneIndex milestone.Index `json:"solidMilestoneIndex"`
	// The milestone index at which the last pruning commenced.
	PruningIndex milestone.Index `json:"pruningIndex"`
	// The features this node exposes.
	Features []string `json:"features"`
}

// tipsResponse defines the response of a tips REST API call.
type tipsResponse struct {
	// The hex encoded hash of the first tip message.
	Tip1 string `json:"tip1MessageId"`
	// The hex encoded hash of the second tip message.
	Tip2 string `json:"tip2MessageId"`
}

type messageMetadataResponse struct {
	MessageID string `json:"messageId"`
	// The 1st parent the message references.
	Parent1 string `json:"parent1MessageId"`
	// The 2nd parent the message references.
	Parent2               string           `json:"parent2MessageId"`
	Solid                 bool             `json:"isSolid"`
	ReferencedByMilestone *milestone.Index `json:"referencedByMilestoneIndex,omitempty"`
	LedgerInclusionState  *string          `json:"ledgerInclusionState,omitempty"`
	ShouldPromote         *bool            `json:"shouldPromote,omitempty"`
	ShouldReattach        *bool            `json:"shouldReattach,omitempty"`
}

type messageCreatedResponse struct {
	MessageID string `json:"messageId"`
}

type childrenResponse struct {
	MessageID  string   `json:"messageId"`
	MaxResults uint32   `json:"maxResults"`
	Count      uint32   `json:"count"`
	Children   []string `json:"childrenMessageIds"`
}

type messageIDsByIndexResponse struct {
	Index      string   `json:"index"`
	MaxResults uint32   `json:"maxResults"`
	Count      uint32   `json:"count"`
	MessageIDs []string `json:"messageIds"`
}

type milestoneResponse struct {
	Index     uint32 `json:"milestoneIndex"`
	MessageID string `json:"messageId"`
	Time      int64  `json:"timestamp"`
}

type outputResponse struct {
	MessageID string `json:"messageId"`
	// The hex encoded transaction id from which this output originated.
	TransactionID string `json:"transactionId"`
	// The index of the output.
	OutputIndex uint16 `json:"outputIndex"`
	// Whether this output is spent.
	Spent bool `json:"isSpent"`
	// The output in its serialized form.
	RawOutput *json.RawMessage `json:"output"`
}

type addressOutputsResponse struct {
	Address    string   `json:"address"`
	MaxResults uint32   `json:"maxResults"`
	Count      uint32   `json:"count"`
	OutputIDs  []string `json:"outputIDs"`
}

type addressBalanceResponse struct {
	Address    string `json:"address"`
	MaxResults uint32 `json:"maxResults"`
	Count      uint32 `json:"count"`
	Balance    uint64 `json:"balance"`
}

type outputIDsResponse struct {
	OutputIDs []string `json:"outputIDs"`
}

type milestoneDiffResponse struct {
	MilestoneIndex milestone.Index   `json:"milestoneIndex"`
	Outputs        []*outputResponse `json:"outputs"`
	Spents         []*outputResponse `json:"spents"`
}

type request struct {
	MessageID        string          `json:"messageId"`
	Type             string          `json:"type"`
	MessageExists    bool            `json:"txExists"`
	EnqueueTimestamp string          `json:"enqueueTimestamp"`
	MilestoneIndex   milestone.Index `json:"milestoneIndex"`
}

type requestsResponse struct {
	Requests []*request `json:"requests"`
}

type entryPoint struct {
	MessageID             string          `json:"messageId"`
	ReferencedByMilestone milestone.Index `json:"referencedByMilestone"`
}

type messageWithParents struct {
	MessageID string `json:"messageId"`
	// The 1st parent the message references.
	Parent1 string `json:"parent1MessageId"`
	// The 2nd parent the message references.
	Parent2 string `json:"parent2MessageId"`
}

type messageConeResponse struct {
	PathLength       int                   `json:"pathLength"`
	EntryPointsCount int                   `json:"entryPointsCount"`
	Path             []*messageWithParents `json:"path"`
	EntryPoints      []*entryPoint         `json:"entryPoints"`
}

