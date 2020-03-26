package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
)

const (
	solidifierThresholdInSeconds int32 = 60
)

var (
	gossipSolidifierWorkerPool = (&async.WorkerPool{}).Tune(1)
)

func processGossipSolidificationTask(cachedTx *tangle.CachedTransaction) {
	// Check solidity of gossip txs if the node is synced
	if !tangle.IsNodeSyncedWithThreshold() {
		// Force release allowed if the node is not synced
		cachedTx.Release(true) // tx -1
		return
	}

	checkSolidityAndPropagate(cachedTx) // tx pass +1
}

func runGossipSolidifier() {
	log.Info("Starting Solidifier ...")

	notifyNewTx := events.NewClosure(func(cachedTx *tangle.CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		if !tangle.IsNodeSyncedWithThreshold() {
			// Force release possible here, since processIncomingTx still holds a reference
			cachedTx.Release(true) // tx -1
			return
		}
		gossipSolidifierWorkerPool.Submit(func() { processGossipSolidificationTask(cachedTx) }) // tx pass +1
	})

	daemon.BackgroundWorker("Tangle Solidifier", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Solidifier ... done")
		Events.ReceivedNewTransaction.Attach(notifyNewTx)
		<-shutdownSignal
		log.Info("Stopping Solidifier ...")
		Events.ReceivedNewTransaction.Detach(notifyNewTx)
		gossipSolidifierWorkerPool.ShutdownGracefully()

		log.Info("Stopping Solidifier ... done")
	}, shutdown.ShutdownPrioritySolidifierGossip)
}

// Checks and updates the solid flag of a transaction and its approvers (future cone).
func checkSolidityAndPropagate(cachedTx *tangle.CachedTransaction) {

	txsToCheck := make(map[string]*tangle.CachedTransaction)
	txsToCheck[cachedTx.GetTransaction().GetHash()] = cachedTx

	// Loop as long as new transactions are added in every loop cycle
	for len(txsToCheck) != 0 {
		for txHash, cachedTxToCheck := range txsToCheck {
			delete(txsToCheck, txHash)

			// We don't have to revalidate in the future cone solidifier, since the node wouldn't be solid anyway
			// if revalidateDatabase was triggered at startup
			_, newlySolid := checkSolidity(cachedTxToCheck.Retain())
			if newlySolid {
				if int32(time.Now().Unix())-cachedTxToCheck.GetMetadata().GetSolidificationTimestamp() > solidifierThresholdInSeconds {
					// Skip older transactions and force release them
					cachedTxToCheck.Release(true) // tx -1
					continue
				}

				for _, approverHash := range tangle.GetApproverHashes(txHash, true) {
					cachedApproverTx := tangle.GetCachedTransactionOrNil(approverHash) // tx +1
					if cachedApproverTx == nil {
						continue
					}

					if _, found := txsToCheck[approverHash]; found {
						// Do no force release here, otherwise cacheTime for new Tx could be ignored
						cachedApproverTx.Release() // tx -1
						continue
					}

					txsToCheck[approverHash] = cachedApproverTx
				}
			}
			// Do no force release here, otherwise cacheTime for new Tx could be ignored
			cachedTxToCheck.Release() // tx -1
		}
	}
}
