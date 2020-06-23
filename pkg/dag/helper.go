package dag

import (
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

var (
	ErrFindAllTailsFailed = errors.New("Unable to find all tails")
)

func FindAllTails(txHash hornet.Hash, forceRelease bool) (map[string]struct{}, error) {

	txsToTraverse := map[string]struct{}{string(txHash): {}}
	txsChecked := make(map[string]struct{})
	tails := make(map[string]struct{})

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

			if tangle.SolidEntryPointsContain(hornet.Hash(txHash)) {
				// Ignore solid entry points (snapshot milestone included)
				continue
			}

			cachedTx := tangle.GetCachedTransactionOrNil(hornet.Hash(txHash)) // tx +1
			if cachedTx == nil {
				return nil, errors.Wrapf(ErrFindAllTailsFailed, "transaction not found: %v", hornet.Hash(txHash).Trytes())
			}

			if cachedTx.GetTransaction().IsTail() {
				tails[txHash] = struct{}{}
				cachedTx.Release(forceRelease) // tx -1
				continue
			}

			// Mark the approvees to be traversed
			txsToTraverse[string(cachedTx.GetTransaction().GetTrunkHash())] = struct{}{}
			txsToTraverse[string(cachedTx.GetTransaction().GetBranchHash())] = struct{}{}
			cachedTx.Release(forceRelease) // tx -1
		}
	}
	return tails, nil
}

// Predicate defines whether a traversal should continue or not.
type Predicate func(cachedTx *tangle.CachedTransaction) (bool, error)

// Consumer consumes the given transaction during traversal.
type Consumer func(cachedTx *tangle.CachedTransaction) error

// OnMissingApprovee gets called when during traversal an approvee is missing.
type OnMissingApprovee func(approveeHash hornet.Hash) error

// OnSolidEntryPoint gets called when during traversal the startTx or approvee is a solid entry point.
type OnSolidEntryPoint func(txHash hornet.Hash)

// TraverseApprovees starts to traverse the approvees of the given start transaction until
// the traversal stops due to no more transactions passing the given condition.
func TraverseApprovees(startTxHash hornet.Hash, condition Predicate, consumer Consumer, onMissingApprovee OnMissingApprovee, onSolidEntryPoint OnSolidEntryPoint, forceRelease bool, traverseSolidEntryPoints bool, abortSignal <-chan struct{}) error {

	if tangle.SolidEntryPointsContain(startTxHash) {
		onSolidEntryPoint(startTxHash)
		if !traverseSolidEntryPoints {
			return nil
		}
	}

	processed := map[string]struct{}{}
	txsToTraverse := map[string]struct{}{string(startTxHash): {}}
	for len(txsToTraverse) != 0 {
		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)

			select {
			case <-abortSignal:
				return tangle.ErrOperationAborted
			default:
			}

			cachedTx := tangle.GetCachedTransactionOrNil(hornet.Hash(txHash)) // tx +1
			if cachedTx == nil {
				return errors.Wrapf(tangle.ErrTransactionNotFound, "hash: %s", hornet.Hash(txHash).Trytes())
			}

			// check condition to decide if tx should be consumed and traversed
			traverse, err := condition(cachedTx.Retain()) // tx + 1
			if err != nil {
				cachedTx.Release(forceRelease)
				return err
			}

			if !traverse {
				cachedTx.Release(forceRelease)
				continue
			}

			// consume the tx
			if err := consumer(cachedTx.Retain()); err != nil { // tx +1
				cachedTx.Release(forceRelease)
				return err
			}

			approveeHashes := map[string]struct{}{
				string(cachedTx.GetTransaction().GetTrunkHash()):  {},
				string(cachedTx.GetTransaction().GetBranchHash()): {},
			}

			cachedTx.Release(forceRelease) // tx -1

			for approveeHash := range approveeHashes {
				if tangle.SolidEntryPointsContain(hornet.Hash(approveeHash)) {
					onSolidEntryPoint(hornet.Hash(approveeHash))
					if !traverseSolidEntryPoints {
						continue
					}
				}

				if _, checked := processed[approveeHash]; checked {
					continue
				}

				processed[approveeHash] = struct{}{}

				cachedApproveeTx := tangle.GetCachedTransactionOrNil(hornet.Hash(approveeHash)) // approvee +1
				if cachedApproveeTx == nil {
					if err := onMissingApprovee(hornet.Hash(approveeHash)); err != nil {
						return err
					}
					continue
				}

				// do not force release since it is loaded again
				cachedApproveeTx.Release() // approvee -1

				txsToTraverse[approveeHash] = struct{}{}
			}
		}
	}

	return nil
}
