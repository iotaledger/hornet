package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/profile"
)

var (
	txMetaStorage *objectstorage.ObjectStorage
)

func txMetaFactory(key []byte) objectstorage.StorableObject {
	txMeta := &hornet.TransactionMetaData{
		TxHash: make([]byte, len(key)),
	}
	copy(txMeta.TxHash, key)
	return txMeta
}

func GetTransactionMetaStorageSize() int {
	return txMetaStorage.GetSize()
}

func configureTransactionMetaStorage() {

	opts := profile.GetProfile().Caches.TransactionMetaData

	txMetaStorage = objectstorage.New(
		database.GetHornetBadgerInstance(),
		[]byte{DBPrefixTransactionMetaData},
		txMetaFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

func ShutdownTransactionMetaStorage() {
	txMetaStorage.Shutdown()
}
