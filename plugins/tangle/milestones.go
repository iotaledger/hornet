package tangle

import (
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
)

var (
	processValidMilestoneWorkerCount = 1 // This must not be done in parallel
	processValidMilestoneQueueSize   = 10000
	processValidMilestoneWorkerPool  *workerpool.WorkerPool
)

func processValidMilestone(cachedBndl *tangle.CachedBundle) {
	defer cachedBndl.Release() // bundle -1

	Events.ReceivedNewMilestone.Trigger(cachedBndl) // bundle pass +1

	solidMsIndex := tangle.GetSolidMilestoneIndex()
	bundleMsIndex := cachedBndl.GetBundle().GetMilestoneIndex()

	latestMilestoneIndex := tangle.GetLatestMilestoneIndex()
	if latestMilestoneIndex < bundleMsIndex {
		err := tangle.SetLatestMilestone(cachedBndl.Retain()) // bundle pass +1
		if err != nil {
			log.Error(err)
		}

		Events.LatestMilestoneChanged.Trigger(cachedBndl) // bundle pass +1
	}
	milestoneSolidifierWorkerPool.TrySubmit(bundleMsIndex, false)

	if bundleMsIndex > solidMsIndex {
		log.Infof("Valid milestone detected! Index: %d, Hash: %v", bundleMsIndex, cachedBndl.GetBundle().GetMilestoneHash())

		// Request trunk and branch
		gossip.RequestMilestoneApprovees(cachedBndl.Retain()) // bundle pass +1

	} else {
		pruningIndex := tangle.GetSnapshotInfo().PruningIndex
		if bundleMsIndex < pruningIndex {
			// This should not happen! We didn't request it and it should be filtered because of timestamp
			log.Panicf("Synced too far! Index: %d (%v), PruningIndex: %d", bundleMsIndex, cachedBndl.GetBundle().GetMilestoneHash(), pruningIndex)
		}
	}
}
