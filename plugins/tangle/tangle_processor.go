package tangle

import (
	"fmt"
	"runtime"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/packages/metrics"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/gossip"
	metrics_plugin "github.com/gohornet/hornet/plugins/metrics"
)

var (
	receiveTxWorkerPool = (&async.WorkerPool{}).Tune(2 * runtime.NumCPU())

	lastIncomingTPS uint32
	lastNewTPS      uint32
	lastOutgoingTPS uint32
)

func configureTangleProcessor(plugin *node.Plugin) {

	metrics_plugin.Events.TPSMetricsUpdated.Attach(events.NewClosure(func(tpsMetrics *metrics_plugin.TPSMetrics) {
		lastIncomingTPS = tpsMetrics.Incoming
		lastNewTPS = tpsMetrics.New
		lastOutgoingTPS = tpsMetrics.Outgoing
	}))

	tangle.Events.ReceivedValidMilestone.Attach(events.NewClosure(onReceivedValidMilestone))
	tangle.Events.ReceivedInvalidMilestone.Attach(events.NewClosure(onReceivedInvalidMilestone))
}

func runTangleProcessor(plugin *node.Plugin) {
	log.Info("Starting TangleProcessor ...")

	runGossipSolidifier()

	notifyReceivedTx := events.NewClosure(func(transaction *hornet.Transaction, requested bool, reqMilestoneIndex milestone_index.MilestoneIndex, neighborMetrics *metrics.NeighborMetrics) {
		receiveTxWorkerPool.Submit(func() { processIncomingTx(transaction, requested, reqMilestoneIndex, neighborMetrics) })
	})

	daemon.BackgroundWorker("TangleProcessor[ReceiveTx]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[ReceiveTx] ... done")
		gossip.Events.ReceivedTransaction.Attach(notifyReceivedTx)
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[ReceiveTx] ...")
		gossip.Events.ReceivedTransaction.Detach(notifyReceivedTx)
		receiveTxWorkerPool.Shutdown()
		log.Info("Stopping TangleProcessor[ReceiveTx] ... done")
	}, shutdown.ShutdownPriorityReceiveTxWorker)

	daemon.BackgroundWorker("TangleProcessor[ProcessMilestone]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[ProcessMilestone] ... done")
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[ProcessMilestone] ...")
		processValidMilestoneWorkerPool.ShutdownGracefully()
		log.Info("Stopping TangleProcessor[ProcessMilestone] ... done")
	}, shutdown.ShutdownPriorityMilestoneProcessor)

	daemon.BackgroundWorker("TangleProcessor[MilestoneSolidifier]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[MilestoneSolidifier] ... done")
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[MilestoneSolidifier] ...")
		milestoneSolidifierWorkerPool.Shutdown()
		log.Info("Stopping TangleProcessor[MilestoneSolidifier] ... done")
	}, shutdown.ShutdownPriorityMilestoneSolidifier)
}

func processSolidificationTask(newMilestoneIndex milestone_index.MilestoneIndex, force bool) {
	if daemon.IsStopped() {
		return
	}
	solidifyMilestone(newMilestoneIndex, force)
}

func processIncomingTx(incomingTx *hornet.Transaction, requested bool, reqMilestoneIndex milestone_index.MilestoneIndex, neighborMetrics *metrics.NeighborMetrics) {

	txHash := incomingTx.GetHash()

	latestMilestoneIndex := tangle.GetLatestMilestoneIndex()
	isNodeSyncedWithThreshold := tangle.IsNodeSyncedWithThreshold()

	// The tx will be added to the storage inside this function, so the transaction object automatically updates
	cachedTx, alreadyAdded := tangle.AddTransactionToStorage(incomingTx, latestMilestoneIndex, requested, !isNodeSyncedWithThreshold, false) // tx +1

	// Release shouldn't be forced, to cache the latest transactions
	defer cachedTx.Release(!isNodeSyncedWithThreshold) // tx -1

	if !alreadyAdded {
		metrics.SharedServerMetrics.IncrNewTransactionsCount()
		if neighborMetrics != nil {
			neighborMetrics.IncrNewTransactionsCount()
		}

		if requested {
			// Add new requests to the requestQueue (needed for sync)
			gossip.RequestApprovees(cachedTx.Retain(), reqMilestoneIndex) // tx pass +1
		}

		solidMilestoneIndex := tangle.GetSolidMilestoneIndex()
		if latestMilestoneIndex == 0 {
			latestMilestoneIndex = solidMilestoneIndex
		}
		Events.ReceivedNewTransaction.Trigger(cachedTx, latestMilestoneIndex, solidMilestoneIndex)

	} else {
		metrics.SharedServerMetrics.IncrKnownTransactionsCount()
		if neighborMetrics != nil {
			neighborMetrics.IncrKnownTransactionsCount()
		}
		Events.ReceivedKnownTransaction.Trigger(cachedTx)
	}

	if requested {
		gossip.RequestQueue.MarkProcessed(txHash)
	}

	if !tangle.IsNodeSynced() && gossip.RequestQueue.IsEmpty() {
		// The node is not synced, but the request queue seems empty => trigger the solidifer
		milestoneSolidifierWorkerPool.Submit(func() { processSolidificationTask(milestone_index.MilestoneIndex(0), false) })
	}
}

func onReceivedValidMilestone(cachedBndl *tangle.CachedBundle) {
	processValidMilestoneWorkerPool.Submit(func() { processValidMilestone(cachedBndl) }) // bundle pass +1
}

func onReceivedInvalidMilestone(err error) {
	log.Info(err)
}

func printStatus() {
	requestedMilestone, requestCount := gossip.RequestQueue.CurrentMilestoneIndexAndSize()

	println(
		fmt.Sprintf(
			"reqQ: %05d, "+
				"reqQMs: %d, "+
				"processor: %d/%d, "+
				"LSMI/LMI: %d/%d, "+
				"seenSpentAddrs: %d, "+
				"bndlsValidated: %d, "+
				"txReqs(Tx/Rx): %d/%d, "+
				"newTxs: %d, "+
				"TPS: %d (in) / %d (new) / %d (out)",
			requestCount,
			requestedMilestone,
			receiveTxWorkerPool.RunningWorkers(), receiveTxWorkerPool.Capacity(),
			tangle.GetSolidMilestoneIndex(),
			tangle.GetLatestMilestoneIndex(),
			metrics.SharedServerMetrics.GetSeenSpentAddrCount(),
			metrics.SharedServerMetrics.GetValidatedBundlesCount(),
			metrics.SharedServerMetrics.GetSentTransactionRequestsCount(),
			metrics.SharedServerMetrics.GetReceivedTransactionRequestsCount(),
			metrics.SharedServerMetrics.GetNewTransactionsCount(),
			lastIncomingTPS,
			lastNewTPS,
			lastOutgoingTPS))
}
