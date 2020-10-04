package restapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/plugins/restapi/common"
	"github.com/gohornet/hornet/plugins/tangle"
)

func setupHealthzRoute(e *echo.Echo) {

	e.GET(NodeAPIHealthzRoute, func(c echo.Context) error {

		if !networkWhitelisted(c) {
			// network is not whitelisted, check if the route is permitted, otherwise deny it.
			if _, permitted := permittedRoutes["healthz"]; !permitted {
				return common.ErrForbidden
			}
		}

		// autopeering entrypoint mode
		if config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
			c.NoContent(http.StatusOK)
			return nil
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
