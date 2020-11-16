package restapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func setupHealthRoute() {
	deps.Echo.GET(nodeAPIHealthRoute, func(c echo.Context) error {

		// node mode
		if !deps.Tangle.IsNodeHealthy() {
			c.NoContent(http.StatusServiceUnavailable)
			return nil
		}

		c.NoContent(http.StatusOK)
		return nil
	})
}
