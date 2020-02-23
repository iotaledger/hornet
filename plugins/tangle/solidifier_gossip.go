package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
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
			cachedTx.Release() // tx -1
		}

		task.Return(nil)
	}, workerpool.WorkerCount(gossipSolidifierWorkerCount), workerpool.QueueSize(gossipSolidifierQueueSize), workerpool.FlushTasksAtShutdown(true))

}

func runGossipSolidifier() {
	log.Info("Starting Solidifier ...")

	notifyNewTx := events.NewClosure(func(cachedTx *tangle.CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		if tangle.IsNodeSyncedWithThreshold() {
			_, added := gossipSolidifierWorkerPool.Submit(cachedTx) // tx pass +1
			if !added {
				cachedTx.Release() // tx -1
			}
		} else {
			cachedTx.Release() // tx -1
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

			solid, _ := checkSolidity(cachedTxToCheck.Retain())
			if solid {
				if int32(time.Now().Unix())-cachedTxToCheck.GetMetadata().GetSolidificationTimestamp() > solidifierThresholdInSeconds {
					// Skip older transactions
					cachedTxToCheck.Release() // tx -1
					continue
				}

				cachedTxApprovers := tangle.GetCachedApprovers(txHash) // approvers +1
				for _, cachedTxApprover := range cachedTxApprovers {
					if cachedTxApprover.Exists() {
						approverHash := cachedTxApprover.GetApprover().GetApproverHash()

						cachedApproverTx := tangle.GetCachedTransaction(approverHash) // tx +1
						if !cachedApproverTx.Exists() {
							cachedApproverTx.Release() // tx -1
							continue
						}

						if _, found := txsToCheck[approverHash]; found {
							cachedApproverTx.Release() // tx -1
							continue
						}

						txsToCheck[approverHash] = cachedApproverTx
					}
				}
				cachedTxApprovers.Release() // approvers -1
			}
			cachedTxToCheck.Release() // tx -1
		}
	}
}
