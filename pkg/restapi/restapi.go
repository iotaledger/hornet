package restapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

var (
	// ErrInvalidParameter defines the invalid parameter error.
	ErrInvalidParameter = echo.NewHTTPError(http.StatusBadRequest, "invalid parameter")

	// ErrServiceNotImplemented defines the service not implemented error.
	ErrServiceNotImplemented = echo.NewHTTPError(http.StatusNotImplemented, "service not implemented")
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

type (
	// AllowedRoute defines a function to allow or disallow routes.
	AllowedRoute func(echo.Context) bool
)
