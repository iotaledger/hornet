package tangle

import (
	"fmt"
	"runtime"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/packages/metrics"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/peering/peer"
	"github.com/gohornet/hornet/packages/protocol/rqueue"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/gossip"
	metrics_plugin "github.com/gohornet/hornet/plugins/metrics"
)

var (
	receiveTxWorkerCount = 2 * runtime.NumCPU()
	receiveTxQueueSize   = 10000
	receiveTxWorkerPool  *workerpool.WorkerPool

	lastIncomingTPS uint64
	lastNewTPS      uint64
	lastOutgoingTPS uint64
)

func configureTangleProcessor(plugin *node.Plugin) {

	configureGossipSolidifier()

	receiveTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		processIncomingTx(plugin, task.Param(0).(*hornet.Transaction), task.Param(1).(*rqueue.Request), task.Param(2).(*peer.Peer))
		task.Return(nil)
	}, workerpool.WorkerCount(receiveTxWorkerCount), workerpool.QueueSize(receiveTxQueueSize))

	processValidMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		processValidMilestone(task.Param(0).(*tangle.CachedBundle)) // bundle pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(processValidMilestoneWorkerCount), workerpool.QueueSize(processValidMilestoneQueueSize), workerpool.FlushTasksAtShutdown(true))

	milestoneSolidifierWorkerPool = workerpool.New(func(task workerpool.Task) {
		if daemon.IsStopped() {
			return
		}
		solidifyMilestone(task.Param(0).(milestone.Index), task.Param(1).(bool))
		task.Return(nil)
	}, workerpool.WorkerCount(milestoneSolidifierWorkerCount), workerpool.QueueSize(milestoneSolidifierQueueSize))

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

	submitReceivedTxForProcessing := events.NewClosure(func(transaction *hornet.Transaction, request *rqueue.Request, p *peer.Peer) {
		receiveTxWorkerPool.Submit(transaction, request, p)
	})

	daemon.BackgroundWorker("TangleProcessor[ReceiveTx]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[ReceiveTx] ... done")
		gossip.Processor().Events.TransactionProcessed.Attach(submitReceivedTxForProcessing)
		receiveTxWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[ReceiveTx] ...")
		gossip.Processor().Events.TransactionProcessed.Detach(submitReceivedTxForProcessing)
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

func processIncomingTx(plugin *node.Plugin, incomingTx *hornet.Transaction, request *rqueue.Request, p *peer.Peer) {

	txHash := incomingTx.GetHash()

	latestMilestoneIndex := tangle.GetLatestMilestoneIndex()
	isNodeSyncedWithThreshold := tangle.IsNodeSyncedWithThreshold()

	// The tx will be added to the storage inside this function, so the transaction object automatically updates
	cachedTx, alreadyAdded := tangle.AddTransactionToStorage(incomingTx, latestMilestoneIndex, request != nil, !isNodeSyncedWithThreshold, false) // tx +1

	// Release shouldn't be forced, to cache the latest transactions
	defer cachedTx.Release(!isNodeSyncedWithThreshold) // tx -1

	if !alreadyAdded {
		metrics.SharedServerMetrics.NewTransactions.Inc()

		if p != nil {
			p.Metrics.NewTransactions.Inc()
		}

		// since we only add the approvees if there was a source request, we only
		// request them for transactions which should be part of milestone cones
		if request != nil {
			// add this newly received transaction's approvees to the request queue
			gossip.RequestApprovees(cachedTx.Retain(), request.MilestoneIndex)
		}

		solidMilestoneIndex := tangle.GetSolidMilestoneIndex()
		if latestMilestoneIndex == 0 {
			latestMilestoneIndex = solidMilestoneIndex
		}
		Events.ReceivedNewTransaction.Trigger(cachedTx, latestMilestoneIndex, solidMilestoneIndex)

	} else {
		metrics.SharedServerMetrics.KnownTransactions.Inc()
		if p != nil {
			p.Metrics.KnownTransactions.Inc()
		}
		Events.ReceivedKnownTransaction.Trigger(cachedTx)
	}

	// mark the transaction as received if it originated from a request we made
	if request != nil {
		gossip.RequestQueue().Received(txHash)
	}

	if !tangle.IsNodeSynced() && gossip.RequestQueue().Empty() {
		// we trigger the milestone solidifier in order to solidify milestones
		// which should be solid given that the request queue is empty
		milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), false)
	}
}

func onReceivedValidMilestone(cachedBndl *tangle.CachedBundle) {
	_, added := processValidMilestoneWorkerPool.Submit(cachedBndl) // bundle pass +1
	if !added {
		// Release shouldn't be forced, to cache the latest milestones
		cachedBndl.Release() // bundle -1
	}
}

func onReceivedInvalidMilestone(err error) {
	log.Info(err)
}

func printStatus() {
	var currentLowestMilestoneIndexInReqQ milestone.Index
	if peekedRequest := gossip.RequestQueue().Peek(); peekedRequest != nil {
		currentLowestMilestoneIndexInReqQ = peekedRequest.MilestoneIndex
	}

	queued, pending := gossip.RequestQueue().Size()
	avgLatency := gossip.RequestQueue().AvgLatency()

	println(
		fmt.Sprintf(
			"reqQ(q/p/l): %d/%d/%dms, "+
				"reqQMs: %d, "+
				"processor: %05d, "+
				"LSMI/LMI: %d/%d, "+
				"seenSpentAddrs: %d, "+
				"bndlsValidated: %d, "+
				"txReqs(Tx/Rx): %d/%d, "+
				"newTxs: %d, "+
				"TPS: %d (in) / %d (new) / %d (out)",
			queued, pending, avgLatency,
			currentLowestMilestoneIndexInReqQ,
			receiveTxWorkerPool.GetPendingQueueSize(),
			tangle.GetSolidMilestoneIndex(),
			tangle.GetLatestMilestoneIndex(),
			metrics.SharedServerMetrics.SeenSpentAddresses.Load(),
			metrics.SharedServerMetrics.ValidatedBundles.Load(),
			metrics.SharedServerMetrics.SentTransactionRequests.Load(),
			metrics.SharedServerMetrics.ReceivedTransactionRequests.Load(),
			metrics.SharedServerMetrics.NewTransactions.Load(),
			lastIncomingTPS,
			lastNewTPS,
			lastOutgoingTPS))
}
