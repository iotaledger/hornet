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
	txStorage *objectstorage.ObjectStorage
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
	objectstorage.CachedObject
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
	return c.Get().(*hornet.Transaction)
}

// tx +1
func (c *CachedTransaction) Retain() *CachedTransaction {
	return &CachedTransaction{c.CachedObject.Retain()}
}

// tx -1
func (c *CachedTransaction) ConsumeTransaction(consumer func(*hornet.Transaction)) {

	c.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*hornet.Transaction))
	})
}

func transactionFactory(key []byte) objectstorage.StorableObject {
	tx := &hornet.Transaction{
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
}

// tx +1
func GetCachedTransaction(transactionHash trinary.Hash) *CachedTransaction {
	return &CachedTransaction{txStorage.Load(trinary.MustTrytesToBytes(transactionHash)[:49])}
}

// tx +-0
func ContainsTransaction(transactionHash trinary.Hash) bool {
	return txStorage.Contains(trinary.MustTrytesToBytes(transactionHash)[:49])
}

// tx +1
func StoreTransaction(transaction *hornet.Transaction) *CachedTransaction {
	return &CachedTransaction{txStorage.Store(transaction)}
}

// tx +-0
func DeleteTransaction(txHash trinary.Hash) {
	txStorage.Delete(trinary.MustTrytesToBytes(txHash)[:49])
}

func ShutdownTransactionStorage() {
	txStorage.Shutdown()
}
