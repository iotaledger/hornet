package tangle

import (
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	iotago "github.com/iotaledger/iota.go/v3"
)

func (t *Tangle) processValidMilestone(blockID iotago.BlockID, cachedMilestone *storage.CachedMilestone, requested bool) {
	defer cachedMilestone.Release(true) // milestone -1

	t.Events.ReceivedNewMilestoneBlock.Trigger(blockID)

	confirmedMsIndex := t.syncManager.ConfirmedMilestoneIndex()
	msIndex := cachedMilestone.Milestone().Index()

	if t.syncManager.SetLatestMilestoneIndex(msIndex) {
		t.Events.LatestMilestoneChanged.Trigger(cachedMilestone) // milestone pass +1
		t.Events.LatestMilestoneIndexChanged.Trigger(msIndex)
	}
	t.milestoneSolidifierWorkerPool.TrySubmit(msIndex, false)

	if msIndex > confirmedMsIndex {
		t.LogInfof("Valid milestone detected! Index: %d", msIndex)
		t.requester.RequestMilestoneParents(cachedMilestone.Retain()) // milestone pass +1
	} else if requested {
		snapshotInfo := t.storage.SnapshotInfo()
		if snapshotInfo == nil {
			t.LogPanic(common.ErrSnapshotInfoNotFound)

			return
		}

		pruningIndex := snapshotInfo.PruningIndex()
		if msIndex < pruningIndex {
			// this should not happen. we requested a milestone that is below pruning index
			t.LogPanicf("Synced too far back! Index: %d, PruningIndex: %d", msIndex, pruningIndex)
		}
	}
}
