package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/profile"
)

var (
	txRawStorage *objectstorage.ObjectStorage
)

func transactionRawFactory(key []byte) objectstorage.StorableObject {
	tx := &hornet.TransactionRawData{
		TxHash: make([]byte, len(key)),
	}
	copy(tx.TxHash, key)
	return tx
}

func GetTransactionRawStorageSize() int {
	return txRawStorage.GetSize()
}

func configureTransactionRawStorage() {

	opts := profile.GetProfile().Caches.TransactionRawData

	txRawStorage = objectstorage.New(
		database.GetHornetBadgerInstance(),
		[]byte{DBPrefixTransactions},
		transactionRawFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

func ShutdownTransactionRawStorage() {
	txRawStorage.Shutdown()
}
