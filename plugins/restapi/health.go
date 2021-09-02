package restapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func setupHealthRoute() {
	deps.Echo.GET(nodeAPIHealthRoute, func(c echo.Context) error {

		// node mode
		if deps.Tangle != nil && !deps.Tangle.IsNodeHealthy() {
			return c.NoContent(http.StatusServiceUnavailable)
		}

		return c.NoContent(http.StatusOK)
	})
}
