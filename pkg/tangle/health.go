package tangle

import (
	"time"

	"github.com/iotaledger/hornet/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/pkg/protocol/gossip"
)

const (
	maxAllowedMilestoneAge = time.Minute * 5
)

// IsNodeHealthy returns whether the node is synced, has active neighbors and its latest milestone is not too old.
func (t *Tangle) IsNodeHealthy(sync ...*syncmanager.SyncState) bool {

	var syncState *syncmanager.SyncState
	if len(sync) > 0 {
		syncState = sync[0]
	} else {
		syncState = t.syncManager.SyncState()
	}

	if !syncState.NodeAlmostSynced {
		return false
	}

	var gossipStreamsOngoing int
	t.gossipService.ForEach(func(_ *gossip.Protocol) bool {
		gossipStreamsOngoing++

		// one stream is enough
		return false
	})

	if gossipStreamsOngoing == 0 {
		return false
	}

	// latest milestone timestamp
	lmi := syncState.LatestMilestoneIndex
	cachedMilestone := t.storage.CachedMilestoneOrNil(lmi) // milestone +1
	if cachedMilestone == nil {
		return false
	}
	defer cachedMilestone.Release(true) // milestone -1

	// check whether the milestone is older than 5 minutes
	timeMs := cachedMilestone.Milestone().Timestamp
	return time.Since(timeMs) < maxAllowedMilestoneAge
}
