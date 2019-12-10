package tangle

import (
	daemon "github.com/iotaledger/hive.go/daemon/ordered"
	"github.com/iotaledger/hive.go/events"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/packages/workerpool"
)

var (
	gossipSolidifierWorkerCount = 1
	gossipSolidifierQueueSize   = 5000
	gossipSolidifierWorkerPool  *workerpool.WorkerPool
)

func configureGossipSolidifier() {
	gossipSolidifierWorkerPool = workerpool.New(func(task workerpool.Task) {
		// Check solidity of gossip txs if the node is synced
		if tangle.IsNodeSynced() {
			checkSolidityAndPropagate(task.Param(0).(*hornet.Transaction))
		}

		task.Return(nil)
	}, workerpool.WorkerCount(gossipSolidifierWorkerCount), workerpool.QueueSize(gossipSolidifierQueueSize))

}

func runGossipSolidifier() {
	log.Info("Starting Solidifier ...")

	notifyNewTx := events.NewClosure(func(transaction *hornet.Transaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		if tangle.IsNodeSynced() {
			gossipSolidifierWorkerPool.Submit(transaction)
		}
	})

	daemon.BackgroundWorker("Tangle Solidifier", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Solidifier ... done")
		Events.ReceivedNewTransaction.Attach(notifyNewTx)
		gossipSolidifierWorkerPool.Start()
		<-shutdownSignal
		Events.ReceivedNewTransaction.Detach(notifyNewTx)
		gossipSolidifierWorkerPool.StopAndWait()

		log.Info("Stopping Solidifier ... done")
	}, shutdown.ShutdownPrioritySolidifierGossip)
}

// Checks and updates the solid flag of a transaction and its approvers (future cone).
func checkSolidityAndPropagate(transaction *hornet.Transaction) {

	txsToCheck := make(map[string]*hornet.Transaction)
	txsToCheck[transaction.GetHash()] = transaction

	// Loop as long as new transactions are added in every loop cycle
	for len(txsToCheck) != 0 {
		for txHash, tx := range txsToCheck {
			delete(txsToCheck, txHash)

			solid, _ := checkSolidity(tx, true)
			if solid {
				transactionApprovers, _ := tangle.GetApprovers(txHash)
				for _, approverHash := range transactionApprovers.GetHashes() {
					approver, _ := tangle.GetTransaction(approverHash)
					if approver != nil {
						txsToCheck[approverHash] = approver
					}
				}
			}
		}
	}
}
