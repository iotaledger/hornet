package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/batchworkerpool"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
)

var (
	addressPersisterWorkerCount            = 1
	addressPersisterQueueSize              = 10000
	addressPersisterBatchSize              = 1000
	addressPersisterBatchCollectionTimeout = 1000 * time.Millisecond
	addressPersisterWorkerPool             *batchworkerpool.BatchWorkerPool

	firstSeenTxWorkerCount            = 1
	firstSeenTxQueueSize              = 10000
	firstSeenTxBatchSize              = 1000
	firstSeenTxBatchCollectionTimeout = 1000 * time.Millisecond
	firstSeenTxWorkerPool             *batchworkerpool.BatchWorkerPool
)

func configurePersisters() {
	configureAddressPersister()
	configureFirstSeenTransactionPersister()
}

func runPersisters() {
	runAddressPersister()
	runFirstSeenTransactionPersister()
}

// Address persister
func configureAddressPersister() {

	// TxHash for Address persisting
	addressPersisterWorkerPool = batchworkerpool.New(func(tasks []batchworkerpool.Task) {

		var txHashesForAddresses []*tangle.TxHashForAddress
		for _, task := range tasks {
			txHashesForAddresses = append(txHashesForAddresses, task.Param(0).(*tangle.TxHashForAddress))
		}

		err := tangle.StoreTransactionHashesForAddressesInDatabase(txHashesForAddresses)
		if err != nil {
			panic(err)
		}

		for _, task := range tasks {
			task.Return(nil)
		}
	}, batchworkerpool.BatchCollectionTimeout(addressPersisterBatchCollectionTimeout), batchworkerpool.BatchSize(addressPersisterBatchSize), batchworkerpool.WorkerCount(addressPersisterWorkerCount), batchworkerpool.QueueSize(addressPersisterQueueSize), batchworkerpool.FlushTasksAtShutdown(true))
}

func runAddressPersister() {
	daemon.BackgroundWorker("AddressPersister", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting AddressPersister ... done")
		addressPersisterWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping AddressPersister ...")
		addressPersisterWorkerPool.StopAndWait()
		log.Info("Stopping AddressPersister ... done")
	}, shutdown.ShutdownPriorityPersisters)
}

func addressPersisterSubmit(address trinary.Hash, transactionHash trinary.Hash) {
	addressPersisterWorkerPool.Submit(&tangle.TxHashForAddress{Address: address, TxHash: transactionHash})
}

// FirstSeen Tx persister
func configureFirstSeenTransactionPersister() {

	firstSeenTxWorkerPool = batchworkerpool.New(func(tasks []batchworkerpool.Task) {

		var operations []*tangle.FirstSeenTxHashOperation
		for _, task := range tasks {
			operations = append(operations, task.Param(0).(*tangle.FirstSeenTxHashOperation))
		}

		err := tangle.StoreFirstSeenTxHashOperations(operations)
		if err != nil {
			panic(err)
		}

		for _, task := range tasks {
			task.Return(nil)
		}
	}, batchworkerpool.BatchCollectionTimeout(firstSeenTxBatchCollectionTimeout), batchworkerpool.BatchSize(firstSeenTxBatchSize), batchworkerpool.WorkerCount(firstSeenTxWorkerCount), batchworkerpool.QueueSize(firstSeenTxQueueSize), batchworkerpool.FlushTasksAtShutdown(true))
}

func runFirstSeenTransactionPersister() {

	notifyNewTx := events.NewClosure(func(transaction *tangle.CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		// Store only non-requested transactions, since all requested transactions are confirmed by a milestone anyway
		// This is only used to delete unconfirmed transactions from the database at pruning
		if !transaction.IsRequested() {
			firstSeenTxWorkerPool.Submit(&tangle.FirstSeenTxHashOperation{
				TxHash:                        transaction.GetTransaction().GetHash(),
				FirstSeenLatestMilestoneIndex: firstSeenLatestMilestoneIndex,
			})
		}
	})

	daemon.BackgroundWorker("FirstSeenTxPersister", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting FirstSeenTxPersister ... done")
		Events.ReceivedNewTransaction.Attach(notifyNewTx)
		firstSeenTxWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping FirstSeenTxPersister ...")
		Events.ReceivedNewTransaction.Detach(notifyNewTx)
		firstSeenTxWorkerPool.StopAndWait()
		log.Info("Stopping FirstSeenTxPersister ... done")
	}, shutdown.ShutdownPriorityPersisters)
}

// Tx stored event
func onEvictTransactions(evicted []*hornet.Transaction) {
	for _, tx := range evicted {
		Events.TransactionStored.Trigger(tx)
	}
}
