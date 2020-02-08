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

func processValidMilestone(bundle *tangle.Bundle) {
	Events.ReceivedNewMilestone.Trigger(bundle)

	// ToDo: Is this still a thing?
	// Mark all tx of a valid milestone as requested, so they get stored on eviction
	// Warp sync milestone txs (via STING) are not requested by default => they would get lost
	cachedTxs := bundle.GetTransactions() // tx +1
	for _, cachedTx := range cachedTxs {
		cachedTx.GetTransaction().SetRequested(true)
	}
	cachedTxs.Release() // tx -1

	solidMsIndex := tangle.GetSolidMilestoneIndex()
	bundleMsIndex := bundle.GetMilestoneIndex()

	latestMilestoneIndex := tangle.GetLatestMilestoneIndex()
	if latestMilestoneIndex < bundleMsIndex {
		err := tangle.SetLatestMilestone(bundle)
		if err != nil {
			log.Error(err)
		}

		Events.LatestMilestoneChanged.Trigger(bundle)
		milestoneSolidifierWorkerPool.TrySubmit(bundleMsIndex)
	}

	if bundleMsIndex > solidMsIndex {
		log.Infof("Valid milestone detected! Index: %d, Hash: %v", bundleMsIndex, bundle.GetMilestoneHash())

		// Request trunk and branch
		gossip.RequestMilestone(bundle)

	} else {
		pruningIndex := tangle.GetSnapshotInfo().PruningIndex
		if bundleMsIndex < pruningIndex {
			// This should not happen! We didn't request it and it should be filtered because of timestamp
			log.Panicf("Synced too far! Index: %d (%v), PruningIndex: %d", bundleMsIndex, bundle.GetMilestoneHash(), pruningIndex)
		}
	}
}
