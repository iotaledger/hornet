package spa

import (
	"time"

	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	tangle_model "github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/packages/workerpool"
	"github.com/gohornet/hornet/plugins/tangle"
	daemon "github.com/iotaledger/hive.go/daemon/ordered"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/iota.go/transaction"
)

var liveFeedWorkerCount = 1
var liveFeedWorkerQueueSize = 50
var liveFeedWorkerPool *workerpool.WorkerPool
var newTxRateLimiter *time.Ticker

func configureLiveFeed() {
	newTxRateLimiter = time.NewTicker(time.Second / 10)
	liveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		switch x := task.Param(0).(type) {
		case *transaction.Transaction:
			<-newTxRateLimiter.C
			sendToAllWSClient(&msg{MsgTypeTx, &tx{x.Hash, x.Value}})
		case milestone_index.MilestoneIndex:
			if tailTx := getMilestone(x); tailTx != nil {
				sendToAllWSClient(&msg{MsgTypeMs, &ms{tailTx.GetHash(), x}})
			}
		}
		task.Return(nil)
	}, workerpool.WorkerCount(liveFeedWorkerCount), workerpool.QueueSize(liveFeedWorkerQueueSize))
}

func runLiveFeed() {

	notifyNewTx := events.NewClosure(func(transaction *hornet.Transaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		if !tangle_model.IsNodeSynced() {
			return
		}
		liveFeedWorkerPool.TrySubmit(transaction.Tx)
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
		if newTxRateLimiter != nil {
			newTxRateLimiter.Stop()
		}
		liveFeedWorkerPool.StopAndWait()
		log.Info("Stopping SPA[TxUpdater] ... done")
	}, shutdown.ShutdownPrioritySPA)
}
