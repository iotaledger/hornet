package common

import (
	"github.com/labstack/echo/v4"
)

// JSONResponse wraps the result into a "data" field and sends the JSON response with status code.
func JSONResponse(c echo.Context, statusCode int, result interface{}) error {
	return c.JSON(statusCode, &HTTPOkResponseEnvelope{Data: result})
}
