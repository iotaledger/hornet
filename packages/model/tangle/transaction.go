package tangle

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	hornetDB "github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/profile"
)

var (
	txStorage *objectstorage.ObjectStorage
)

func TransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx *CachedTransaction))(params[0].(*CachedTransaction).Retain())
}

func NewTransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx *CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex))(params[0].(*CachedTransaction).Retain(), params[1].(milestone_index.MilestoneIndex), params[2].(milestone_index.MilestoneIndex))
}

func TransactionConfirmedCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx *CachedTransaction, msIndex milestone_index.MilestoneIndex, confTime int64))(params[0].(*CachedTransaction).Retain(), params[1].(milestone_index.MilestoneIndex), params[2].(int64))
}

type CachedTransaction struct {
	objectstorage.CachedObject
}

type CachedTransactions []*CachedTransaction

func (cachedTxs CachedTransactions) Retain() CachedTransactions {
	result := CachedTransactions{}
	for _, cachedTx := range cachedTxs {
		result = append(result, cachedTx.Retain())
	}
	return result
}

func (cachedTxs CachedTransactions) Release() {
	for _, cachedTx := range cachedTxs {
		cachedTx.Release()
	}
}

func (c *CachedTransaction) GetTransaction() *hornet.Transaction {
	return c.Get().(*hornet.Transaction)
}

func (c *CachedTransaction) Retain() *CachedTransaction {
	return &CachedTransaction{c.CachedObject.Retain()}
}

func (c *CachedTransaction) ConsumeTransaction(consumer func(*hornet.Transaction)) {

	c.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*hornet.Transaction))
	})
}

func transactionFactory(key []byte) objectstorage.StorableObject {
	return &hornet.Transaction{
		TxHash: key,
	}
}

func GetTransactionStorageSize() int {
	return txStorage.GetSize()
}

func configureTransactionStorage() {

	opts := profile.GetProfile().Caches.Transactions

	txStorage = objectstorage.New(
		[]byte{DBPrefixTransactions},
		transactionFactory,
		objectstorage.BadgerInstance(hornetDB.GetHornetBadgerInstance()),
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		//objectstorage.EnableLeakDetection(objectstorage.LeakDetectionOptions{
		//	MaxConsumersPerObject: 20,
		//	MaxConsumerHoldTime:   100 * time.Second,
		//}),
	)
}

func GetCachedTransaction(transactionHash trinary.Hash) *CachedTransaction {
	return &CachedTransaction{txStorage.Load(trinary.MustTrytesToBytes(transactionHash)[:49])}
}

func ContainsTransaction(transactionHash trinary.Hash) bool {
	return txStorage.Contains(trinary.MustTrytesToBytes(transactionHash)[:49])
}

func StoreTransaction(transaction *hornet.Transaction) *CachedTransaction {
	return &CachedTransaction{txStorage.Store(transaction)}
}

func DeleteTransaction(txHash trinary.Hash) {
	txStorage.Delete(trinary.MustTrytesToBytes(txHash)[:49])
}

func ShutdownTransactionStorage() {
	txStorage.Shutdown()
}
