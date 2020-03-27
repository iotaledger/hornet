package spa

import (
	"time"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"

	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	tangle_model "github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/tangle"
)

var (
	liveFeedWorkerPool = (&async.NonBlockingWorkerPool{}).Tune(1)
)

func runLiveFeed() {

	newTxRateLimiter := time.NewTicker(time.Second / 10)

	notifyNewTx := events.NewClosure(func(cachedTx *tangle_model.CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		cachedTx.ConsumeTransaction(func(transaction *hornet.Transaction, metadata *hornet.TransactionMetadata) {
			if !tangle_model.IsNodeSyncedWithThreshold() {
				return
			}
			select {
			case <-newTxRateLimiter.C:
				liveFeedWorkerPool.Submit(func() { hub.BroadcastMsg(&msg{MsgTypeTx, &tx{transaction.Tx.Hash, transaction.Tx.Value}}) })
			default:
			}
		})
	})

	notifyLMChanged := events.NewClosure(func(cachedBndl *tangle_model.CachedBundle) {
		if added := liveFeedWorkerPool.Submit(func() {
			if cachedTailTx := cachedBndl.GetBundle().GetTail(); cachedTailTx != nil { // tx +1
				hub.BroadcastMsg(&msg{MsgTypeMs, &ms{cachedTailTx.GetTransaction().GetHash(), cachedBndl.GetBundle().GetMilestoneIndex()}})
				cachedTailTx.Release(true) // tx -1
			}
			cachedBndl.Release(true) // bundle -1
		}); added {
			return // Avoid bundle -1 (done inside workerpool task)
		}
		cachedBndl.Release(true) // bundle -1
	})

	daemon.BackgroundWorker("SPA[TxUpdater]", func(shutdownSignal <-chan struct{}) {
		tangle.Events.ReceivedNewTransaction.Attach(notifyNewTx)
		tangle.Events.LatestMilestoneChanged.Attach(notifyLMChanged)
		<-shutdownSignal
		log.Info("Stopping SPA[TxUpdater] ...")
		tangle.Events.ReceivedNewTransaction.Detach(notifyNewTx)
		tangle.Events.LatestMilestoneChanged.Detach(notifyLMChanged)
		newTxRateLimiter.Stop()
		liveFeedWorkerPool.Shutdown()
		log.Info("Stopping SPA[TxUpdater] ... done")
	}, shutdown.ShutdownPrioritySPA)
}
