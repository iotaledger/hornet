package dag

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/tangle"
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
				return nil, errors.Wrapf(ErrFindAllTailsFailed, "transaction not found: %v", txHash)
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

// Predicate defines whether a traversal should continue or not.
type Predicate func(cachedTx *tangle.CachedTransaction) bool

// Consumer consumes the given transaction during traversal.
type Consumer func(cachedTx *tangle.CachedTransaction)

// OnMissingApprovee gets called when during traversal an approvee is missing.
type OnMissingApprovee func(approveeHash trinary.Hash)

// TraverseApprovees starts to traverse the approvees of the given start transaction until
// the traversal stops due to no more transactions passing the given condition.
func TraverseApprovees(startTxHash trinary.Hash, condition Predicate, consumer Consumer, onMissingApprovee OnMissingApprovee, forceRelease bool) {

	if tangle.SolidEntryPointsContain(startTxHash) {
		return
	}

	processed := map[trinary.Hash]struct{}{}
	txsToTraverse := map[trinary.Hash]struct{}{startTxHash: {}}
	for len(txsToTraverse) != 0 {
		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)

			cachedTx := tangle.GetCachedTransactionOrNil(txHash) // tx +1
			if cachedTx == nil {
				continue
			}

			if txHash != startTxHash && !condition(cachedTx.Retain()) { // tx + 1
				cachedTx.Release(forceRelease)
				continue
			}

			// do not consume the start transaction
			if txHash != startTxHash {
				consumer(cachedTx.Retain()) // tx +1
			}

			approveeHashes := map[trinary.Hash]struct{}{
				cachedTx.GetTransaction().GetTrunk():  {},
				cachedTx.GetTransaction().GetBranch(): {},
			}

			cachedTx.Release(forceRelease) // tx -1

			for approveeHash := range approveeHashes {
				if tangle.SolidEntryPointsContain(approveeHash) {
					continue
				}

				if _, checked := processed[approveeHash]; checked {
					continue
				}

				processed[approveeHash] = struct{}{}

				cachedApproveeTx := tangle.GetCachedTransactionOrNil(approveeHash) // approvee +1
				if cachedApproveeTx == nil {
					onMissingApprovee(approveeHash)
					continue
				}

				// do not force release since it is loaded again
				cachedApproveeTx.Release() // approvee -1

				txsToTraverse[approveeHash] = struct{}{}
			}
		}
	}
}
