package tangle

import (
	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/iota.go/trinary"
	"time"
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

	txStorage = objectstorage.New(database.GetBadgerInstance(),
		[]byte{DBPrefixTransactions},
		transactionFactory,
		objectstorage.CacheTime(5*time.Second),
		objectstorage.PersistenceEnabled(true))
}

func GetCachedTransaction(transactionHash trinary.Hash) (*CachedTransaction, error) {
	cached, err := txStorage.Load(trinary.MustTrytesToBytes(transactionHash))
	if err != nil {
		return nil, err
	}
	return &CachedTransaction{cached}, nil
}

func ContainsTransaction(transactionHash trinary.Hash) (result bool, err error) {

	cachedObject, err := txStorage.Load(trinary.MustTrytesToBytes(transactionHash))
	if err != nil {
		return false, err
	}

	defer cachedObject.Release()
	return cachedObject.Exists(), nil
}

func StoreTransaction(transaction *hornet.Transaction) *CachedTransaction {
	cached := &CachedTransaction{txStorage.Store(transaction)}
	return cached
}

func DiscardTransaction(txHash trinary.Hash) {
	txStorage.Delete(trinary.MustTrytesToBytes(txHash))
}

func FlushTransactionCache() {
	//TODO
}
