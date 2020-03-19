package tangle

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/profile"
)

var (
	txStorage       *objectstorage.ObjectStorage
	metadataStorage *objectstorage.ObjectStorage
)

func TransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedTx *CachedTransaction))(params[0].(*CachedTransaction).Retain())
}

func NewTransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedTx *CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex))(params[0].(*CachedTransaction).Retain(), params[1].(milestone_index.MilestoneIndex), params[2].(milestone_index.MilestoneIndex))
}

func TransactionConfirmedCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedTx *CachedTransaction, msIndex milestone_index.MilestoneIndex, confTime int64))(params[0].(*CachedTransaction).Retain(), params[1].(milestone_index.MilestoneIndex), params[2].(int64))
}

type CachedTransaction struct {
	tx       objectstorage.CachedObject
	metadata objectstorage.CachedObject
}

type CachedTransactions []*CachedTransaction

// tx +1
func (cachedTxs CachedTransactions) Retain() CachedTransactions {
	cachedResult := CachedTransactions{}
	for _, cachedTx := range cachedTxs {
		cachedResult = append(cachedResult, cachedTx.Retain())
	}
	return cachedResult
}

// tx -1
func (cachedTxs CachedTransactions) Release(force ...bool) {
	for _, cachedTx := range cachedTxs {
		cachedTx.Release(force...)
	}
}

func (c *CachedTransaction) GetTransaction() *hornet.Transaction {
	return c.tx.Get().(*hornet.Transaction)
}

func (c *CachedTransaction) GetMetadata() *hornet.TransactionMetadata {
	return c.metadata.Get().(*hornet.TransactionMetadata)
}

// tx +1
func (c *CachedTransaction) Retain() *CachedTransaction {
	return &CachedTransaction{
		c.tx.Retain(),
		c.metadata.Retain(),
	}
}

func (c *CachedTransaction) Exists() bool {
	return c.tx.Exists()
}

// tx -1
func (c *CachedTransaction) ConsumeTransaction(consumer func(*hornet.Transaction, *hornet.TransactionMetadata)) {

	c.tx.Consume(func(txObject objectstorage.StorableObject) {
		c.metadata.Consume(func(metadataObject objectstorage.StorableObject) {
			consumer(txObject.(*hornet.Transaction), metadataObject.(*hornet.TransactionMetadata))
		})
	})
}

// tx -1
func (c *CachedTransaction) Release(force ...bool) {
	c.tx.Release(force...)
	c.metadata.Release(force...)
}

func transactionFactory(key []byte) objectstorage.StorableObject {
	tx := &hornet.Transaction{
		TxHash: make([]byte, len(key)),
	}
	copy(tx.TxHash, key)
	return tx
}

func metadataFactory(key []byte) objectstorage.StorableObject {
	tx := &hornet.TransactionMetadata{
		TxHash: make([]byte, len(key)),
	}
	copy(tx.TxHash, key)
	return tx
}

func GetTransactionStorageSize() int {
	return txStorage.GetSize()
}

func configureTransactionStorage() {

	opts := profile.GetProfile().Caches.Transactions

	txStorage = objectstorage.New(
		database.GetHornetBadgerInstance(),
		[]byte{DBPrefixTransactions},
		transactionFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)

	metadataStorage = objectstorage.New(
		database.GetHornetBadgerInstance(),
		[]byte{DBPrefixTransactionMetadata},
		metadataFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// tx +1
func GetCachedTransactionOrNil(transactionHash trinary.Hash) *CachedTransaction {
	txHash := trinary.MustTrytesToBytes(transactionHash)[:49]

	cachedTx := txStorage.Load(txHash) // tx +1
	if !cachedTx.Exists() {
		cachedTx.Release(true) // tx -1
		return nil
	}

	cachedMeta := metadataStorage.Load(txHash) // tx +1
	if !cachedMeta.Exists() {
		cachedTx.Release(true)   // tx -1
		cachedMeta.Release(true) // tx -1
		return nil
	}

	return &CachedTransaction{
		tx:       cachedTx,
		metadata: cachedMeta,
	}
}

// tx +-0
func ContainsTransaction(transactionHash trinary.Hash) bool {
	return txStorage.Contains(trinary.MustTrytesToBytes(transactionHash)[:49])
}

// tx +1
func StoreTransactionIfAbsent(transaction *hornet.Transaction) (cachedTx *CachedTransaction, newlyAdded bool) {

	txHash := trinary.MustTrytesToBytes(transaction.GetHash())[:49]

	// Store metadata first, because existence is checked via tx
	newlyAddedMetadata := false
	metadata := metadataFactory(txHash)
	cachedMeta := metadataStorage.ComputeIfAbsent(metadata.GetStorageKey(), func(key []byte) objectstorage.StorableObject { // meta +1
		newlyAddedMetadata = true
		metadata.Persist()
		metadata.SetModified()
		return metadata
	})

	cachedTxData := txStorage.ComputeIfAbsent(transaction.GetStorageKey(), func(key []byte) objectstorage.StorableObject { // tx +1
		if !newlyAddedMetadata {
			// Metadata was known, but transaction was missing => Reset corrupted metadata
			cachedMeta.Get().(*hornet.TransactionMetadata).Reset()
		}
		transaction.Persist()
		transaction.SetModified()
		return transaction
	})

	return &CachedTransaction{tx: cachedTxData, metadata: cachedMeta}, newlyAdded
}

// tx +-0
func DeleteTransaction(transactionHash trinary.Hash) {
	txHash := trinary.MustTrytesToBytes(transactionHash)[:49]
	txStorage.Delete(txHash)
	metadataStorage.Delete(txHash)
}

func ShutdownTransactionStorage() {
	txStorage.Shutdown()
	metadataStorage.Shutdown()
}
