package framework

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/plugins/webapi"
)

var (
	// ErrBadRequest defines the "bad request" error.
	ErrBadRequest = errors.New("bad request")
	// ErrInternalServerError defines the "internal server error" error.
	ErrInternalServerError = errors.New("internal server error")
	// ErrNotFound defines the "not found" error.
	ErrNotFound = errors.New("not found")
	// ErrUnauthorized defines the "unauthorized" error.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrUnknownError defines the "unknown error" error.
	ErrUnknownError = errors.New("unknown error")
	// ErrNotImplemented defines the "operation not implemented/supported/available" error.
	ErrNotImplemented = errors.New("operation not implemented/supported/available")
)

const (
	contentTypeJSON = "application/json"
)

// NewWebAPI returns a new web API instance.
func NewWebAPI(baseURL string, httpClient ...http.Client) *WebAPI {
	if len(httpClient) > 0 {
		return &WebAPI{baseURL: baseURL, httpClient: httpClient[0]}
	}
	return &WebAPI{baseURL: baseURL}
}

// WebAPI is an API wrapper over the web API.
type WebAPI struct {
	httpClient http.Client
	baseURL    string
}

type errorresponse struct {
	Error string `json:"error"`
}

func interpretBody(res *http.Response, decodeTo interface{}) error {
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("unable to read response body: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusCreated {
		return json.Unmarshal(resBody, decodeTo)
	}

	errRes := &errorresponse{}
	if err := json.Unmarshal(resBody, errRes); err != nil {
		return fmt.Errorf("unable to read error from response body: %w", err)
	}

	switch res.StatusCode {
	case http.StatusInternalServerError:
		return fmt.Errorf("%w: %s", ErrInternalServerError, errRes.Error)
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, res.Request.URL.String())
	case http.StatusBadRequest:
		return fmt.Errorf("%w: %s", ErrBadRequest, errRes.Error)
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: %s", ErrUnauthorized, errRes.Error)
	case http.StatusNotImplemented:
		return fmt.Errorf("%w: %s", ErrNotImplemented, errRes.Error)
	}

	return fmt.Errorf("%w: %s", ErrUnknownError, errRes.Error)
}

func (api *WebAPI) do(method string, reqObj interface{}, resObj interface{}) error {
	// marshal request object
	var data []byte
	if reqObj != nil {
		var err error
		data, err = json.Marshal(reqObj)
		if err != nil {
			return err
		}
	}

	// construct request
	req, err := http.NewRequest(method, api.baseURL, func() io.Reader {
		if data == nil {
			return nil
		}
		return bytes.NewReader(data)
	}())
	if err != nil {
		return err
	}

	if data != nil {
		req.Header.Set("Content-Type", contentTypeJSON)
	}

	// make the request
	res, err := api.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resObj == nil {
		return nil
	}

	// write response into response object
	if err := interpretBody(res, resObj); err != nil {
		return err
	}
	return nil
}

// Neighbors returns the neighbors to which the node is connected to.
func (api *WebAPI) Neighbors() ([]*peer.Info, error) {
	res := &webapi.GetNeighborsReturn{}
	if err := api.do(http.MethodPost, struct {
		Command string `json:"command"`
	}{Command: "getNeighbors"}, res); err != nil {
		return nil, err
	}
	return res.Neighbors, nil
}

func (api *WebAPI) Info() (*webapi.GetNodeInfoReturn, error) {
	res := &webapi.GetNodeInfoReturn{}
	if err := api.do(http.MethodPost, struct {
		Command string `json:"command"`
	}{Command: "getNodeInfo"}, res); err != nil {
		return nil, err
	}
	return res, nil

}

// BaseURL returns the baseURL of the API.
func (api *WebAPI) BaseURL() string {
	return api.baseURL
}
