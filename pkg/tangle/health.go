package tangle

import (
	"time"

	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
)

const (
	maxAllowedMilestoneAge = time.Minute * 5
)

// IsNodeHealthy returns whether the node is synced, has active peers and its latest milestone is not too old.
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

	if !t.protocolManager.NextPendingSupported() {
		return false
	}

	// latest milestone timestamp
	milestoneTimestamp, err := t.storage.MilestoneTimestampByIndex(syncState.LatestMilestoneIndex)
	if err != nil {
		return false
	}

	// check whether the milestone is older than 5 minutes
	return time.Since(milestoneTimestamp) < maxAllowedMilestoneAge
}
