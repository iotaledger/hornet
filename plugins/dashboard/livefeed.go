package dashboard

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/iota.go/transaction"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	tanglemodel "github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/tangle"
)

var liveFeedWorkerCount = 1
var liveFeedWorkerQueueSize = 50
var liveFeedWorkerPool *workerpool.WorkerPool

func configureLiveFeed() {
	liveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		switch x := task.Param(0).(type) {
		case *transaction.Transaction:
			hub.BroadcastMsg(&msg{MsgTypeTx, &tx{x.Hash, x.Value}})
		case milestone.Index:
			if cachedTailTx := getMilestoneTail(x); cachedTailTx != nil { // tx +1
				hub.BroadcastMsg(&msg{MsgTypeMs, &ms{cachedTailTx.GetTransaction().Tx.Hash, x}})
				cachedTailTx.Release(true) // tx -1
			}
		}
		task.Return(nil)
	}, workerpool.WorkerCount(liveFeedWorkerCount), workerpool.QueueSize(liveFeedWorkerQueueSize))
}

func runLiveFeed() {

	newTxRateLimiter := time.NewTicker(time.Second / 10)

	notifyNewTx := events.NewClosure(func(cachedTx *tanglemodel.CachedTransaction, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
		cachedTx.ConsumeTransaction(func(tx *hornet.Transaction, metadata *hornet.TransactionMetadata) {
			if !tanglemodel.IsNodeSyncedWithThreshold() {
				return
			}
			select {
			case <-newTxRateLimiter.C:
				liveFeedWorkerPool.TrySubmit(tx.Tx)
			default:
			}
		})
	})

	notifyLMChanged := events.NewClosure(func(cachedBndl *tanglemodel.CachedBundle) {
		liveFeedWorkerPool.TrySubmit(cachedBndl.GetBundle().GetMilestoneIndex())
		cachedBndl.Release(true) // bundle -1
	})

	daemon.BackgroundWorker("Dashboard[TxUpdater]", func(shutdownSignal <-chan struct{}) {
		tangle.Events.ReceivedNewTransaction.Attach(notifyNewTx)
		defer tangle.Events.ReceivedNewTransaction.Detach(notifyNewTx)
		tangle.Events.LatestMilestoneChanged.Attach(notifyLMChanged)
		defer tangle.Events.LatestMilestoneChanged.Detach(notifyLMChanged)

		liveFeedWorkerPool.Start()
		<-shutdownSignal

		log.Info("Stopping Dashboard[TxUpdater] ...")
		newTxRateLimiter.Stop()
		liveFeedWorkerPool.StopAndWait()
		log.Info("Stopping Dashboard[TxUpdater] ... done")
	}, shutdown.PriorityDashboard)
}
