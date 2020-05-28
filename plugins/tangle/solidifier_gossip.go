package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
)

const (
	solidifierThresholdInSeconds int32 = 60
)

var (
	gossipSolidifierWorkerCount = 1
	gossipSolidifierQueueSize   = 5000
	gossipSolidifierWorkerPool  *workerpool.WorkerPool
)

func configureGossipSolidifier() {
	gossipSolidifierWorkerPool = workerpool.New(func(task workerpool.Task) {
		// Check solidity of gossip txs if the node is synced
		cachedTx := task.Param(0).(*tangle.CachedTransaction)
		if tangle.IsNodeSyncedWithThreshold() {
			checkSolidityAndPropagate(cachedTx) // tx pass +1
		} else {
			// Force release allowed if the node is not synced
			cachedTx.Release(true) // tx -1
		}

		task.Return(nil)
	}, workerpool.WorkerCount(gossipSolidifierWorkerCount), workerpool.QueueSize(gossipSolidifierQueueSize), workerpool.FlushTasksAtShutdown(true))
}

func runGossipSolidifier() {
	log.Info("Starting Solidifier ...")

	notifyNewTx := events.NewClosure(func(cachedTx *tangle.CachedTransaction, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
		if tangle.IsNodeSyncedWithThreshold() {
			_, added := gossipSolidifierWorkerPool.Submit(cachedTx) // tx pass +1
			if !added {
				// Force release possible here, since processIncomingTx still holds a reference
				cachedTx.Release(true) // tx -1
			}
		} else {
			// Force release possible here, since processIncomingTx still holds a reference
			cachedTx.Release(true) // tx -1
		}
	})

	daemon.BackgroundWorker("Tangle Solidifier", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Solidifier ... done")
		Events.ReceivedNewTransaction.Attach(notifyNewTx)
		gossipSolidifierWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Solidifier ...")
		Events.ReceivedNewTransaction.Detach(notifyNewTx)
		gossipSolidifierWorkerPool.StopAndWait()

		log.Info("Stopping Solidifier ... done")
	}, shutdown.PrioritySolidifierGossip)
}

// Checks and updates the solid flag of a transaction and its approvers (future cone).
func checkSolidityAndPropagate(cachedTx *tangle.CachedTransaction) {

	txsToCheck := make(map[string]*tangle.CachedTransaction)
	txsToCheck[string(cachedTx.GetTransaction().GetTxHash())] = cachedTx

	// Loop as long as new transactions are added in every loop cycle
	for len(txsToCheck) != 0 {
		for txHash, cachedTxToCheck := range txsToCheck {
			delete(txsToCheck, txHash)

			_, newlySolid := checkSolidity(cachedTxToCheck.Retain())
			if newlySolid {
				if int32(time.Now().Unix())-cachedTxToCheck.GetMetadata().GetSolidificationTimestamp() > solidifierThresholdInSeconds {
					// Skip older transactions and force release them
					cachedTxToCheck.Release(true) // tx -1
					continue
				}

				for _, approverHash := range tangle.GetApproverHashes(hornet.Hash(txHash), true) {
					cachedApproverTx := tangle.GetCachedTransactionOrNil(approverHash) // tx +1
					if cachedApproverTx == nil {
						continue
					}

					if _, found := txsToCheck[string(approverHash)]; found {
						// Do no force release here, otherwise cacheTime for new Tx could be ignored
						cachedApproverTx.Release() // tx -1
						continue
					}

					txsToCheck[string(approverHash)] = cachedApproverTx
				}
			}
			// Do no force release here, otherwise cacheTime for new Tx could be ignored
			cachedTxToCheck.Release() // tx -1
		}
	}
}
