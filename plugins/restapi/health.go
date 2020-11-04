package restapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/gohornet/hornet/core/tangle"
)

func setupHealthRoute() {
	deps.Echo.GET(nodeAPIHealthRoute, func(c echo.Context) error {

		// node mode
		if !tangle.IsNodeHealthy() {
			c.NoContent(http.StatusServiceUnavailable)
			return nil
		}

		c.NoContent(http.StatusOK)
		return nil
	})
}
