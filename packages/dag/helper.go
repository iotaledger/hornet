package dag

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/tangle"
)

var (
	ErrFindAllTailsFailed = errors.New("Unable to find all tails")
)

func FindAllTails(txHash trinary.Hash, forceRelease bool) (map[string]struct{}, error) {

	txsToTraverse := make(map[string]struct{})
	txsChecked := make(map[string]struct{})
	tails := make(map[string]struct{})

	txsToTraverse[txHash] = struct{}{}

	// Collect all tx to check by traversing the tangle
	// Loop as long as new transactions are added in every loop cycle
	for len(txsToTraverse) != 0 {

		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)

			if _, checked := txsChecked[txHash]; checked {
				// Tx was already checked => ignore
				continue
			}
			txsChecked[txHash] = struct{}{}

			if tangle.SolidEntryPointsContain(txHash) {
				// Ignore solid entry points (snapshot milestone included)
				continue
			}

			cachedTx := tangle.GetCachedTransactionOrNil(txHash) // tx +1
			if cachedTx == nil {
				return nil, errors.Wrapf(ErrFindAllTailsFailed, "Transaction not found: %v", txHash)
			}

			if cachedTx.GetTransaction().IsTail() {
				tails[txHash] = struct{}{}
				cachedTx.Release(forceRelease) // tx -1
				continue
			}

			// Mark the approvees to be traversed
			txsToTraverse[cachedTx.GetTransaction().GetTrunk()] = struct{}{}
			txsToTraverse[cachedTx.GetTransaction().GetBranch()] = struct{}{}
			cachedTx.Release(forceRelease) // tx -1
		}
	}
	return tails, nil
}
