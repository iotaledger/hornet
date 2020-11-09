package tangle

import (
	"time"

	"github.com/gohornet/hornet/pkg/protocol/gossip"
)

const (
	maxAllowedMilestoneAge = time.Minute * 5
)

// IsNodeHealthy returns whether the node is synced, has active neighbors and its latest milestone is not too old.
func IsNodeHealthy() bool {
	if !deps.Tangle.IsNodeSyncedWithThreshold() {
		return false
	}

	var gossipStreamsOngoing int
	deps.Service.ForEach(func(proto *gossip.Protocol) bool {
		gossipStreamsOngoing++
		return true
	})

	if gossipStreamsOngoing == 0 {
		return false
	}

	// latest milestone timestamp
	lmi := deps.Tangle.GetLatestMilestoneIndex()
	cachedLatestMilestone := deps.Tangle.GetCachedMilestoneOrNil(lmi) // milestone +1
	if cachedLatestMilestone == nil {
		return false
	}
	defer cachedLatestMilestone.Release(true)

	// check whether the milestone is older than 5 minutes
	timeMs := cachedLatestMilestone.GetMilestone().Timestamp
	return time.Since(timeMs) < maxAllowedMilestoneAge
}
