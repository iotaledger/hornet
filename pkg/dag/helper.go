package dag

import (
	"bytes"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

var (
	ErrFindAllTailsFailed = errors.New("Unable to find all tails")
)

// FindAllTails searches all tail transactions the given startTxHash references.
// If skipStartTx is true, the startTxHash will be ignored and traversed, even if it is a tail transaction.
func FindAllTails(startTxHash hornet.Hash, skipStartTx bool, forceRelease bool) (map[string]struct{}, error) {

	tails := make(map[string]struct{})

	err := TraverseApprovees(startTxHash,
		// traversal stops if no more transactions pass the given condition
		func(cachedTx *tangle.CachedTransaction) (bool, error) { // tx +1
			defer cachedTx.Release(true) // tx -1

			if skipStartTx && bytes.Equal(startTxHash, cachedTx.GetTransaction().GetTxHash()) {
				// skip the start tx
				return true, nil
			}

			if cachedTx.GetTransaction().IsTail() {
				// transaction is a tail, do not traverse further
				tails[string(cachedTx.GetTransaction().GetTxHash())] = struct{}{}
				return false, nil
			}

			return true, nil
		},
		// consumer
		func(cachedTx *tangle.CachedTransaction) error { // tx +1
			defer cachedTx.Release(true) // tx -1
			return nil
		},
		// called on missing approvees
		func(approveeHash hornet.Hash) error {
			return errors.Wrapf(ErrFindAllTailsFailed, "transaction not found: %v", approveeHash.Trytes())
		},
		// called on solid entry points
		func(approveeHash hornet.Hash) {
			// Ignore solid entry points (snapshot milestone included)
		}, true, false, nil)

	return tails, err
}

// Predicate defines whether a traversal should continue or not.
type Predicate func(cachedTx *tangle.CachedTransaction) (bool, error)

// Consumer consumes the given transaction during traversal.
type Consumer func(cachedTx *tangle.CachedTransaction) error

// OnMissingApprovee gets called when during traversal an approvee is missing.
type OnMissingApprovee func(approveeHash hornet.Hash) error

// OnSolidEntryPoint gets called when during traversal the startTx or approvee is a solid entry point.
type OnSolidEntryPoint func(txHash hornet.Hash)

// TraverseApprovees starts to traverse the approvees (past cone) of the given start transaction until
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
				cachedTx.Release(forceRelease) // tx -1
				return err
			}

			if !traverse {
				cachedTx.Release(forceRelease) // tx -1
				continue
			}

			// consume the tx
			if err := consumer(cachedTx.Retain()); err != nil { // tx +1
				cachedTx.Release(forceRelease) // tx -1
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

// TraverseApprovers starts to traverse the approvers (future cone) of the given start transaction until
// the traversal stops due to no more transactions passing the given condition.
func TraverseApprovers(startTxHash hornet.Hash, condition Predicate, consumer Consumer, forceRelease bool, abortSignal <-chan struct{}) error {

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
				cachedTx.Release(forceRelease) // tx -1
				return err
			}

			if !traverse {
				cachedTx.Release(forceRelease) // tx -1
				continue
			}

			// consume the tx
			if err := consumer(cachedTx.Retain()); err != nil { // tx +1
				cachedTx.Release(forceRelease) // tx -1
				return err
			}
			cachedTx.Release(forceRelease) // tx -1

			for _, approverHash := range tangle.GetApproverHashes(hornet.Hash(txHash), forceRelease) {

				if _, checked := processed[string(approverHash)]; checked {
					continue
				}

				processed[string(approverHash)] = struct{}{}

				cachedApproverTx := tangle.GetCachedTransactionOrNil(approverHash) // approver +1
				if cachedApproverTx == nil {
					return errors.Wrapf(tangle.ErrTransactionNotFound, "hash: %s", approverHash.Trytes())
				}

				// do not force release since it is loaded again
				cachedApproverTx.Release() // approver -1

				txsToTraverse[string(approverHash)] = struct{}{}
			}
		}
	}

	return nil
}
