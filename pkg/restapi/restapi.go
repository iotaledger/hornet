package restapi

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

var (
	// ErrInvalidParameter defines the invalid parameter error.
	ErrInvalidParameter = errors.New("invalid parameter")

	// ErrInternalError defines the internal error.
	ErrInternalError = errors.New("internal error")

	// ErrNotFound defines the not found error.
	ErrNotFound = errors.New("not found")

	// ErrForbidden defines the forbidden error.
	ErrForbidden = errors.New("forbidden")

	// ErrServiceUnavailable defines the service unavailable error.
	ErrServiceUnavailable = errors.New("service unavailable")

	// ErrServiceNotImplemented defines the service not implemented error.
	ErrServiceNotImplemented = errors.New("service not implemented")
)

// JSONResponse wraps the result into a "data" field and sends the JSON response with status code.
func JSONResponse(c echo.Context, statusCode int, result interface{}) error {
	return c.JSON(statusCode, &HTTPOkResponseEnvelope{Data: result})
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
