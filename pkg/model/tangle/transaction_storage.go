package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/profile"
)

var (
	txStorage       *objectstorage.ObjectStorage
	metadataStorage *objectstorage.ObjectStorage
)

func TransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedTx *CachedTransaction))(params[0].(*CachedTransaction).Retain())
}

func TransactionHashCaller(handler interface{}, params ...interface{}) {
	handler.(func(txHash hornet.Hash))(params[0].(hornet.Hash))
}

func NewTransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedTx *CachedTransaction, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index))(params[0].(*CachedTransaction).Retain(), params[1].(milestone.Index), params[2].(milestone.Index))
}

func TransactionConfirmedCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedTx *CachedTransaction, msIndex milestone.Index, confTime int64))(params[0].(*CachedTransaction).Retain(), params[1].(milestone.Index), params[2].(int64))
}

// CachedTransaction contains two cached objects, one for transaction data and one for metadata.
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

func transactionFactory(key []byte) (objectstorage.StorableObject, int, error) {
	tx := hornet.NewTransaction(key[:49])
	return tx, 49, nil
}

func metadataFactory(key []byte) (objectstorage.StorableObject, int, error) {
	tx := hornet.NewTransactionMetadata(key[:49])
	return tx, 49, nil
}

func GetTransactionStorageSize() int {
	return txStorage.GetSize()
}

func configureTransactionStorage(store kvstore.KVStore) {

	opts := profile.LoadProfile().Caches.Transactions

	txStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixTransactions}),
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
		store.WithRealm([]byte{StorePrefixTransactionMetadata}),
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
func GetCachedTransactionOrNil(txHash hornet.Hash) *CachedTransaction {
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

// GetStoredTransactionOrNil returns a transaction object without accessing the cache layer.
func GetStoredTransactionOrNil(txHash hornet.Hash) *hornet.Transaction {
	storedTx := txStorage.LoadObjectFromStore(txHash)
	if storedTx == nil {
		return nil
	}
	return storedTx.(*hornet.Transaction)
}

// GetStoredMetadataOrNil returns a metadata object without accessing the cache layer.
func GetStoredMetadataOrNil(txHash hornet.Hash) *hornet.TransactionMetadata {
	storedMeta := metadataStorage.LoadObjectFromStore(txHash)
	if storedMeta == nil {
		return nil
	}
	return storedMeta.(*hornet.TransactionMetadata)
}

// tx +-0
func ContainsTransaction(txHash hornet.Hash) bool {
	return txStorage.Contains(txHash)
}

// tx +1
func StoreTransactionIfAbsent(transaction *hornet.Transaction) (cachedTx *CachedTransaction, newlyAdded bool) {

	// Store tx + metadata atomically in the same callback
	var cachedMeta objectstorage.CachedObject

	cachedTxData := txStorage.ComputeIfAbsent(transaction.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject { // tx +1
		newlyAdded = true

		metadata, _, _ := metadataFactory(transaction.GetTxHash())
		cachedMeta = metadataStorage.Store(metadata) // meta +1

		transaction.Persist()
		transaction.SetModified()
		return transaction
	})

	// if we didn't create a new entry - retrieve the corresponding metadata (it should always exist since it gets created atomically)
	if !newlyAdded {
		cachedMeta = metadataStorage.Load(transaction.GetTxHash()) // meta +1
	}

	return &CachedTransaction{tx: cachedTxData, metadata: cachedMeta}, newlyAdded
}

type TransactionConsumer func(cachedTx objectstorage.CachedObject, cachedTxMeta objectstorage.CachedObject)

type TransactionHashBytesConsumer func(txHash hornet.Hash) bool

func ForEachTransaction(consumer TransactionConsumer) {
	txStorage.ForEach(func(txHash []byte, cachedTx objectstorage.CachedObject) bool {
		defer cachedTx.Release(true) // tx -1

		cachedMeta := metadataStorage.Load(txHash) // tx meta +1
		defer cachedMeta.Release(true)             // tx meta -1
		if cachedMeta.Exists() {
			consumer(cachedTx.Retain(), cachedMeta.Retain())
			return true
		}

		consumer(cachedTx.Retain(), nil)
		return true
	})
}

// ForEachTransactionHash loops over all transaction hashes.
func ForEachTransactionHash(consumer TransactionHashBytesConsumer) {
	txStorage.ForEachKeyOnly(func(txHash []byte) bool {
		return consumer(txHash)
	}, false)
}

// tx +-0
func DeleteTransaction(txHash hornet.Hash) {
	txStorage.Delete(txHash)
	metadataStorage.Delete(txHash)
}

func ShutdownTransactionStorage() {
	txStorage.Shutdown()
	metadataStorage.Shutdown()
}

func FlushTransactionStorage() {
	txStorage.Flush()
	metadataStorage.Flush()
}
