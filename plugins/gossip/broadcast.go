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

// BroadcastMilestoneRequests broadcasts requests for all missing milestones between LSMI and LMI
// to every connected peer who supports STING.
func BroadcastMilestoneRequests(solidMilestoneIndex milestone.Index, knownLatestMilestone milestone.Index) {

	if solidMilestoneIndex == 0 || knownLatestMilestone == 0 || solidMilestoneIndex == knownLatestMilestone {
		// don't request anything if we are sync (or don't know about a newer ms)
		return
	}

	var msIndexes []milestone.Index
	for milestoneIndex := solidMilestoneIndex + 1; milestoneIndex < knownLatestMilestone; milestoneIndex++ {
		// make sure we only request what we don't have
		if !tangle.ContainsMilestone(milestoneIndex) {
			msIndexes = append(msIndexes, milestoneIndex)
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
