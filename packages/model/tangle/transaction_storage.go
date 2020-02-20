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
func (cachedTxs CachedTransactions) Release() {
	for _, cachedTx := range cachedTxs {
		cachedTx.Release()
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
func (c *CachedTransaction) ConsumeTransaction(consumer func(*hornet.Transaction)) {

	c.tx.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*hornet.Transaction))
	})
}

// tx -1
func (c *CachedTransaction) ConsumeMetadata(consumer func(*hornet.TransactionMetadata)) {

	c.metadata.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*hornet.TransactionMetadata))
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
		[]byte{DBPrefixTransactions},
		transactionFactory,
		objectstorage.BadgerInstance(database.GetHornetBadgerInstance()),
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)

	metadataStorage = objectstorage.New(
		[]byte{DBPrefixTransactionMetadata},
		metadataFactory,
		objectstorage.BadgerInstance(database.GetHornetBadgerInstance()),
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
func GetCachedTransaction(transactionHash trinary.Hash) *CachedTransaction {
	txHash := trinary.MustTrytesToBytes(transactionHash)[:49]
	return &CachedTransaction{
		txStorage.Load(txHash),
		metadataStorage.ComputeIfAbsent(txHash, metadataFactory),
	}
}

// tx +-0
func ContainsTransaction(transactionHash trinary.Hash) bool {
	return txStorage.Contains(trinary.MustTrytesToBytes(transactionHash)[:49])
}

// tx +1
func StoreTransaction(transaction *hornet.Transaction) *CachedTransaction {
	return &CachedTransaction{
		txStorage.Store(transaction),
		metadataStorage.ComputeIfAbsent(transaction.TxHash, metadataFactory),
	}
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
