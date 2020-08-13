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
func checkSolidityAndPropagate(cachedTxMeta *tangle.CachedMetadata) {

	txsMetaToCheck := make(map[string]*tangle.CachedMetadata)
	txsMetaToCheck[string(cachedTxMeta.GetMetadata().GetTxHash())] = cachedTxMeta

	// Loop as long as new transactions are added in every loop cycle
	for len(txsMetaToCheck) != 0 {
		for txHash, cachedTxMetaToCheck := range txsMetaToCheck {
			delete(txsMetaToCheck, txHash)

			_, newlySolid := checkSolidity(cachedTxMetaToCheck.Retain())
			if newlySolid {
				if int32(time.Now().Unix())-cachedTxMetaToCheck.GetMetadata().GetSolidificationTimestamp() > solidifierThresholdInSeconds {
					// Skip older transactions and force release them
					cachedTxMetaToCheck.Release(true) // meta -1
					continue
				}

				for _, approverHash := range tangle.GetApproverHashes(hornet.Hash(txHash), true) {
					cachedApproverTxMeta := tangle.GetCachedTxMetadataOrNil(approverHash) // meta +1
					if cachedApproverTxMeta == nil {
						continue
					}

					if _, found := txsMetaToCheck[string(approverHash)]; found {
						// Do no force release here, otherwise cacheTime for new Tx could be ignored
						cachedApproverTxMeta.Release() // meta -1
						continue
					}

					txsMetaToCheck[string(approverHash)] = cachedApproverTxMeta
				}
			}
			// Do no force release here, otherwise cacheTime for new Tx could be ignored
			cachedTxMetaToCheck.Release() // meta -1
		}
	}
}
