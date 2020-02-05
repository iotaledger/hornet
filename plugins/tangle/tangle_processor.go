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

	markedSpentAddrs atomic.Uint64
	bundlesValidated atomic.Uint64
)

func configureTangleProcessor(plugin *node.Plugin) {

	configureGossipSolidifier()
	configurePersisters()

	receiveTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		processIncomingTx(plugin, task.Param(0).(*hornet.Transaction))
		task.Return(nil)
	}, workerpool.WorkerCount(receiveTxWorkerCount), workerpool.QueueSize(receiveTxQueueSize))

	checkForMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		checkBundleForMilestone(task.Param(0).(*tangle.Bundle))
		task.Return(nil)
	}, workerpool.WorkerCount(checkForMilestoneWorkerCount), workerpool.QueueSize(checkForMilestoneQueueSize), workerpool.FlushTasksAtShutdown(true))

	milestoneSolidifierWorkerPool = workerpool.New(func(task workerpool.Task) {
		solidifyMilestone(task.Param(0).(milestone_index.MilestoneIndex))
		task.Return(nil)
	}, workerpool.WorkerCount(milestoneSolidifierWorkerCount), workerpool.QueueSize(milestoneSolidifierQueueSize))

	metrics.Events.TPSMetricsUpdated.Attach(events.NewClosure(func(tpsMetrics *metrics.TPSMetrics) {
		lastIncomingTPS = tpsMetrics.Incoming
		lastNewTPS = tpsMetrics.New
		lastOutgoingTPS = tpsMetrics.Outgoing
	}))
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

	daemon.BackgroundWorker("TangleProcessor[CheckForMilestone]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[CheckForMilestone] ... done")
		checkForMilestoneWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[CheckForMilestone] ...")
		checkForMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping TangleProcessor[CheckForMilestone] ... done")
	}, shutdown.ShutdownPriorityMilestoneChecker)

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
	transaction := tangle.GetCachedTransaction(txHash) //+1

	// The tx will be added to the storage inside this function, so the transaction object automatically updates
	bundlesAddedTo, alreadyAdded := addTransactionToBundleBucket(incomingTx)

	if !alreadyAdded {

		if !transaction.Exists() {
			log.Panic("Transaction should have been added to storage!")
		}

		server.SharedServerMetrics.IncrNewTransactionsCount()

		addressPersisterSubmit(transaction.GetTransaction().Tx.Address, transaction.GetTransaction().GetHash())
		latestMilestoneIndex := tangle.GetLatestMilestoneIndex()
		solidMilestoneIndex := tangle.GetSolidMilestoneIndex()
		if latestMilestoneIndex == 0 {
			latestMilestoneIndex = solidMilestoneIndex
		}
		Events.ReceivedNewTransaction.Trigger(transaction, latestMilestoneIndex, solidMilestoneIndex)

		tangle.StoreApprover(transaction.GetTransaction().GetTrunk(), transaction.GetTransaction().GetHash()).Release()
		tangle.StoreApprover(transaction.GetTransaction().GetBranch(), transaction.GetTransaction().GetHash()).Release()

		for _, bundle := range bundlesAddedTo {
			// this iteration might be true concurrently between different processIncomingTx()
			// for the same bundle instance and bucket
			if bundle.IsComplete() {

				// validate the bundle
				if bundle.IsValid() {
					// in a value spam bundle, the address' mutation to the ledger is zero,
					// thereby it is sufficient to simply check for negative balance mutations
					// while iterating over the ledger changes for this bundle
					ledgerChanges, isZeroValueBundle := bundle.GetLedgerChanges()
					if !isZeroValueBundle {
						for addr, change := range ledgerChanges {
							if change < 0 {
								tangle.MarkAddressAsSpent(addr)
								markedSpentAddrs.Inc()
								Events.AddressSpent.Trigger(addr)
							}
						}
					} else {
						// Milestone bundles itself do not mutate the ledger
						// => Check bundle for a milestone
						checkForMilestoneWorkerPool.Submit(bundle)
					}
				}
				bundlesValidated.Inc()
			}
		}
	} else {
		Events.ReceivedKnownTransaction.Trigger(transaction)
	}

	if transaction.GetTransaction().IsRequested() {
		// Add new requests to the requestQueue (needed for sync)
		gossip.RequestApprovees(transaction.Retain()) //Pass +1
	}

	transaction.Release() //-1

	queueEmpty := gossip.RequestQueue.MarkProcessed(txHash)
	if queueEmpty {
		milestoneSolidifierWorkerPool.TrySubmit(milestone_index.MilestoneIndex(0))
	}
}

func printStatus() {
	requestedMilestone, requestCount := gossip.RequestQueue.CurrentMilestoneIndexAndSize()

	println(
		fmt.Sprintf(
			"reqQ: %05d, "+
				"reqQMs: %d, "+
				"processor: %05d, "+
				"LSMI/LMI: %d/%d, "+
				"addrsMarked: %d, "+
				"bndlsValidated: %d, "+
				"txReqs(Tx/Rx): %d/%d, "+
				"newTxs: %d, "+
				"TPS: %d (in) / %d (new) / %d (out)",
			requestCount,
			requestedMilestone,
			receiveTxWorkerPool.GetPendingQueueSize(),
			tangle.GetSolidMilestoneIndex(),
			tangle.GetLatestMilestoneIndex(),
			markedSpentAddrs.Load(),
			bundlesValidated.Load(),
			server.SharedServerMetrics.GetSentTransactionRequestCount(),
			server.SharedServerMetrics.GetReceivedTransactionRequestCount(),
			server.SharedServerMetrics.GetNewTransactionsCount(),
			lastIncomingTPS,
			lastNewTPS,
			lastOutgoingTPS))
}
