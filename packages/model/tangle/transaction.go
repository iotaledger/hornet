package tangle

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	hornetDB "github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
)

var txStorage *objectstorage.ObjectStorage

func TransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx *CachedTransaction))(params[0].(*CachedTransaction))
}

func NewTransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx *CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex))(params[0].(*CachedTransaction), params[1].(milestone_index.MilestoneIndex), params[2].(milestone_index.MilestoneIndex))
}

func TransactionConfirmedCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx *CachedTransaction, msIndex milestone_index.MilestoneIndex, confTime int64))(params[0].(*CachedTransaction), params[1].(milestone_index.MilestoneIndex), params[2].(int64))
}

type CachedTransaction struct {
	*objectstorage.CachedObject
}

type CachedTransactions []*CachedTransaction

func (cachedTxs CachedTransactions) RegisterConsumer() {
	for _, cachedTx := range cachedTxs {
		cachedTx.RegisterConsumer()
	}
}

func (cachedTxs CachedTransactions) Release() {
	for _, cachedTx := range cachedTxs {
		cachedTx.Release()
	}
}

func (c *CachedTransaction) GetTransaction() *hornet.Transaction {
	return c.Get().(*hornet.Transaction)
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

	txStorage = objectstorage.New(
		[]byte{DBPrefixTransactions},
		transactionFactory,
		objectstorage.BadgerInstance(hornetDB.GetHornetBadgerInstance()),
		objectstorage.CacheTime(1500*time.Millisecond),
		objectstorage.PersistenceEnabled(true))
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

func FlushTransactionStorage() {
	txStorage.Flush()
}
