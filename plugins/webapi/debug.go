package webapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

func init() {
	addEndpoint("getRequests", getRequests, implementedAPIcalls)
	addEndpoint("triggerSolidifier", triggerSolidifier, implementedAPIcalls)
}

func getRequests(_ interface{}, c *gin.Context, _ <-chan struct{}) {
	queued, pending, processing := gossip.RequestQueue().Requests()
	debugReqs := make([]*DebugRequest, len(queued)+len(pending))

	offset := 0
	for i := 0; i < len(queued); i++ {
		req := queued[i]
		debugReqs[offset+i] = &DebugRequest{
			MessageID:        req.MessageID.Hex(),
			Type:             "queued",
			TxExists:         tangle.ContainsMessage(req.MessageID),
			MilestoneIndex:   req.MilestoneIndex,
			EnqueueTimestamp: req.EnqueueTime.Unix(),
		}
	}
	offset += len(queued)
	for i := 0; i < len(pending); i++ {
		req := pending[i]
		debugReqs[offset+i] = &DebugRequest{
			MessageID:        req.MessageID.Hex(),
			Type:             "pending",
			TxExists:         tangle.ContainsMessage(req.MessageID),
			MilestoneIndex:   req.MilestoneIndex,
			EnqueueTimestamp: req.EnqueueTime.Unix(),
		}
	}
	offset += len(pending)
	for i := 0; i < len(processing); i++ {
		req := processing[i]
		debugReqs[offset+i] = &DebugRequest{
			MessageID:        req.MessageID.Hex(),
			Type:             "processing",
			TxExists:         tangle.ContainsMessage(req.MessageID),
			MilestoneIndex:   req.MilestoneIndex,
			EnqueueTimestamp: req.EnqueueTime.Unix(),
		}
	}
	c.JSON(http.StatusOK, GetRequestsReturn{Requests: debugReqs})
}

func triggerSolidifier(i interface{}, c *gin.Context, _ <-chan struct{}) {
	tanglePlugin.TriggerSolidifier()
	c.Status(http.StatusAccepted)
}
