package tangle

import (
	"fmt"
	"runtime"

	"go.uber.org/atomic"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/gossip/server"
	"github.com/gohornet/hornet/plugins/metrics"
)

var (
	receiveTxWorkerCount = 2 * runtime.NumCPU()
	receiveTxQueueSize   = 10000
	receiveTxWorkerPool  *workerpool.WorkerPool

	lastIncomingTPS uint32
	lastNewTPS      uint32
	lastOutgoingTPS uint32

	seenSpentAddrs   atomic.Uint64
	bundlesValidated atomic.Uint64
)

func configureTangleProcessor(plugin *node.Plugin) {

	configureGossipSolidifier()
	configurePersisters()

	receiveTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		processIncomingTx(plugin, task.Param(0).(*hornet.Transaction))
		task.Return(nil)
	}, workerpool.WorkerCount(receiveTxWorkerCount), workerpool.QueueSize(receiveTxQueueSize))

	processValidMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		processValidMilestone(task.Param(0).(*tangle.CachedBundle)) // bundle pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(processValidMilestoneWorkerCount), workerpool.QueueSize(processValidMilestoneQueueSize), workerpool.FlushTasksAtShutdown(true))

	milestoneSolidifierWorkerPool = workerpool.New(func(task workerpool.Task) {
		solidifyMilestone(task.Param(0).(milestone_index.MilestoneIndex), task.Param(1).(bool), task.Param(2).(bool))
		task.Return(nil)
	}, workerpool.WorkerCount(milestoneSolidifierWorkerCount), workerpool.QueueSize(milestoneSolidifierQueueSize))

	metrics.Events.TPSMetricsUpdated.Attach(events.NewClosure(func(tpsMetrics *metrics.TPSMetrics) {
		lastIncomingTPS = tpsMetrics.Incoming
		lastNewTPS = tpsMetrics.New
		lastOutgoingTPS = tpsMetrics.Outgoing
	}))

	Events.TransactionSolid.Attach(events.NewClosure(onTransactionSolidEvent))
	tangle.Events.ReceivedValidMilestone.Attach(events.NewClosure(onReceivedValidMilestone))
	tangle.Events.ReceivedInvalidMilestone.Attach(events.NewClosure(onReceivedInvalidMilestone))
}

func runTangleProcessor(plugin *node.Plugin) {
	log.Info("Starting TangleProcessor ...")

	runGossipSolidifier()
	runPersisters()

	notifyReceivedTx := events.NewClosure(func(transaction *hornet.Transaction) {
		receiveTxWorkerPool.Submit(transaction)
	})

	daemon.BackgroundWorker("TangleProcessor[ReceiveTx]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[ReceiveTx] ... done")
		gossip.Events.ReceivedTransaction.Attach(notifyReceivedTx)
		receiveTxWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[ReceiveTx] ...")
		gossip.Events.ReceivedTransaction.Detach(notifyReceivedTx)
		receiveTxWorkerPool.StopAndWait()
		log.Info("Stopping TangleProcessor[ReceiveTx] ... done")
	}, shutdown.ShutdownPriorityReceiveTxWorker)

	daemon.BackgroundWorker("TangleProcessor[ProcessMilestone]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[ProcessMilestone] ... done")
		processValidMilestoneWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[ProcessMilestone] ...")
		processValidMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping TangleProcessor[ProcessMilestone] ... done")
	}, shutdown.ShutdownPriorityMilestoneProcessor)

	daemon.BackgroundWorker("TangleProcessor[MilestoneSolidifier]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[MilestoneSolidifier] ... done")
		milestoneSolidifierWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[MilestoneSolidifier] ...")
		milestoneSolidifierWorkerPool.StopAndWait()
		log.Info("Stopping TangleProcessor[MilestoneSolidifier] ... done")
	}, shutdown.ShutdownPriorityMilestoneSolidifier)
}

func processIncomingTx(plugin *node.Plugin, incomingTx *hornet.Transaction) {

	txHash := incomingTx.GetHash()
	cachedTx := tangle.GetCachedTransaction(txHash) // tx +1
	defer cachedTx.Release()                        // tx -1

	requested, reqMilestoneIndex := incomingTx.IsRequested()

	// The tx will be added to the storage inside this function, so the transaction object automatically updates
	alreadyAdded := tangle.AddTransactionToStorage(incomingTx)
	if !alreadyAdded {
		if requested {
			// Add new requests to the requestQueue (needed for sync)
			gossip.RequestApprovees(cachedTx.Retain(), reqMilestoneIndex) // tx pass +1
		}

		server.SharedServerMetrics.IncrNewTransactionsCount()

		addressPersisterSubmit(cachedTx.GetTransaction().Tx.Address, cachedTx.GetTransaction().GetHash())
		latestMilestoneIndex := tangle.GetLatestMilestoneIndex()
		solidMilestoneIndex := tangle.GetSolidMilestoneIndex()
		if latestMilestoneIndex == 0 {
			latestMilestoneIndex = solidMilestoneIndex
		}
		Events.ReceivedNewTransaction.Trigger(cachedTx, latestMilestoneIndex, solidMilestoneIndex)

	} else {
		Events.ReceivedKnownTransaction.Trigger(cachedTx)
	}

	if requested {
		gossip.RequestQueue.MarkProcessed(txHash)
	}

	if !tangle.IsNodeSynced() && gossip.RequestQueue.IsEmpty() {
		// The node is not synced, but the request queue seems empty => trigger the solidifer
		milestoneSolidifierWorkerPool.TrySubmit(milestone_index.MilestoneIndex(0), false, true)
	}
}

func onTransactionSolidEvent(cachedTx *tangle.CachedTransaction) {
	if cachedTx.GetTransaction().IsTail() {
		tangle.OnTailTransactionSolid(cachedTx.Retain()) // tx pass +1
	}
	cachedTx.Release() // tx -1
}

func onReceivedValidMilestone(cachedBndl *tangle.CachedBundle) {
	_, added := processValidMilestoneWorkerPool.Submit(cachedBndl) // bundle pass +1
	if !added {
		cachedBndl.Release() // bundle -1
	}
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
				"processor: %05d, "+
				"LSMI/LMI: %d/%d, "+
				"seenSpentAddrs: %d, "+
				"bndlsValidated: %d, "+
				"txReqs(Tx/Rx): %d/%d, "+
				"newTxs: %d, "+
				"TPS: %d (in) / %d (new) / %d (out)",
			requestCount,
			requestedMilestone,
			receiveTxWorkerPool.GetPendingQueueSize(),
			tangle.GetSolidMilestoneIndex(),
			tangle.GetLatestMilestoneIndex(),
			seenSpentAddrs.Load(),
			bundlesValidated.Load(),
			server.SharedServerMetrics.GetSentTransactionRequestCount(),
			server.SharedServerMetrics.GetReceivedTransactionRequestCount(),
			server.SharedServerMetrics.GetNewTransactionsCount(),
			lastIncomingTPS,
			lastNewTPS,
			lastOutgoingTPS))
}
