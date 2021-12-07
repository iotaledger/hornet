package framework

import (
	iotago "github.com/iotaledger/iota.go/v3"
)

// NewDebugNodeAPIClient returns a new debug node API instance.
func NewDebugNodeAPIClient(baseURL string, deSeriParas *iotago.DeSerializationParameters, opts ...iotago.NodeHTTPAPIClientOption) *DebugNodeAPIClient {
	return &DebugNodeAPIClient{NodeHTTPAPIClient: iotago.NewNodeHTTPAPIClient(baseURL, deSeriParas, opts...)}
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
