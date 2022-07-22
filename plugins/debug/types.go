package debug

import (
	"github.com/iotaledger/hornet/v2/plugins/coreapi"
	iotago "github.com/iotaledger/iota.go/v3"
)

// outputIDsResponse defines the response of a GET debug outputs REST API call.
type outputIDsResponse struct {
	// The output IDs (transaction hash + output index) of the outputs.
	OutputIDs []string `json:"outputIds"`
}

// outputIDsResponse defines the response of a GET debug milestone diff REST API call.
type milestoneDiffResponse struct {
	// The index of the milestone.
	MilestoneIndex iotago.MilestoneIndex `json:"index"`
	// The newly created outputs by this milestone diff.
	Outputs []*coreapi.OutputResponse `json:"outputs"`
	// The used outputs (spents) by this milestone diff.
	Spents []*coreapi.OutputResponse `json:"spents"`
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
	MilestoneIndex iotago.MilestoneIndex `json:"milestoneIndex"`
}

// requestsResponse defines the response of a GET debug requests REST API call.
type requestsResponse struct {
	// The pending requests of the node.
	Requests []*request `json:"requests"`
}

// entryPoint defines an entryPoint with information about the milestone index of the cone it references.
type entryPoint struct {
	// The hex encoded block ID of the block.
	BlockID               string                `json:"blockId"`
	ReferencedByMilestone iotago.MilestoneIndex `json:"referencedByMilestone"`
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
