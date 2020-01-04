package tangle

import (
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/lru_cache"

	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/profile"
)

var (
	TransactionCache       *lru_cache.LRUCache
	evictionNotifyCallback func(notifyStoredTx []*hornet.Transaction)
)

func InitTransactionCache(notifyCallback func(notifyStoredTx []*hornet.Transaction)) {
	opts := profile.GetProfile().Caches.Transactions
	TransactionCache = lru_cache.NewLRUCache(opts.Size, &lru_cache.LRUCacheOptions{
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
	TransactionCache.Set(transaction.GetHash(), transaction)
}

func DiscardTransactionFromCache(txHash trinary.Hash) {
	TransactionCache.DeleteWithoutEviction(txHash)
}

func FlushTransactionCache() {
	TransactionCache.DeleteAll()
}
