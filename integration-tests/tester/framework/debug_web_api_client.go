package framework

import (
	iotago "github.com/iotaledger/iota.go/v2"
)

// NewDebugNodeAPIClient returns a new debug node API instance.
func NewDebugNodeAPIClient(baseURL string, opts ...iotago.NodeAPIClientOption) *DebugNodeAPIClient {
	return &DebugNodeAPIClient{NodeAPIClient: iotago.NewNodeAPIClient(baseURL, opts...)}
}

// DebugNodeAPIClient is an API wrapper over the debug node API.
type DebugNodeAPIClient struct {
	*iotago.NodeAPIClient
}

// BaseURL returns the baseURL of the API.
func (api *DebugNodeAPIClient) BaseURL() string {
	return api.NodeAPIClient.BaseURL
}

// Add debug API endpoints here
