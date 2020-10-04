package common

// HTTPErrorResponse defines the error struct for the HTTPErrorResponseEnvelope.
type HTTPErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// HTTPErrorResponseEnvelope defines the error response schema for node API responses.
type HTTPErrorResponseEnvelope struct {
	Error HTTPErrorResponse `json:"error"`
}

// HTTPOkResponseEnvelope defines the ok response schema for node API responses.
type HTTPOkResponseEnvelope struct {
	Data interface{} `json:"data"`
}
