package gossip

import (
	"encoding/binary"
	"github.com/gohornet/hornet/plugins/gossip/server"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

type Heartbeat struct {
	SolidMilestoneIndex  milestone_index.MilestoneIndex `json:"solid_milestone_index"`
	PrunedMilestoneIndex milestone_index.MilestoneIndex `json:"pruned_milestone_index"`
}

func HeartbeatFromBytes(data []byte) *Heartbeat {
	return &Heartbeat{
		SolidMilestoneIndex:  milestone_index.MilestoneIndex(binary.BigEndian.Uint32(data[:4])),
		PrunedMilestoneIndex: milestone_index.MilestoneIndex(binary.BigEndian.Uint32(data[4:])),
	}
}

func SendHeartbeat() {
	neighborQueuesMutex.RLock()
	defer neighborQueuesMutex.RUnlock()

	snapshotInfo := tangle.GetSnapshotInfo()

	for _, neighborQueue := range neighborQueues {
		if !neighborQueue.protocol.SupportsSTING() {
			continue
		}

		msg := &Heartbeat{SolidMilestoneIndex: tangle.GetSolidMilestoneIndex(), PrunedMilestoneIndex: snapshotInfo.PruningIndex}
		select {
		case neighborQueue.heartbeatQueue <- msg:
		default:
			neighborQueue.protocol.Neighbor.Metrics.IncrDroppedSendPacketsCount()
			server.SharedServerMetrics.IncrDroppedSendPacketsCount()
		}

	}
}
