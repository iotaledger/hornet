package spa

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/iota.go/transaction"

	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone"
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
			hub.BroadcastMsg(&msg{MsgTypeTx, &tx{x.Hash, x.Value}})
		case milestone.Index:
			if cachedTailTx := getMilestoneTail(x); cachedTailTx != nil { // tx +1
				hub.BroadcastMsg(&msg{MsgTypeMs, &ms{cachedTailTx.GetTransaction().GetHash(), x}})
				cachedTailTx.Release(true) // tx -1
			}
		}
		task.Return(nil)
	}, workerpool.WorkerCount(liveFeedWorkerCount), workerpool.QueueSize(liveFeedWorkerQueueSize))
}

func runLiveFeed() {

	newTxRateLimiter := time.NewTicker(time.Second / 10)

	notifyNewTx := events.NewClosure(func(cachedTx *tangle_model.CachedTransaction, firstSeenLatestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
		cachedTx.ConsumeTransaction(func(tx *hornet.Transaction, metadata *hornet.TransactionMetadata) {
			if !tangle_model.IsNodeSyncedWithThreshold() {
				return
			}
			select {
			case <-newTxRateLimiter.C:
				liveFeedWorkerPool.TrySubmit(tx.Tx)
			default:
			}
		})
	})

	notifyLMChanged := events.NewClosure(func(cachedBndl *tangle_model.CachedBundle) {
		liveFeedWorkerPool.TrySubmit(cachedBndl.GetBundle().GetMilestoneIndex())
		cachedBndl.Release(true) // bundle -1
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
