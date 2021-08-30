package tangle

import (
	"time"

	"github.com/gohornet/hornet/pkg/protocol/gossip"
)

const (
	maxAllowedMilestoneAge = time.Minute * 5
)

// IsNodeHealthy returns whether the node is synced, has active neighbors and its latest milestone is not too old.
func (t *Tangle) IsNodeHealthy() bool {
	if !t.storage.IsNodeAlmostSynced() {
		return false
	}

	var gossipStreamsOngoing int
	t.service.ForEach(func(_ *gossip.Protocol) bool {
		gossipStreamsOngoing++
		return true
	})

	if gossipStreamsOngoing == 0 {
		return false
	}

	// latest milestone timestamp
	lmi := t.storage.LatestMilestoneIndex()
	cachedLatestMilestone := t.storage.CachedMilestoneOrNil(lmi) // milestone +1
	if cachedLatestMilestone == nil {
		return false
	}
	defer cachedLatestMilestone.Release(true)

	// check whether the milestone is older than 5 minutes
	timeMs := cachedLatestMilestone.Milestone().Timestamp
	return time.Since(timeMs) < maxAllowedMilestoneAge
}
