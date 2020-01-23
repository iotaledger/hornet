package tangle

import (
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/tangle"
)

func addTransactionToBundleBucket(transaction *hornet.Transaction) (bundles []*tangle.Bundle, alreadyAdded bool) {

	bundleBucket, err := tangle.GetBundleBucket(transaction.Tx.Bundle)
	if err != nil {
		log.Panic(err)
	}

	return bundleBucket.AddTransaction(transaction)
}
