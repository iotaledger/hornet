package webapi

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

var (
	// ErrNodeNotSync is returned when the node was not synced.
	ErrNodeNotSync = errors.New("node not synced")
	// ErrInternalError is returned when there was an internal node error.
	ErrInternalError = errors.New("internal error")
)

func networkWhitelisted(c *gin.Context) bool {
	remoteHost, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
	remoteAddress := net.ParseIP(remoteHost)
	for _, whitelistedNet := range whitelistedNetworks {
		if whitelistedNet.Contains(remoteAddress) {
			return true
		}
	}
	return false
}

func webAPIRoute() {
	api.POST(webAPIBase, func(c *gin.Context) {

		request := make(map[string]interface{})

		err := c.ShouldBindJSON(&request)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorReturn{Error: err.Error()})
			return
		}

		originCmd, exists := request["command"]
		if !exists {
			c.JSON(http.StatusBadRequest, ErrorReturn{Error: "error parsing command"})
			return
		}
		cmd := strings.ToLower(originCmd.(string))

		// get the command and check if it's implemented
		implementation, apiCallExists := implementedAPIcalls[cmd]
		if !apiCallExists {
			c.JSON(http.StatusBadRequest, ErrorReturn{Error: fmt.Sprintf("command [%v] is unknown", originCmd)})
			return
		}

		if !networkWhitelisted(c) {
			// network is not whitelisted, check if the command is permitted, otherwise deny it.
			if _, permitted := permittedEndpoints[cmd]; !permitted {
				c.JSON(http.StatusForbidden, ErrorReturn{Error: fmt.Sprintf("command [%v] is protected", originCmd)})
				return
			}
		}

		implementation(&request, c, serverShutdownSignal)
	})
}
