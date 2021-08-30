package faucet

// faucetEnqueueRequest defines the request for a POST RouteFaucetEnqueue REST API call.
type faucetEnqueueRequest struct {
	// The bech32 address.
	Address string `json:"address"`
}
