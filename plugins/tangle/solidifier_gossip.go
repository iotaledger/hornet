package tangle

import (
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

const (
	solidifierThresholdInSeconds int32 = 60
)

// Checks and updates the solid flag of a transaction and its approvers (future cone).
func checkSolidityAndPropagate(cachedTx *tangle.CachedTransaction) {

	txsToCheck := make(map[string]*tangle.CachedTransaction)
	txsToCheck[string(cachedTx.GetTransaction().GetTxHash())] = cachedTx

	// Loop as long as new transactions are added in every loop cycle
	for len(txsToCheck) != 0 {
		for txHash, cachedTxToCheck := range txsToCheck {
			delete(txsToCheck, txHash)

			_, newlySolid := checkSolidity(cachedTxToCheck.Retain())
			if newlySolid {
				if int32(time.Now().Unix())-cachedTxToCheck.GetMetadata().GetSolidificationTimestamp() > solidifierThresholdInSeconds {
					// Skip older transactions and force release them
					cachedTxToCheck.Release(true) // tx -1
					continue
				}

				for _, approverHash := range tangle.GetApproverHashes(hornet.Hash(txHash), true) {
					cachedApproverTx := tangle.GetCachedTransactionOrNil(approverHash) // tx +1
					if cachedApproverTx == nil {
						continue
					}

					if _, found := txsToCheck[string(approverHash)]; found {
						// Do no force release here, otherwise cacheTime for new Tx could be ignored
						cachedApproverTx.Release() // tx -1
						continue
					}

					txsToCheck[string(approverHash)] = cachedApproverTx
				}
			}
			// Do no force release here, otherwise cacheTime for new Tx could be ignored
			cachedTxToCheck.Release() // tx -1
		}
	}
}
