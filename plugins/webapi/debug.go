package webapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gohornet/hornet/plugins/gossip"
)

func init() {
	addEndpoint("getRequests", getRequests, implementedAPIcalls)
}

func getRequests(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	grr := &GetRequestsReturn{}
	grr.Requests = gossip.RequestQueue.DebugRequests()
	c.JSON(http.StatusOK, grr)
}
