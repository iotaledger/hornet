package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/batchworkerpool"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
)

var (
	addressPersisterWorkerCount            = 1
	addressPersisterQueueSize              = 10000
	addressPersisterBatchSize              = 1000
	addressPersisterBatchCollectionTimeout = 1000 * time.Millisecond
	addressPersisterWorkerPool             *batchworkerpool.BatchWorkerPool
)

func configurePersisters() {
	configureAddressPersister()
}

func runPersisters() {
	runAddressPersister()
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
