package indexer

import "github.com/gohornet/hornet/pkg/model/milestone"

// outputsResponse defines the response of a GET outputs REST API call.
type outputsResponse struct {
	// The ledger index at which these outputs where available at.
	LedgerIndex milestone.Index `json:"ledgerIndex"`
	// The maximum count of results that are returned by the node.
	Limit uint32 `json:"limit,omitempty"`
	// The actual count of results that are returned.
	Count uint32 `json:"count"`
	// The cursor to use for getting the next results.
	Cursor string `json:"cursor,omitempty"`
	// The output IDs (transaction hash + output index) of the outputs on this address.
	OutputIDs []string `json:"data"`
}
