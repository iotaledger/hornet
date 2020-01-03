package dag

import (
	"fmt"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/tangle"
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

			tx, _ := tangle.GetTransaction(txHash)
			if tx == nil {
				return nil, fmt.Errorf("FindAllTails: Transaction not found: %v", txHash)
			}

			if tx.IsTail() {
				tails[txHash] = struct{}{}
				continue
			}

			// Mark the approvees to be traversed
			txsToTraverse[tx.GetTrunk()] = struct{}{}
			txsToTraverse[tx.GetBranch()] = struct{}{}
		}
	}
	return tails, nil
}
