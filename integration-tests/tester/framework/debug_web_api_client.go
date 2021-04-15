package framework

import (
	iotago "github.com/iotaledger/iota.go/v2"
)

// NewDebugNodeAPIClient returns a new debug node API instance.
func NewDebugNodeAPIClient(baseURL string, opts ...iotago.NodeHTTPAPIClientOption) *DebugNodeAPIClient {
	return &DebugNodeAPIClient{NodeHTTPAPIClient: iotago.NewNodeHTTPAPIClient(baseURL, opts...)}
}

// DebugNodeAPIClient is an API wrapper over the debug node API.
type DebugNodeAPIClient struct {
	*iotago.NodeHTTPAPIClient
}

// BaseURL returns the baseURL of the API.
func (api *DebugNodeAPIClient) BaseURL() string {
	return api.NodeHTTPAPIClient.BaseURL
}

// Add debug API endpoints here
