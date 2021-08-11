package tangle

import (
	"github.com/gohornet/hornet/pkg/model/storage"
)

func (t *Tangle) processValidMilestone(cachedMilestone *storage.CachedMilestone, requested bool) {
	defer cachedMilestone.Release(true) // message -1

	t.Events.ReceivedNewMilestone.Trigger(cachedMilestone) // milestone pass +1

	confirmedMsIndex := t.storage.ConfirmedMilestoneIndex()
	msIndex := cachedMilestone.Milestone().Index

	if t.storage.SetLatestMilestoneIndex(msIndex) {
		t.Events.LatestMilestoneChanged.Trigger(cachedMilestone) // milestone pass +1
		t.Events.LatestMilestoneIndexChanged.Trigger(msIndex)
	}
	t.milestoneSolidifierWorkerPool.TrySubmit(msIndex, false)

	if msIndex > confirmedMsIndex {
		t.log.Infof("Valid milestone detected! Index: %d", msIndex)
		t.requester.RequestMilestoneParents(cachedMilestone.Retain()) // milestone pass +1
	} else if requested {
		pruningIndex := t.storage.SnapshotInfo().PruningIndex
		if msIndex < pruningIndex {
			// this should not happen. we requested a milestone that is below pruning index
			t.log.Panicf("Synced too far back! Index: %d, PruningIndex: %d", msIndex, pruningIndex)
		}
	}
}
