package v1

import (
	"encoding/json"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

// okResponseEnvelope defines the ok response schema for node API responses.
type okResponseEnvelope struct {
	// The response is encapsulated in the Data field.
	Data interface{} `json:"data"`
}

// infoResponse defines the response of a GET info REST API call.
type infoResponse struct {
	// The name of the node software.
	Name string `json:"name"`
	// The semver version of the node software.
	Version string `json:"version"`
	// Whether the node is healthy.
	IsHealthy bool `json:"isHealthy"`
	// The hex encoded public key of the coordinator.
	CoordinatorPublicKey string `json:"coordinatorPublicKey"`
	// The hex encoded message ID of the latest known milestone.
	LatestMilestoneMessageID string `json:"latestMilestoneMessageId"`
	// The latest known milestone index.
	LatestMilestoneIndex milestone.Index `json:"latestMilestoneIndex"`
	// The hex encoded message ID of the current solid milestone.
	SolidMilestoneMessageID string `json:"solidMilestoneMessageId"`
	// The current solid milestone's index.
	SolidMilestoneIndex milestone.Index `json:"solidMilestoneIndex"`
	// The milestone index at which the last pruning commenced.
	PruningIndex milestone.Index `json:"pruningIndex"`
	// The features this node exposes.
	Features []string `json:"features"`
}

// tipsResponse defines the response of a GET tips REST API call.
type tipsResponse struct {
	// The hex encoded message ID of the 1st tip.
	Tip1 string `json:"tip1MessageId"`
	// The hex encoded message ID of the 2nd tip.
	Tip2 string `json:"tip2MessageId"`
}

// messageMetadataResponse defines the response of a GET message metadata REST API call.
type messageMetadataResponse struct {
	// The hex encoded message ID of the message.
	MessageID string `json:"messageId"`
	// The hex encoded message ID of the 1st parent the message references.
	Parent1 string `json:"parent1MessageId"`
	// The hex encoded message ID of the 2nd parent the message references.
	Parent2 string `json:"parent2MessageId"`
	// Whether the message is solid.
	Solid bool `json:"isSolid"`
	// The milestone index that references this message.
	ReferencedByMilestone *milestone.Index `json:"referencedByMilestoneIndex,omitempty"`
	// The ledger inclusion state of the transaction payload.
	LedgerInclusionState *string `json:"ledgerInclusionState,omitempty"`
	// Whether the message should be promoted.
	ShouldPromote *bool `json:"shouldPromote,omitempty"`
	// Whether the message should be reattached.
	ShouldReattach *bool `json:"shouldReattach,omitempty"`
}

// messageCreatedResponse defines the response of a POST messages REST API call.
type messageCreatedResponse struct {
	// The hex encoded message ID of the message.
	MessageID string `json:"messageId"`
}

// childrenResponse defines the response of a GET children REST API call.
type childrenResponse struct {
	// The hex encoded message ID of the message.
	MessageID string `json:"messageId"`
	// The maximum count of results that are returned by the node.
	MaxResults uint32 `json:"maxResults"`
	// The actual count of results that are returned.
	Count uint32 `json:"count"`
	// The hex encoded message IDs of the children of this message.
	Children []string `json:"childrenMessageIds"`
}

// messageIDsByIndexResponse defines the response of a GET messages REST API call.
type messageIDsByIndexResponse struct {
	// The index of the messages.
	Index string `json:"index"`
	// The maximum count of results that are returned by the node.
	MaxResults uint32 `json:"maxResults"`
	// The actual count of results that are returned.
	Count uint32 `json:"count"`
	// The hex encoded message IDs of the found messages with this index.
	MessageIDs []string `json:"messageIds"`
}

// milestoneResponse defines the response of a GET milestones REST API call.
type milestoneResponse struct {
	// The index of the milestone.
	Index uint32 `json:"milestoneIndex"`
	// The hex encoded message ID of the message.
	MessageID string `json:"messageId"`
	// The unix time of the milestone payload.
	Time int64 `json:"timestamp"`
}

// outputResponse defines the response of a GET outputs REST API call.
type outputResponse struct {
	// The hex encoded message ID of the message.
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

// addressBalanceResponse defines the response of a GET addresses REST API call.
type addressBalanceResponse struct {
	// The hex encoded address.
	Address string `json:"address"`
	// The maximum count of results that are returned by the node.
	MaxResults uint32 `json:"maxResults"`
	// The actual count of results that are returned.
	Count uint32 `json:"count"`
	// The balance of the address.
	Balance uint64 `json:"balance"`
}

// outputResponse defines an output response.
type addressOutputsResponse struct {
	// The hex encoded address.
	Address string `json:"address"`
	// The maximum count of results that are returned by the node.
	MaxResults uint32 `json:"maxResults"`
	// The actual count of results that are returned.
	Count uint32 `json:"count"`
	// The output IDs (transaction hash + output index) of the outputs on this address.
	OutputIDs []string `json:"outputIds"`
}

// outputIDsResponse defines the response of a GET debug outputs REST API call.
type outputIDsResponse struct {
	// The output IDs (transaction hash + output index) of the outputs.
	OutputIDs []string `json:"outputIds"`
}

// outputIDsResponse defines the response of a GET debug milestone diff REST API call.
type milestoneDiffResponse struct {
	// The index of the milestone.
	MilestoneIndex milestone.Index `json:"milestoneIndex"`
	// The newly created outputs by this milestone diff.
	Outputs []*outputResponse `json:"outputs"`
	// The used outputs (spents) by this milestone diff.
	Spents []*outputResponse `json:"spents"`
}

// request defines an request response.
type request struct {
	// The hex encoded message ID of the message.
	MessageID string `json:"messageId"`
	// The type of the request.
	Type string `json:"type"`
	// Whether the message already exists in the storage layer.
	MessageExists bool `json:"txExists"`
	// The time the request was enqueued.
	EnqueueTimestamp string `json:"enqueueTimestamp"`
	// The index of the milestone this request belongs to.
	MilestoneIndex milestone.Index `json:"milestoneIndex"`
}

// requestsResponse defines the response of a GET debug requests REST API call.
type requestsResponse struct {
	// The pending requests of the node.
	Requests []*request `json:"requests"`
}

// entryPoint defines an entryPoint with information about the milestone index of the cone it references.
type entryPoint struct {
	// The hex encoded message ID of the message.
	MessageID             string          `json:"messageId"`
	ReferencedByMilestone milestone.Index `json:"referencedByMilestone"`
}

// messageWithParents defines a message with information about it's parents.
type messageWithParents struct {
	// The hex encoded message ID of the message.
	MessageID string `json:"messageId"`
	// The hex encoded message ID of the 1st parent the message references.
	Parent1 string `json:"parent1MessageId"`
	// The hex encoded message ID of the 2nd parent the message references.
	Parent2 string `json:"parent2MessageId"`
}

// messageConeResponse defines the response of a GET debug message cone REST API call.
type messageConeResponse struct {
	// The count of elements in the cone.
	ConeElementsCount int `json:"coneElementsCount"`
	// The count of found entry points.
	EntryPointsCount int `json:"entryPointsCount"`
	// The cone of the message.
	Cone []*messageWithParents `json:"cone"`
	// The entry points of the cone of this message.
	EntryPoints []*entryPoint `json:"entryPoints"`
}

