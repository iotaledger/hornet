package tangle

import (
	"time"

	"github.com/iotaledger/hornet/pkg/protocol/gossip"
)

const (
	maxAllowedMilestoneAge = time.Minute * 5
)

// IsNodeHealthy returns whether the node is synced, has active neighbors and its latest milestone is not too old.
func (t *Tangle) IsNodeHealthy() bool {
	if !t.syncManager.IsNodeAlmostSynced() {
		return false
	}

	var gossipStreamsOngoing int
	t.gossipService.ForEach(func(_ *gossip.Protocol) bool {
		gossipStreamsOngoing++
		return true
	})

	if gossipStreamsOngoing == 0 {
		return false
	}

	if !t.protoMng.NextPendingSupported() {
		return false
	}

	// latest milestone timestamp
	lmi := t.syncManager.LatestMilestoneIndex()

	milestoneTimestamp, err := t.storage.MilestoneTimestampByIndex(lmi)
	if err != nil {
		return false
	}

	// check whether the milestone is older than 5 minutes
	return time.Since(milestoneTimestamp) < maxAllowedMilestoneAge
}
