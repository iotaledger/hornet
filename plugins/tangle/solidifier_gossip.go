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
		tx := task.Param(0).(*tangle.CachedTransaction) //1
		if tangle.IsNodeSynced() {
			checkSolidityAndPropagate(tx)
		}
		// Release the consumer, since it was registered before adding to the pool
		tx.Release() //-1

		task.Return(nil)
	}, workerpool.WorkerCount(gossipSolidifierWorkerCount), workerpool.QueueSize(gossipSolidifierQueueSize))

}

func runGossipSolidifier() {
	log.Info("Starting Solidifier ...")

	notifyNewTx := events.NewClosure(func(transaction *tangle.CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		if tangle.IsNodeSynced() {
			transaction.RegisterConsumer() //+1
			gossipSolidifierWorkerPool.Submit(transaction)
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
func checkSolidityAndPropagate(transaction *tangle.CachedTransaction) {

	//Register consumer here, since we will add it to txsToCheck which will release every tx when they are processed
	transaction.RegisterConsumer() //+1

	txsToCheck := make(map[string]*tangle.CachedTransaction)
	txsToCheck[transaction.GetTransaction().GetHash()] = transaction

	// Loop as long as new transactions are added in every loop cycle
	for len(txsToCheck) != 0 {
		for txHash, tx := range txsToCheck {
			delete(txsToCheck, txHash)

			solid, _ := checkSolidity(tx, true)
			if solid {
				if int32(time.Now().Unix())-tx.GetTransaction().GetSolidificationTimestamp() > solidifierThresholdInSeconds {
					// Skip older transactions
					tx.Release() //-1
					continue
				}

				transactionApprovers := tangle.GetCachedApprovers(txHash) //+1
				for _, approver := range transactionApprovers {
					if approver.Exists() {
						approverHash := approver.GetApprover().GetHash()
						approverTx := tangle.GetCachedTransaction(approverHash) //+1
						if approverTx.Exists() {
							_, found := txsToCheck[approverHash]
							if !found {
								txsToCheck[approverHash] = approverTx
							} else {
								approverTx.Release() //-1
							}
						} else {
							approverTx.Release() //-1
						}
					}
				}
				transactionApprovers.Release() //-1
			}
			tx.Release() //-1
		}
	}
}
