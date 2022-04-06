package framework

import (
	"context"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/nodeclient"
)

// NewDebugNodeAPIClient returns a new debug node API instance.
func NewDebugNodeAPIClient(baseURL string, opts ...nodeclient.ClientOption) *DebugNodeAPIClient {
	return &DebugNodeAPIClient{Client: nodeclient.New(baseURL, opts...)}
}

// DebugNodeAPIClient is an API wrapper over the debug node API.
type DebugNodeAPIClient struct {
	*nodeclient.Client
}

// BaseURL returns the baseURL of the API.
func (api *DebugNodeAPIClient) BaseURL() string {
	return api.Client.BaseURL
}

// Add debug API endpoints here

func (api *DebugNodeAPIClient) BalanceByAddress(ctx context.Context, addr iotago.Address) (uint64, error) {
	var balance uint64

	indexer, err := api.Indexer(ctx)
	if err != nil {
		return 0, err
	}

	result, err := indexer.Outputs(ctx, &nodeclient.BasicOutputsQuery{AddressBech32: addr.Bech32(iotago.PrefixTestnet)})
	if err != nil {
		return 0, err
	}

	for result.Next() {
		outputs, err := result.Outputs()
		if err != nil {
			return 0, err
		}

		for _, output := range outputs {
			balance += output.Deposit()
		}
	}
	if result.Error != nil {
		return 0, result.Error
	}

	return balance, nil
}
