package tangle

import (
	"github.com/iotaledger/iota.go/trinary"
	"github.com/gohornet/hornet/packages/datastructure"
	"github.com/gohornet/hornet/packages/model/hornet"
)

var (
	transactionCache       *datastructure.LRUCache
	evictionNotifyCallback func(notifyStoredTx []*hornet.Transaction)
)

func InitTransactionCache(notifyCallback func(notifyStoredTx []*hornet.Transaction)) {
	transactionCache = datastructure.NewLRUCache(TransactionCacheSize, &datastructure.LRUCacheOptions{
		EvictionCallback:  onEvictTransactions,
		EvictionBatchSize: 1000,
	})
	evictionNotifyCallback = notifyCallback
}

func onEvictTransactions(_ interface{}, values interface{}) {

	valT := values.([]interface{})

	var txs []*hornet.Transaction

	for _, obj := range valT {
		tx := obj.(*hornet.Transaction)
		//if tx.IsModified() && (tx.IsRequested() || tx.IsSolid()) {
		if tx.IsModified() {
			// Store modified tx that are solid or were requested
			txs = append(txs, tx)
		}
	}

	if err := StoreTransactionsInDatabase(txs); err != nil {
		panic(err)
	}

	evictionNotifyCallback(txs)
}

func StoreEvictedTransactions(evicted []*hornet.Transaction) []*hornet.Transaction {
	var txs []*hornet.Transaction

	for _, tx := range evicted {
		if tx.IsModified() && (tx.IsRequested() || tx.IsSolid()) {
			// Store modified tx that are solid or were requested
			txs = append(txs, tx)
		}
	}

	if err := StoreTransactionsInDatabase(txs); err != nil {
		panic(err)
	}

	return txs
}

func StoreTransactionInCache(transaction *hornet.Transaction) {
	transactionCache.Set(transaction.GetHash(), transaction)
}

func DiscardTransactionFromCache(txHash trinary.Hash) {
	transactionCache.DeleteWithoutEviction(txHash)
}

func FlushTransactionCache() {
	transactionCache.DeleteAll()
}
