package webapi

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/utils"
)

const healthzRoute = "healthz"

var (
	// ErrNodeNotSync is returned when the node was not synced.
	ErrNodeNotSync = errors.New("node not synced")
	// ErrInternalError is returned when there was an internal node error.
	ErrInternalError = errors.New("internal error")
)

func webAPIRoute() {
	api.POST(webAPIBase, func(c *gin.Context) {

		request := make(map[string]interface{})

		err := c.ShouldBindJSON(&request)
		if err != nil {
			fmt.Println(err)
		}

		// Get the command and check if it's implemented
		originCommand := fmt.Sprint(request["command"])
		cmd := strings.ToLower(originCommand)

		implementation, apiCallExists := implementedAPIcalls[cmd]

		whitelisted := false
		remoteHost, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		remoteAddress := net.ParseIP(remoteHost)
		for _, whitelistedNet := range whitelistedNetworks {
			if whitelistedNet.Contains(remoteAddress) {
				whitelisted = true
				break
			}
		}

		if !whitelisted {
			// Check if command is permitted. If it's not permited and the request does not come from localhost, deny it.
			_, permited := permitedEndpoints[cmd]
			if apiCallExists && !permited {
				e := ErrorReturn{
					Error: fmt.Sprintf("Command [%v] is protected", originCommand),
				}
				c.JSON(http.StatusForbidden, e)
				return
			}
		}

		if !apiCallExists {
			e := ErrorReturn{
				Error: fmt.Sprintf("Command [%v] is unknown", originCommand),
			}
			c.JSON(http.StatusBadRequest, e)
			return
		}

		implementation(&request, c, serverShutdownSignal)
	})
}

// health check
func restAPIRoute() {

	if config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
		// autopeering entry node mode
		// GET /healthz
		api.GET(healthzRoute, func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
		return
	}

	// node mode
	// GET /healthz
	api.GET(healthzRoute, func(c *gin.Context) {
		if !utils.IsNodeHealthy() {
			c.Status(http.StatusServiceUnavailable)
			return
		}

		c.Status(http.StatusOK)
	})
}
