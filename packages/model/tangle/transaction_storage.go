package tangle

import (
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/compressed"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
)

func CachedTransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedTx *CachedTransaction))(params[0].(*CachedTransaction).Retain())
}

func CachedNewTransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedTx *CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex))(params[0].(*CachedTransaction).Retain(), params[1].(milestone_index.MilestoneIndex), params[2].(milestone_index.MilestoneIndex))
}

func CachedTransactionConfirmedCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedTx *CachedTransaction, msIndex milestone_index.MilestoneIndex, confTime int64))(params[0].(*CachedTransaction).Retain(), params[1].(milestone_index.MilestoneIndex), params[2].(int64))
}

type CachedTransaction struct {
	objectstorage.CachedObject

	tx *Transaction
}

func (c *CachedTransaction) Exists() bool {
	return c.tx.cachedRawData.Exists() && c.tx.cachedMetaData.Exists()
}

func (c *CachedTransaction) Get() objectstorage.StorableObject {
	return nil
}

func (c *CachedTransaction) Consume(consumer func(objectstorage.StorableObject)) bool {
	return true
}

// tx +1
func (c *CachedTransaction) Retain() *CachedTransaction {
	c.tx.cachedRawData.Retain()
	c.tx.cachedMetaData.Retain()
	return c
}

func (c *CachedTransaction) Release(force ...bool) {
	c.tx.cachedRawData.Release(force...)
	c.tx.cachedMetaData.Release(force...)
}

func (c *CachedTransaction) GetTransaction() *Transaction {
	return c.tx
}

// tx -1
func (c *CachedTransaction) ConsumeTransaction(consumer func(*Transaction)) {

	c.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Transaction))
	})
}

// tx +1
func GetCachedTransaction(txHash trinary.Hash) *CachedTransaction {

	txHashBytes := trinary.MustTrytesToBytes(txHash)[:49]

	cachedTransactionRawData := &hornet.CachedTransactionRawData{txRawStorage.Load(txHashBytes)}
	cachedTransactionMetaData := &hornet.CachedTransactionMetaData{txMetaStorage.Load(txHashBytes)}

	transaction, _ := compressed.TransactionFromCompressedBytes(cachedTransactionRawData.GetTransactionRawData().RawBytes, txHash)

	tx := &Transaction{
		TxHash:         txHashBytes,
		cachedRawData:  cachedTransactionRawData,
		cachedMetaData: cachedTransactionMetaData,
		Tx:             transaction,
		timestamp:      getTimestampFromTx(transaction),
	}

	return &CachedTransaction{tx: tx}
}

// tx +-0
func ContainsTransaction(transactionHash trinary.Hash) bool {
	return txRawStorage.Contains(trinary.MustTrytesToBytes(transactionHash)[:49])
}

// tx +1
func StoreTransaction(transaction *Transaction) *CachedTransaction {
	// Todo?
	return nil
}

// tx +-0
func DeleteTransaction(txHash trinary.Hash) {
	txHashBytes := trinary.MustTrytesToBytes(txHash)[:49]
	txRawStorage.Delete(txHashBytes)
	txMetaStorage.Delete(txHashBytes)
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
