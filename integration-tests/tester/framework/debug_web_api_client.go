package framework

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
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

// NewDebugNodeAPI returns a new debug node API instance.
func NewDebugNodeAPI(baseURL string, httpClient ...http.Client) *DebugNodeAPI {
	if len(httpClient) > 0 {
		return &DebugNodeAPI{baseURL: baseURL, httpClient: httpClient[0]}
	}
	return &DebugNodeAPI{baseURL: baseURL}
}

// DebugNodeAPI is an API wrapper over the debug node API.
type DebugNodeAPI struct {
	httpClient http.Client
	baseURL    string
}

// HTTPErrorResponse defines the error struct for the HTTPErrorResponseEnvelope.
type HTTPErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HTTPErrorResponseEnvelope defines the error response schema for node API responses.
type HTTPErrorResponseEnvelope struct {
	Error HTTPErrorResponse `json:"error"`
}

// HTTPOkResponseEnvelope defines the ok response schema for node API responses.
type HTTPOkResponseEnvelope struct {
	// The response is encapsulated in the Data field.
	Data interface{} `json:"data"`
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

	errRes := &HTTPErrorResponseEnvelope{}
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

func (api *DebugNodeAPI) do(method string, reqObj interface{}, resObj interface{}) error {
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

// BaseURL returns the baseURL of the API.
func (api *DebugNodeAPI) BaseURL() string {
	return api.baseURL
}
