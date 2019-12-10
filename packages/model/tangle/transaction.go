package tangle

import (
	"github.com/iotaledger/iota.go/trinary"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/typeutils"
)

func GetTransaction(transactionHash trinary.Hash) (result *hornet.Transaction, err error) {
	if cacheResult := transactionCache.ComputeIfAbsent(transactionHash, func() interface{} {
		if transaction, dbErr := readTransactionFromDatabase(transactionHash); dbErr != nil {
			err = dbErr
			return nil
		} else if transaction != nil {
			return transaction
		} else {
			return nil
		}
	}); !typeutils.IsInterfaceNil(cacheResult) {
		result = cacheResult.(*hornet.Transaction)
	}

	return
}

func ContainsTransaction(transactionHash trinary.Hash) (result bool, err error) {
	if transactionCache.Contains(transactionHash) {
		result = true
	} else {
		result, err = databaseContainsTransaction(transactionHash)
	}
	return
}
