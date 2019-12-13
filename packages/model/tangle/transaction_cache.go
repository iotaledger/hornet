package tangle

import (
	"github.com/gohornet/hornet/packages/datastructure"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/profile"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	transactionCache       *datastructure.LRUCache
	evictionNotifyCallback func(notifyStoredTx []*hornet.Transaction)
)

func InitTransactionCache(notifyCallback func(notifyStoredTx []*hornet.Transaction)) {
	opts := profile.GetProfile().Caches.Transactions
	transactionCache = datastructure.NewLRUCache(opts.Size, &datastructure.LRUCacheOptions{
		EvictionCallback:  onEvictTransactions,
		EvictionBatchSize: opts.EvictionSize,
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
