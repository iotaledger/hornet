package webapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/hornet/pkg/config"
	"github.com/iotaledger/hornet/plugins/tangle"
)

func healthzRoute() {
	e.GET("/healthz", func(c echo.Context) error {
		// autopeering entrypoint mode
		if config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
			return c.NoContent(http.StatusOK)
		}

		// node mode
		if !tangle.IsNodeHealthy() {
			return c.NoContent(http.StatusServiceUnavailable)
		}

		return c.NoContent(http.StatusOK)
	})
}
