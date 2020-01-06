package spa

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/iota.go/transaction"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	tangle_model "github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/tangle"
)

var liveFeedWorkerCount = 1
var liveFeedWorkerQueueSize = 50
var liveFeedWorkerPool *workerpool.WorkerPool

func configureLiveFeed() {
	liveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		switch x := task.Param(0).(type) {
		case *transaction.Transaction:
			sendToAllWSClient(&msg{MsgTypeTx, &tx{x.Hash, x.Value}})
		case milestone_index.MilestoneIndex:
			if tailTx := getMilestone(x); tailTx != nil {
				sendToAllWSClient(&msg{MsgTypeMs, &ms{tailTx.GetTransaction().GetHash(), x}})
				tailTx.Release()
			}
		}
		task.Return(nil)
	}, workerpool.WorkerCount(liveFeedWorkerCount), workerpool.QueueSize(liveFeedWorkerQueueSize))
}

func runLiveFeed() {

	newTxRateLimiter := time.NewTicker(time.Second / 10)

	notifyNewTx := events.NewClosure(func(transaction *tangle_model.CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		if !tangle_model.IsNodeSynced() {
			return
		}
		select {
		case <-newTxRateLimiter.C:
			liveFeedWorkerPool.TrySubmit(transaction.GetTransaction().Tx)
		default:
		}
	})

	notifyLMChanged := events.NewClosure(func(bndl *tangle_model.Bundle) {
		liveFeedWorkerPool.TrySubmit(bndl.GetMilestoneIndex())
	})

	daemon.BackgroundWorker("SPA[TxUpdater]", func(shutdownSignal <-chan struct{}) {
		tangle.Events.ReceivedNewTransaction.Attach(notifyNewTx)
		tangle.Events.LatestMilestoneChanged.Attach(notifyLMChanged)
		liveFeedWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping SPA[TxUpdater] ...")
		tangle.Events.ReceivedNewTransaction.Detach(notifyNewTx)
		tangle.Events.LatestMilestoneChanged.Detach(notifyLMChanged)
		newTxRateLimiter.Stop()
		liveFeedWorkerPool.StopAndWait()
		log.Info("Stopping SPA[TxUpdater] ... done")
	}, shutdown.ShutdownPrioritySPA)
}
