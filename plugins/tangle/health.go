package tangle

import (
	"time"

	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/plugins/peering"
)

const (
	maxAllowedMilestoneAge = time.Minute * 5
)

// IsNodeHealthy returns whether the node is synced, has active neighbors and its latest milestone is not too old.
func IsNodeHealthy() bool {
	// Synced
	if !tangle.IsNodeSyncedWithThreshold() {
		return false
	}

	// Has connected neighbors
	if peering.Manager().ConnectedPeerCount() == 0 {
		return false
	}

	// Latest milestone timestamp
	lmi := tangle.GetLatestMilestoneIndex()
	cachedLatestMilestone := tangle.GetCachedMilestoneOrNil(lmi) // milestone +1
	if cachedLatestMilestone == nil {
		return false
	}
	defer cachedLatestMilestone.Release(true)

	// Check whether the milestone is older than 5 minutes
	timeMs := cachedLatestMilestone.GetMilestone().Timestamp
	return time.Since(timeMs) < maxAllowedMilestoneAge
}
