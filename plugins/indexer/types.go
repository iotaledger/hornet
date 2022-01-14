package indexer

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
)

// outputsResponse defines the response of a GET outputs REST API call.
type outputsResponse struct {
	// The maximum count of results that are returned by the node.
	MaxResults uint32 `json:"maxResults"`
	// The actual count of results that are returned.
	Count uint32 `json:"count"`
	// The output IDs (transaction hash + output index) of the outputs on this address.
	OutputIDs []string `json:"outputIds"`
	// The ledger index at which these outputs where available at.
	LedgerIndex milestone.Index `json:"ledgerIndex"`
}
