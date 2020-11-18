package tangle

import (
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/pkg/model/storage"
)

var (
	processValidMilestoneWorkerCount = 1 // This must not be done in parallel
	processValidMilestoneQueueSize   = 10000
	processValidMilestoneWorkerPool  *workerpool.WorkerPool
)

func (t *Tangle) processValidMilestone(cachedMilestone *storage.CachedMilestone) {
	defer cachedMilestone.Release(true) // message -1

	t.Events.ReceivedNewMilestone.Trigger(cachedMilestone) // milestone pass +1

	solidMsIndex := t.storage.GetSolidMilestoneIndex()
	msIndex := cachedMilestone.GetMilestone().Index

	if t.storage.SetLatestMilestoneIndex(msIndex) {
		t.Events.LatestMilestoneChanged.Trigger(cachedMilestone) // milestone pass +1
		t.Events.LatestMilestoneIndexChanged.Trigger(msIndex)
	}
	t.milestoneSolidifierWorkerPool.TrySubmit(msIndex, false)

	if msIndex > solidMsIndex {
		t.log.Infof("Valid milestone detected! Index: %d", msIndex)

		// request parent1 and parent2
		t.requester.RequestMilestoneParents(cachedMilestone.Retain()) // milestone pass +1
	} else {
		pruningIndex := t.storage.GetSnapshotInfo().PruningIndex
		if msIndex < pruningIndex {
			// this should not happen. we didn't request it and it should be filtered because of timestamp
			t.log.Warnf("Synced too far back! Index: %d, PruningIndex: %d", msIndex, pruningIndex)
		}
	}
}
