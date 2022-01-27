package framework

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/nodeclient"
)

const (
	IndexerAPIRouteOutputs = "/api/plugins/indexer/v1/outputs"
)

// NewDebugNodeAPIClient returns a new debug node API instance.
func NewDebugNodeAPIClient(baseURL string, deSeriParas *iotago.DeSerializationParameters, opts ...nodeclient.NodeHTTPAPIClientOption) *DebugNodeAPIClient {
	return &DebugNodeAPIClient{NodeHTTPAPIClient: nodeclient.NewNodeHTTPAPIClient(baseURL, deSeriParas, opts...)}
}

// DebugNodeAPIClient is an API wrapper over the debug node API.
type DebugNodeAPIClient struct {
	*nodeclient.NodeHTTPAPIClient
}

// BaseURL returns the baseURL of the API.
func (api *DebugNodeAPIClient) BaseURL() string {
	return api.NodeHTTPAPIClient.BaseURL
}

// Add debug API endpoints here

//TODO: remove al this once iota.go/v3 supports the Indexer

var (
	httpCodeToErr = map[int]error{
		http.StatusBadRequest:          nodeclient.ErrHTTPBadRequest,
		http.StatusInternalServerError: nodeclient.ErrHTTPInternalServerError,
		http.StatusNotFound:            nodeclient.ErrHTTPNotFound,
		http.StatusUnauthorized:        nodeclient.ErrHTTPUnauthorized,
		http.StatusNotImplemented:      nodeclient.ErrHTTPNotImplemented,
	}
)

// OutputsByAddress returns the outputs for a given address.
func (api *DebugNodeAPIClient) OutputsByAddress(ctx context.Context, addr iotago.Address) (*OutputsResponse, error) {
	query := fmt.Sprintf("%s?address=%s", IndexerAPIRouteOutputs, addr.Bech32(iotago.PrefixTestnet))

	var data []byte
	// construct request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s%s", api.BaseURL(), query), func() io.Reader {
		if data == nil {
			return nil
		}
		return bytes.NewReader(data)
	}())
	if err != nil {
		return nil, fmt.Errorf("unable to build http request: %w", err)
	}

	// make the request
	httpRes, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	res := &OutputsResponse{}
	// write response into response object
	if err := interpretBody(httpRes, res); err != nil {
		return nil, err
	}
	return res, nil

}

func (api *DebugNodeAPIClient) BalanceByAddress(ctx context.Context, addr iotago.Address) (uint64, error) {
	var balance uint64

	response, err := api.OutputsByAddress(ctx, addr)
	if err != nil {
		return 0, err
	}

	for _, outputIDHex := range response.OutputIDs {
		outputID, err := iotago.OutputIDFromHex(outputIDHex)
		if err != nil {
			return 0, err
		}
		resp, err := api.OutputByID(ctx, outputID)
		if err != nil {
			return 0, err
		}
		output, err := resp.Output()
		if err != nil {
			return 0, err
		}
		balance += output.Deposit()
	}

	return balance, nil
}

// OutputsResponse defines the response of a GET outputs REST API call.
type OutputsResponse struct {
	// The ledger index at which these outputs where available at.
	LedgerIndex milestone.Index `json:"ledgerIndex"`
	// The maximum count of results that are returned by the node.
	Limit uint32 `json:"limit,omitempty"`
	// The offset to use for getting the next results.
	Offset string `json:"offset,omitempty"`
	// The actual count of results that are returned.
	Count uint32 `json:"count"`
	// The output IDs (transaction hash + output index) of the outputs on this address.
	OutputIDs []string `json:"data"`
}

func readBody(res *http.Response) ([]byte, error) {
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %w", err)
	}
	return resBody, nil
}

func interpretBody(res *http.Response, decodeTo interface{}) error {
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusCreated {
		if decodeTo == nil {
			return nil
		}

		resBody, err := readBody(res)
		if err != nil {
			return err
		}

		return json.Unmarshal(resBody, decodeTo)
	}

	if res.StatusCode == http.StatusServiceUnavailable {
		return nil
	}

	resBody, err := readBody(res)
	if err != nil {
		return err
	}

	errRes := &nodeclient.HTTPErrorResponseEnvelope{}
	if err := json.Unmarshal(resBody, errRes); err != nil {
		return fmt.Errorf("unable to read error from response body: %w", err)
	}

	err, ok := httpCodeToErr[res.StatusCode]
	if !ok {
		err = nodeclient.ErrHTTPUnknownError
	}

	return fmt.Errorf("%w: url %s, error message: %s", err, res.Request.URL.String(), errRes.Error.Message)
}
