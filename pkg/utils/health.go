package utils

import (
	"time"

	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/plugins/peering"
)

const maxAllowedMilestoneAge = time.Minute * 5

// IsNodeHealthy returns whether the node is synced, has active neighbors and its latest milestone is not too old
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
	var milestoneTimestamp int64
	lmi := tangle.GetLatestMilestoneIndex()
	cachedLatestMs := tangle.GetMilestoneOrNil(lmi) // bundle +1
	if cachedLatestMs == nil {
		return false
	}

	cachedMsTailTx := cachedLatestMs.GetBundle().GetTail() // tx +1
	milestoneTimestamp = cachedMsTailTx.GetTransaction().GetTimestamp()
	cachedMsTailTx.Release(true) // tx -1
	cachedLatestMs.Release(true) // bundle -1

	// Check whether the milestone is older than 5 minutes
	timeMs := time.Unix(milestoneTimestamp, 0)
	return time.Since(timeMs) < maxAllowedMilestoneAge
}
