package coordinator

import (
	"encoding/hex"
	"fmt"
	"net/http"

	iotago "github.com/iotaledger/iota.go/v2"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

const (
	// NodeAPIRouteDebugComputeWhiteFlag is the debug route to compute the white flag confirmation for the cone of the given parents.
	// POST computes the white flag confirmation.
	NodeAPIRouteDebugComputeWhiteFlag = "/api/plugins/debug/whiteflag"
)

// NewDebugNodeAPIClient returns a new DebugNodeAPIClient with the given BaseURL.
func NewDebugNodeAPIClient(baseURL string, opts ...iotago.NodeHTTPAPIClientOption) *DebugNodeAPIClient {
	return &DebugNodeAPIClient{NodeHTTPAPIClient: iotago.NewNodeHTTPAPIClient(baseURL, opts...)}
}

// DebugNodeAPIClient is a client for node HTTP REST APIs.
type DebugNodeAPIClient struct {
	*iotago.NodeHTTPAPIClient
}

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

// Whiteflag is the debug route to compute the white flag confirmation for the cone of the given parents.
// This function returns the merkle tree hash calculated by the node.
func (api *DebugNodeAPIClient) Whiteflag(index milestone.Index, parents hornet.MessageIDs) (*MerkleTreeHash, error) {

	req := &computeWhiteFlagMutationsRequest{
		Index:   index,
		Parents: parents.ToHex(),
	}
	res := &computeWhiteFlagMutationsResponse{}

	if _, err := api.Do(http.MethodPost, NodeAPIRouteDebugComputeWhiteFlag, req, res); err != nil {
		return nil, err
	}

	merkleTreeHashBytes, err := hex.DecodeString(res.MerkleTreeHash)
	if err != nil {
		return nil, err
	}

	if len(merkleTreeHashBytes) != iotago.MilestoneInclusionMerkleProofLength {
		return nil, fmt.Errorf("unknown merkle tree hash length (%d)", len(merkleTreeHashBytes))
	}

	var merkleTreeHash MerkleTreeHash
	copy(merkleTreeHash[:], merkleTreeHashBytes)
	return &merkleTreeHash, nil
}
