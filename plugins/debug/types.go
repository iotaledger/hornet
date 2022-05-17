package debug

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
	restapiv2 "github.com/gohornet/hornet/plugins/restapi/v2"
)

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
	Outputs []*restapiv2.OutputResponse `json:"outputs"`
	// The used outputs (spents) by this milestone diff.
	Spents []*restapiv2.OutputResponse `json:"spents"`
}

// request defines an request response.
type request struct {
	// The hex encoded block ID of the block.
	BlockID string `json:"blockId"`
	// The type of the request.
	Type string `json:"type"`
	// Whether the block already exists in the storage layer.
	BlockExists bool `json:"txExists"`
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
	// The hex encoded block ID of the block.
	BlockID               string          `json:"blockId"`
	ReferencedByMilestone milestone.Index `json:"referencedByMilestone"`
}

// blockWithParents defines a block with information about it's parents.
type blockWithParents struct {
	// The hex encoded block ID of the block.
	BlockID string `json:"blockId"`
	// The hex encoded block IDs of the parents the block references.
	Parents []string `json:"parents"`
}

// blockConeResponse defines the response of a GET debug block cone REST API call.
type blockConeResponse struct {
	// The count of elements in the cone.
	ConeElementsCount int `json:"coneElementsCount"`
	// The count of found entry points.
	EntryPointsCount int `json:"entryPointsCount"`
	// The cone of the block.
	Cone []*blockWithParents `json:"cone"`
	// The entry points of the cone of this block.
	EntryPoints []*entryPoint `json:"entryPoints"`
}
