package debug

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
	v1 "github.com/gohornet/hornet/plugins/restapi/v1"
)

// computeWhiteFlagMutationsRequest defines the request for a POST debugComputeWhiteFlagMutations REST API call.
type computeWhiteFlagMutationsRequest struct {
	// The index of the milestone.
	Index milestone.Index `json:"index"`
	// The hex encoded message IDs of the parents the milestone references.
	Parents []string `json:"parentMessageIds"`
}

// computeWhiteFlagMutationsRequest defines the response for a POST debugComputeWhiteFlagMutations REST API call.
type computeWhiteFlagMutationsResponse struct {
	// The hex encoded merkle tree hash as a result of the white flag computation.
	MerkleTreeHash string `json:"merkleTreeHash"`
}

// controlFreeMemoryRequest defines the request for a PUT controlFreeMemory REST API call.
// If no flags are given, all memories are cleared.
type controlFreeMemoryRequest struct {
	// Free the unused memory of the request queue.
	RequestQueue     *bool `json:"requestQueue,omitempty"`
	// Free the unused memory of the message processor.
	MessageProcessor *bool `json:"messageProcessor,omitempty"`
	// Free the unused memory of the object storage.
	Storage          *bool `json:"storage,omitempty"`
}

// outputIDsResponse defines the response of a GET debug outputs REST API call.
type outputIDsResponse struct {
	// The output IDs (transaction hash + output index) of the outputs.
	OutputIDs []string `json:"outputIds"`
}

// address defines the response of a GET debug addresses REST API call.
type address struct {
	// The type of the address (0=Ed25519).
	AddressType byte `json:"addressType"`
	// The hex encoded address.
	Address string `json:"address"`
	// The balance of the address.
	Balance uint64 `json:"balance"`
}

// addressesResponse defines the response of a GET debug addresses REST API call.
type addressesResponse struct {
	// The addresses (type + hex encoded address).
	Addresses []*address `json:"addresses"`
}

// outputIDsResponse defines the response of a GET debug milestone diff REST API call.
type milestoneDiffResponse struct {
	// The index of the milestone.
	MilestoneIndex milestone.Index `json:"index"`
	// The newly created outputs by this milestone diff.
	Outputs []*v1.OutputResponse `json:"outputs"`
	// The used outputs (spents) by this milestone diff.
	Spents []*v1.OutputResponse `json:"spents"`
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
	// The hex encoded message IDs of the parents the message references.
	Parents []string `json:"parentMessageIds"`
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
