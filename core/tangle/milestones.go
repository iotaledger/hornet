package tangle

import (
	"encoding/hex"

	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/core/gossip"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

var (
	processValidMilestoneWorkerCount = 1 // This must not be done in parallel
	processValidMilestoneQueueSize   = 10000
	processValidMilestoneWorkerPool  *workerpool.WorkerPool
)

func processValidMilestone(cachedMilestone *tangle.CachedMilestone) {
	defer cachedMilestone.Release(true) // message -1

	Events.ReceivedNewMilestone.Trigger(cachedMilestone) // milestone pass +1

	solidMsIndex := database.Tangle().GetSolidMilestoneIndex()
	msIndex := cachedMilestone.GetMilestone().Index

	if database.Tangle().SetLatestMilestoneIndex(msIndex) {
		Events.LatestMilestoneChanged.Trigger(cachedMilestone) // milestone pass +1
		Events.LatestMilestoneIndexChanged.Trigger(msIndex)
	}
	milestoneSolidifierWorkerPool.TrySubmit(msIndex, false)

	if msIndex > solidMsIndex {
		log.Infof("Valid milestone detected! Index: %d, MilestoneID: %v", msIndex, hex.EncodeToString(cachedMilestone.GetMilestone().MilestoneID[:]))

		// request parent1 and parent2
		gossip.RequestMilestoneParents(cachedMilestone.Retain()) // milestone pass +1
	} else {
		pruningIndex := database.Tangle().GetSnapshotInfo().PruningIndex
		if msIndex < pruningIndex {
			// this should not happen. we didn't request it and it should be filtered because of timestamp
			log.Warnf("Synced too far back! Index: %d (%v), PruningIndex: %d", msIndex, hex.EncodeToString(cachedMilestone.GetMilestone().MilestoneID[:]), pruningIndex)
		}
	}
}
