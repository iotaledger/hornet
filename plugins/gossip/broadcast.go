package gossip

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol/helpers"
	"github.com/gohornet/hornet/pkg/protocol/sting"
)

// BroadcastHeartbeat broadcasts a heartbeat message to every connected peer who supports STING.
func BroadcastHeartbeat() {
	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		return
	}
	heartbeatMsg, _ := sting.NewHeartbeatMessage(tangle.GetSolidMilestoneIndex(), snapshotInfo.PruningIndex)
	manager.ForAllConnected(func(p *peer.Peer) bool {
		if !p.Protocol.Supports(sting.FeatureSet) {
			return false
		}
		p.EnqueueForSending(heartbeatMsg)
		return false
	})
}

// BroadcastLatestMilestoneRequest broadcasts a milestone request for the latest milestone every connected peer who supports STING.
func BroadcastLatestMilestoneRequest() {
	manager.ForAllConnected(func(p *peer.Peer) bool {
		if !p.Protocol.Supports(sting.FeatureSet) {
			return false
		}
		helpers.SendLatestMilestoneRequest(p)
		return false
	})
}

// BroadcastMilestoneRequests broadcasts up to N requests for milestones nearest to the current solid milestone index
// to every connected peer who supports STING.
func BroadcastMilestoneRequests(rangeToRequest int) {

	// make sure we only request what we don't have
	solidMilestoneIndex := tangle.GetSolidMilestoneIndex()
	var msIndexes []milestone.Index
	for i := 1; i <= rangeToRequest; i++ {
		toReq := solidMilestoneIndex + milestone.Index(i)
		// only request if we do not have the milestone
		if !tangle.ContainsMilestone(toReq) {
			msIndexes = append(msIndexes, toReq)
		}
	}

	if len(msIndexes) == 0 {
		return
	}

	// send each ms request to a random peer who supports the message
	for _, msIndex := range msIndexes {
		manager.ForAllConnected(func(p *peer.Peer) bool {
			if !p.Protocol.Supports(sting.FeatureSet) {
				return false
			}
			if !p.HasDataFor(msIndex) {
				return false
			}
			helpers.SendMilestoneRequest(p, msIndex)
			return true
		})
	}
}
