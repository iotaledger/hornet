package dag

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/tangle"
)

var (
	ErrFindAllTailsFailed = errors.New("Unable to find all tails")
)

func FindAllTails(txHash trinary.Hash) (map[string]struct{}, error) {

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

			tx := tangle.GetCachedTransaction(txHash) //+1

			if !tx.Exists() {
				tx.Release() //-1
				return nil, errors.Wrapf(ErrFindAllTailsFailed, "Transaction not found: %v", txHash)
			}

			if tx.GetTransaction().IsTail() {
				tails[txHash] = struct{}{}
				tx.Release() //-1
				continue
			}

			// Mark the approvees to be traversed
			txsToTraverse[tx.GetTransaction().GetTrunk()] = struct{}{}
			txsToTraverse[tx.GetTransaction().GetBranch()] = struct{}{}
			tx.Release() //-1
		}
	}
	return tails, nil
}
