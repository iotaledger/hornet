package restapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/gohornet/hornet/plugins/restapi/common"
	"github.com/gohornet/hornet/plugins/tangle"
)

func setupHealthRoute(e *echo.Echo) {

	e.GET(NodeAPIHealthRoute, func(c echo.Context) error {

		if !networkWhitelisted(c) {
			// network is not whitelisted, check if the route is permitted, otherwise deny it.
			if _, permitted := permittedRoutes[NodeAPIHealthRoute]; !permitted {
				return common.ErrForbidden
			}
		}

		// node mode
		if !tangle.IsNodeHealthy() {
			c.NoContent(http.StatusServiceUnavailable)
			return nil
		}

		c.NoContent(http.StatusOK)
		return nil
	})
}
