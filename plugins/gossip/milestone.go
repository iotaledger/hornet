package gossip

import (
	"math"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip/server"
)

const milestoneRequestRange = 50

func SendMilestoneRequests(solidMilestoneIndex milestone_index.MilestoneIndex, knownLatestMilestone milestone_index.MilestoneIndex) {
	var rangeToRequest int
	if solidMilestoneIndex != 0 && knownLatestMilestone != 0 {
		rangeToRequest = int(math.Min(float64(milestoneRequestRange), float64(knownLatestMilestone-solidMilestoneIndex)))
	} else {
		rangeToRequest = milestoneRequestRange
	}

	// don't request anything if we are sync (or don't know about a newer ms)
	if rangeToRequest == 0 {
		return
	}

	// make sure we only request what we don't have
	msIndexesToRequest := []milestone_index.MilestoneIndex{}
	for i := 1; i <= rangeToRequest; i++ {
		toReq := solidMilestoneIndex + milestone_index.MilestoneIndex(i)
		ms, _ := tangle.GetMilestone(toReq)
		// don't need to request as we have the milestone
		if ms != nil {
			continue
		}
		msIndexesToRequest = append(msIndexesToRequest, toReq)
	}

	if len(msIndexesToRequest) == 0 {
		return
	}

	neighborQueuesMutex.RLock()
	defer neighborQueuesMutex.RUnlock()

	// send each ms request to a random neighbor who supports the message
	for _, msIndexToReq := range msIndexesToRequest {
		for _, neighborQueue := range neighborQueues {
			if !neighborQueue.protocol.SupportsSTING() {
				continue
			}
			latestHb := neighborQueue.protocol.Neighbor.LatestHeartbeat
			if latestHb == nil {
				continue
			}
			// neighbor must have the range of transactions in which the milestone exists
			if msIndexToReq <= latestHb.PrunedMilestoneIndex || msIndexToReq > latestHb.SolidMilestoneIndex {
				continue
			}
			select {
			case neighborQueue.sendMilestoneRequestQueue <- msIndexToReq:
			default:
				neighborQueue.protocol.Neighbor.Metrics.IncrDroppedSendPacketsCount()
				server.SharedServerMetrics.IncrDroppedSendPacketsCount()
			}

		}
	}
}
