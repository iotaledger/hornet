package tangle

import (
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
)

var (
	processValidMilestoneWorkerCount = 1 // This must not be done in parallel
	processValidMilestoneQueueSize   = 10000
	processValidMilestoneWorkerPool  *workerpool.WorkerPool
)

func processValidMilestone(cachedMilestone *tangle.CachedMilestone) {
	defer cachedMilestone.Release() // message -1

	Events.ReceivedNewMilestone.Trigger(cachedMilestone) // milestone pass +1

	solidMsIndex := tangle.GetSolidMilestoneIndex()
	msIndex := cachedMilestone.GetMilestone().Index

	if tangle.SetLatestMilestoneIndex(msIndex) {
		Events.LatestMilestoneChanged.Trigger(cachedMilestone) // milestone pass +1
		Events.LatestMilestoneIndexChanged.Trigger(msIndex)
	}
	milestoneSolidifierWorkerPool.TrySubmit(msIndex, false)

	if msIndex > solidMsIndex {
		log.Infof("Valid milestone detected! Index: %d, MilestoneMessageID: %v", msIndex, cachedMilestone.GetMilestone().MessageID.Hex())

		// request parent1 and parent2
		gossip.RequestMilestoneParents(cachedMilestone.Retain()) // milestone pass +1
	} else {
		pruningIndex := tangle.GetSnapshotInfo().PruningIndex
		if msIndex < pruningIndex {
			// this should not happen. we didn't request it and it should be filtered because of timestamp
			log.Warnf("Synced too far back! Index: %d (%v), PruningIndex: %d", msIndex, cachedMilestone.GetMilestone().MessageID.Hex(), pruningIndex)
		}
	}
}
