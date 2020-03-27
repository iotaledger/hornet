package webapi

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
)

const (
	healthzRoute = "healthz"
	healthRoute  = "health"
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

	// GET /health
	api.GET(healthRoute, func(c *gin.Context) {
		if !isNodeHealthy() {
			c.Status(http.StatusServiceUnavailable)
			return
		}

		c.Status(http.StatusOK)
	})

	// GET /healthz
	api.GET(healthzRoute, func(c *gin.Context) {
		if !isNodeHealthy() {
			c.Status(http.StatusServiceUnavailable)
			return
		}

		c.Status(http.StatusOK)
	})
}

func isNodeHealthy() bool {
	// Synced
	if !tangle.IsNodeSyncedWithThreshold() {
		return false
	}

	// Has connected neighbors
	if len(gossip.GetConnectedNeighbors()) == 0 {
		return false
	}

	// Latest milestone timestamp
	var milestoneTimestamp int64
	lmi := tangle.GetLatestMilestoneIndex()
	cachedLatestMs := tangle.GetMilestoneOrNil(lmi) // bundle +1
	if cachedLatestMs != nil {
		cachedMsTailTx := cachedLatestMs.GetBundle().GetTail() // tx +1
		milestoneTimestamp = cachedMsTailTx.GetTransaction().GetTimestamp()
		cachedMsTailTx.Release(true) // tx -1
		cachedLatestMs.Release(true) // bundle -1
	}

	// Check whether the milestone is older than 5 minutes
	timeMs := time.Unix(int64(milestoneTimestamp), 0)
	if time.Since(timeMs) > (time.Minute * 5) {
		return false
	}

	return true
}
