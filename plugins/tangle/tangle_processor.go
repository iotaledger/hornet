package tangle

import (
	"fmt"
	"runtime"
	"sync"

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

	startWaitGroup sync.WaitGroup
)

func configureTangleProcessor(_ *node.Plugin) {

	receiveTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		processIncomingTx(task.Param(0).(*hornet.Transaction), task.Param(1).(*rqueue.Request), task.Param(2).(*peer.Peer))
		task.Return(nil)
	}, workerpool.WorkerCount(receiveTxWorkerCount), workerpool.QueueSize(receiveTxQueueSize))

	processValidMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		processValidMilestone(task.Param(0).(*tangle.CachedBundle)) // bundle pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(processValidMilestoneWorkerCount), workerpool.QueueSize(processValidMilestoneQueueSize), workerpool.FlushTasksAtShutdown(true))

	milestoneSolidifierWorkerPool = workerpool.New(func(task workerpool.Task) {
		solidifyMilestone(task.Param(0).(milestone.Index), task.Param(1).(bool))
		task.Return(nil)
	}, workerpool.WorkerCount(milestoneSolidifierWorkerCount), workerpool.QueueSize(milestoneSolidifierQueueSize))
}

func runTangleProcessor(_ *node.Plugin) {
	log.Info("Starting TangleProcessor ...")

	startWaitGroup.Add(4)

	onTransactionProcessed := events.NewClosure(func(transaction *hornet.Transaction, request *rqueue.Request, p *peer.Peer) {
		receiveTxWorkerPool.Submit(transaction, request, p)
	})

	onTPSMetricsUpdated := events.NewClosure(func(tpsMetrics *metricsplugin.TPSMetrics) {
		lastIncomingTPS = tpsMetrics.Incoming
		lastNewTPS = tpsMetrics.New
		lastOutgoingTPS = tpsMetrics.Outgoing
	})

	onReceivedValidMilestone := events.NewClosure(func(cachedBndl *tangle.CachedBundle) {
		_, added := processValidMilestoneWorkerPool.Submit(cachedBndl) // bundle pass +1
		if !added {
			// Release shouldn't be forced, to cache the latest milestones
			cachedBndl.Release() // bundle -1
		}
	})

	onReceivedInvalidMilestone := events.NewClosure(func(err error) {
		log.Info(err)
	})

	daemon.BackgroundWorker("TangleProcessor[UpdateMetrics]", func(shutdownSignal <-chan struct{}) {
		metricsplugin.Events.TPSMetricsUpdated.Attach(onTPSMetricsUpdated)
		startWaitGroup.Done()
		<-shutdownSignal
		metricsplugin.Events.TPSMetricsUpdated.Detach(onTPSMetricsUpdated)
	}, shutdown.PriorityMetricsUpdater)

	daemon.BackgroundWorker("TangleProcessor[ReceiveTx]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[ReceiveTx] ... done")
		gossip.Processor().Events.TransactionProcessed.Attach(onTransactionProcessed)
		receiveTxWorkerPool.Start()
		startWaitGroup.Done()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[ReceiveTx] ...")
		gossip.Processor().Events.TransactionProcessed.Detach(onTransactionProcessed)
		receiveTxWorkerPool.StopAndWait()
		log.Info("Stopping TangleProcessor[ReceiveTx] ... done")
	}, shutdown.PriorityReceiveTxWorker)

	daemon.BackgroundWorker("TangleProcessor[ProcessMilestone]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[ProcessMilestone] ... done")
		processValidMilestoneWorkerPool.Start()
		tangle.Events.ReceivedValidMilestone.Attach(onReceivedValidMilestone)
		tangle.Events.ReceivedInvalidMilestone.Attach(onReceivedInvalidMilestone)
		startWaitGroup.Done()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[ProcessMilestone] ...")
		tangle.Events.ReceivedValidMilestone.Detach(onReceivedValidMilestone)
		tangle.Events.ReceivedInvalidMilestone.Detach(onReceivedInvalidMilestone)
		processValidMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping TangleProcessor[ProcessMilestone] ... done")
	}, shutdown.PriorityMilestoneProcessor)

	daemon.BackgroundWorker("TangleProcessor[MilestoneSolidifier]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[MilestoneSolidifier] ... done")
		milestoneSolidifierWorkerPool.Start()
		startWaitGroup.Done()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[MilestoneSolidifier] ...")
		milestoneSolidifierWorkerPool.StopAndWait()
		log.Info("Stopping TangleProcessor[MilestoneSolidifier] ... done")
	}, shutdown.PriorityMilestoneSolidifier)

}

// WaitForTangleProcessorStartup waits until all background workers of the tangle processor are started.
func WaitForTangleProcessorStartup() {
	startWaitGroup.Wait()
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

	// "ProcessedTransaction" event has to be fired after "ReceivedNewTransaction" event,
	// otherwise there is a race condition in the coordinator plugin that tries to "ComputeMerkleTreeRootHash"
	// with the transactions it issued itself because the transactions may be not solid yet and therefore their bundles
	// are not created yet.
	Events.ProcessedTransaction.Trigger(incomingTx.GetTxHash())

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
				"TPS (in/new/out): %05d/%05d/%05d, "+
				"Tips (non-/semi-lazy): %d/%d",
			queued, pending, processing, avgLatency,
			currentLowestMilestoneIndexInReqQ,
			receiveTxWorkerPool.GetPendingQueueSize(),
			tangle.GetSolidMilestoneIndex(),
			tangle.GetLatestMilestoneIndex(),
			lastIncomingTPS,
			lastNewTPS,
			lastOutgoingTPS,
			metrics.SharedServerMetrics.TipsNonLazy.Load(),
			metrics.SharedServerMetrics.TipsSemiLazy.Load()))
}
