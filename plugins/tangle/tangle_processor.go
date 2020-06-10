package tangle

import (
	"fmt"
	"runtime"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol/rqueue"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/gossip"
	metricsplugin "github.com/gohornet/hornet/plugins/metrics"
)

var (
	receiveTxWorkerCount = 2 * runtime.NumCPU()
	receiveTxQueueSize   = 10000
	receiveTxWorkerPool  *workerpool.WorkerPool

	lastIncomingTPS uint32
	lastNewTPS      uint32
	lastOutgoingTPS uint32
)

func configureTangleProcessor(_ *node.Plugin) {

	configureGossipSolidifier()

	receiveTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		processIncomingTx(task.Param(0).(*hornet.Transaction), task.Param(1).(*rqueue.Request), task.Param(2).(*peer.Peer))
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

	metricsplugin.Events.TPSMetricsUpdated.Attach(events.NewClosure(func(tpsMetrics *metricsplugin.TPSMetrics) {
		lastIncomingTPS = tpsMetrics.Incoming
		lastNewTPS = tpsMetrics.New
		lastOutgoingTPS = tpsMetrics.Outgoing
	}))

	tangle.Events.ReceivedValidMilestone.Attach(events.NewClosure(onReceivedValidMilestone))
	tangle.Events.ReceivedInvalidMilestone.Attach(events.NewClosure(onReceivedInvalidMilestone))
}

func runTangleProcessor(_ *node.Plugin) {
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
	}, shutdown.PriorityReceiveTxWorker)

	daemon.BackgroundWorker("TangleProcessor[ProcessMilestone]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[ProcessMilestone] ... done")
		processValidMilestoneWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[ProcessMilestone] ...")
		processValidMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping TangleProcessor[ProcessMilestone] ... done")
	}, shutdown.PriorityMilestoneProcessor)

	daemon.BackgroundWorker("TangleProcessor[MilestoneSolidifier]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[MilestoneSolidifier] ... done")
		milestoneSolidifierWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[MilestoneSolidifier] ...")
		milestoneSolidifierWorkerPool.StopAndWait()
		log.Info("Stopping TangleProcessor[MilestoneSolidifier] ... done")
	}, shutdown.PriorityMilestoneSolidifier)
}

func IsReceiveTxWorkerPoolBusy() bool {
	return receiveTxWorkerPool.GetPendingQueueSize() > (receiveTxQueueSize / 2)
}

func processIncomingTx(incomingTx *hornet.Transaction, request *rqueue.Request, p *peer.Peer) {

	latestMilestoneIndex := tangle.GetLatestMilestoneIndex()
	isNodeSyncedWithThreshold := tangle.IsNodeSyncedWithThreshold()

	// The tx will be added to the storage inside this function, so the transaction object automatically updates
	cachedTx, alreadyAdded := tangle.AddTransactionToStorage(incomingTx, latestMilestoneIndex, request != nil, !isNodeSyncedWithThreshold, false) // tx +1

	// Release shouldn't be forced, to cache the latest transactions
	defer cachedTx.Release(!isNodeSyncedWithThreshold) // tx -1

	Events.ProcessedTransaction.Trigger(incomingTx.GetTxHash())

	if !alreadyAdded {
		metrics.SharedServerMetrics.NewTransactions.Inc()

		if p != nil {
			p.Metrics.NewTransactions.Inc()
		}

		// since we only add the approvees if there was a source request, we only
		// request them for transactions which should be part of milestone cones
		if request != nil {
			// add this newly received transaction's approvees to the request queue
			gossip.RequestApprovees(cachedTx.Retain(), request.MilestoneIndex, true)
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

	if request != nil {
		// mark the received request as processed
		gossip.RequestQueue().Processed(incomingTx.GetTxHash())
	}

	// we check whether the request is nil, so we only trigger the solidifier when
	// we actually handled a transaction stemming from a request (as otherwise the solidifier
	// is triggered too often through transactions received from normal gossip)
	if !tangle.IsNodeSynced() && request != nil && gossip.RequestQueue().Empty() {
		// we trigger the milestone solidifier in order to solidify milestones
		// which should be solid given that the request queue is empty
		milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), true)
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

	queued, pending, processing := gossip.RequestQueue().Size()
	avgLatency := gossip.RequestQueue().AvgLatency()

	println(
		fmt.Sprintf(
			"req(qu/pe/proc/lat): %05d/%05d/%05d/%04dms, "+
				"reqQMs: %d, "+
				"processor: %05d, "+
				"LSMI/LMI: %d/%d, "+
				"TPS (in/new/out): %05d/%05d/%05d",
			queued, pending, processing, avgLatency,
			currentLowestMilestoneIndexInReqQ,
			receiveTxWorkerPool.GetPendingQueueSize(),
			tangle.GetSolidMilestoneIndex(),
			tangle.GetLatestMilestoneIndex(),
			lastIncomingTPS,
			lastNewTPS,
			lastOutgoingTPS))
}
