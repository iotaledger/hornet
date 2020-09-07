package webapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/plugins/tangle"
)

func healthzRoute() {
	api.GET("/healthz", func(c *gin.Context) {

		if !networkWhitelisted(c) {
			// network is not whitelisted, check if the route is permitted, otherwise deny it.
			if _, permitted := permittedRESTroutes["healthz"]; !permitted {
				c.JSON(http.StatusForbidden, ErrorReturn{Error: "route [healthz] is protected"})
				return
			}
		}

		// autopeering entrypoint mode
		if config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
			c.Status(http.StatusOK)
			return
		}

		// node mode
		if !tangle.IsNodeHealthy() {
			c.Status(http.StatusServiceUnavailable)
			return
		}

		c.Status(http.StatusOK)
	})
}
