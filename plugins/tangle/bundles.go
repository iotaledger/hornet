package tangle

import (
	"github.com/gohornet/hornet/packages/model/tangle"
)

func addTransactionToBundleBucket(transaction *tangle.CachedTransaction) []*tangle.Bundle {

	transaction.RegisterConsumer() //+1
	defer transaction.Release()    //-1

	bundleBucket, err := tangle.GetBundleBucket(transaction.GetTransaction().Tx.Bundle)
	if err != nil {
		log.Panic(err)
	}

	return bundleBucket.AddTransaction(transaction)
}
