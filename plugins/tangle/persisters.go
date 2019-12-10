package tangle

import (
	"time"

	daemon "github.com/iotaledger/hive.go/daemon/ordered"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/gohornet/hornet/packages/batchworkerpool"
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

	unconfirmedTxWorkerCount            = 1
	unconfirmedTxQueueSize              = 10000
	unconfirmedTxBatchSize              = 1000
	unconfirmedTxBatchCollectionTimeout = 1000 * time.Millisecond
	unconfirmedTxWorkerPool             *batchworkerpool.BatchWorkerPool
)

func configurePersisters() {
	configureAddressPersister()
	configureUnconfirmedTransactionPersister()
}

func runPersisters() {
	runAddressPersister()
	runUnconfirmedTransactionPersister()
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
	}, batchworkerpool.BatchCollectionTimeout(addressPersisterBatchCollectionTimeout), batchworkerpool.BatchSize(addressPersisterBatchSize), batchworkerpool.WorkerCount(addressPersisterWorkerCount), batchworkerpool.QueueSize(addressPersisterQueueSize))
}

func runAddressPersister() {
	daemon.BackgroundWorker("AddressPersister", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting AddressPersister ... done")
		addressPersisterWorkerPool.Start()
		<-shutdownSignal
		addressPersisterWorkerPool.StopAndWait()
		log.Info("Stopping AddressPersister ... done")
	}, shutdown.ShutdownPriorityPersisters)
}

func addressPersisterSubmit(address trinary.Hash, transactionHash trinary.Hash) {
	addressPersisterWorkerPool.Submit(&tangle.TxHashForAddress{Address: address, TxHash: transactionHash})
}

// Unconfirmed Tx persister
func configureUnconfirmedTransactionPersister() {

	unconfirmedTxWorkerPool = batchworkerpool.New(func(tasks []batchworkerpool.Task) {

		var operations []*tangle.UnconfirmedTxHashOperation
		for _, task := range tasks {
			operations = append(operations, task.Param(0).(*tangle.UnconfirmedTxHashOperation))
		}

		err := tangle.StoreUnconfirmedTxHashOperations(operations)
		if err != nil {
			panic(err)
		}

		for _, task := range tasks {
			task.Return(nil)
		}
	}, batchworkerpool.BatchCollectionTimeout(unconfirmedTxBatchCollectionTimeout), batchworkerpool.BatchSize(unconfirmedTxBatchSize), batchworkerpool.WorkerCount(unconfirmedTxWorkerCount), batchworkerpool.QueueSize(unconfirmedTxQueueSize))
}

func runUnconfirmedTransactionPersister() {

	notifyNewTx := events.NewClosure(func(transaction *hornet.Transaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		unconfirmedTxWorkerPool.Submit(&tangle.UnconfirmedTxHashOperation{
			TxHash:                        transaction.GetHash(),
			FirstSeenLatestMilestoneIndex: firstSeenLatestMilestoneIndex,
		})
	})

	notifyConfirmedTx := events.NewClosure(func(transaction *hornet.Transaction, msIndex milestone_index.MilestoneIndex, confTime int64) {
		unconfirmedTxWorkerPool.Submit(&tangle.UnconfirmedTxHashOperation{
			TxHash:    transaction.GetHash(),
			Confirmed: true,
		})
	})

	daemon.BackgroundWorker("UnconfirmedTxPersister", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting UnconfirmedTxPersister ... done")
		Events.ReceivedNewTransaction.Attach(notifyNewTx)
		Events.TransactionConfirmed.Attach(notifyConfirmedTx)
		unconfirmedTxWorkerPool.Start()
		<-shutdownSignal
		Events.ReceivedNewTransaction.Detach(notifyNewTx)
		Events.TransactionConfirmed.Detach(notifyConfirmedTx)
		unconfirmedTxWorkerPool.StopAndWait()
		log.Info("Stopping UnconfirmedTxPersister ... done")
	}, shutdown.ShutdownPriorityPersisters)
}

// Tx stored event
func onEvictTransactions(evicted []*hornet.Transaction) {
	for _, tx := range evicted {
		Events.TransactionStored.Trigger(tx)
	}
}
