package dag

import (
	"bytes"
	"container/list"
	"fmt"

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
		// Caution: condition func is not in DFS order
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
		}, true, false, false, nil)

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

// processStackApprovees checks if the current element in the stack must be processed or traversed.
// first the trunk is traversed, then the branch.
func processStackApprovees(stack *list.List, processed map[string]struct{}, checked map[string]bool, condition Predicate, consumer Consumer, onMissingApprovee OnMissingApprovee, onSolidEntryPoint OnSolidEntryPoint, forceRelease bool, traverseSolidEntryPoints bool, traverseTailsOnly bool, abortSignal <-chan struct{}) error {

	select {
	case <-abortSignal:
		return tangle.ErrOperationAborted
	default:
	}

	// load candidate tx
	ele := stack.Front()
	currentTxHash := ele.Value.(hornet.Hash)

	if _, wasProcessed := processed[string(currentTxHash)]; wasProcessed {
		// transaction was already processed
		// remove the transaction from the stack
		stack.Remove(ele)
		return nil
	}

	// check if the transaction is a solid entry point
	if tangle.SolidEntryPointsContain(currentTxHash) {
		onSolidEntryPoint(currentTxHash)

		if !traverseSolidEntryPoints {
			// remove the transaction from the stack, trunk and branch are not traversed
			processed[string(currentTxHash)] = struct{}{}
			delete(checked, string(currentTxHash))
			stack.Remove(ele)
			return nil
		}
	}

	cachedTx := tangle.GetCachedTransactionOrNil(currentTxHash) // tx +1
	if cachedTx == nil {
		// remove the transaction from the stack, trunk and branch are not traversed
		processed[string(currentTxHash)] = struct{}{}
		delete(checked, string(currentTxHash))
		stack.Remove(ele)

		// stop processing the stack if the caller returns an error
		return onMissingApprovee(currentTxHash)
	}

	defer cachedTx.Release(forceRelease) // tx -1

	traverse, checkedBefore := checked[string(currentTxHash)]
	if !checkedBefore {
		var err error

		// check condition to decide if tx should be consumed and traversed
		traverse, err = condition(cachedTx.Retain()) // tx + 1
		if err != nil {
			// there was an error, stop processing the stack
			return err
		}

		// mark the transaction as checked and remember the result of the traverse condition
		checked[string(currentTxHash)] = traverse
	}

	if !traverse {
		// remove the transaction from the stack, trunk and branch are not traversed
		// transaction will not get consumed
		processed[string(currentTxHash)] = struct{}{}
		delete(checked, string(currentTxHash))
		stack.Remove(ele)
		return nil
	}

	var trunkHash, branchHash hornet.Hash

	if !traverseTailsOnly {
		trunkHash = cachedTx.GetTransaction().GetTrunkHash()
		branchHash = cachedTx.GetTransaction().GetBranchHash()
	} else {
		// load up bundle to retrieve trunk and branch of the head tx
		cachedBundle := tangle.GetCachedBundleOrNil(currentTxHash)
		if cachedBundle == nil {
			return fmt.Errorf("%w: bundle %s of candidate tx %s doesn't exist", tangle.ErrBundleNotFound, cachedTx.GetTransaction().Tx.Bundle, currentTxHash.Trytes())
		}
		defer cachedBundle.Release(true)

		cachedBundleHeadTx := cachedBundle.GetBundle().GetHead()
		defer cachedBundleHeadTx.Release(true)
		trunkHash = cachedBundleHeadTx.GetTransaction().GetTrunkHash()
		branchHash = cachedBundleHeadTx.GetTransaction().GetBranchHash()
	}

	approveeHashes := hornet.Hashes{trunkHash}
	if !bytes.Equal(trunkHash, branchHash) {
		approveeHashes = append(approveeHashes, branchHash)
	}

	for _, approveeHash := range approveeHashes {
		if _, approveeProcessed := processed[string(approveeHash)]; !approveeProcessed {
			// approvee was not processed yet
			// traverse this transaction
			stack.PushFront(approveeHash)
			return nil
		}
	}

	// remove the transaction from the stack
	processed[string(currentTxHash)] = struct{}{}
	delete(checked, string(currentTxHash))
	stack.Remove(ele)

	// consume the transaction
	if err := consumer(cachedTx.Retain()); err != nil { // tx +1
		// there was an error, stop processing the stack
		return err
	}

	return nil
}

// TraverseApprovees starts to traverse the approvees (past cone) of the given start transaction until
// the traversal stops due to no more transactions passing the given condition.
// It is a DFS with trunk / branch.
// Caution: condition func is not in DFS order
func TraverseApprovees(startTxHash hornet.Hash, condition Predicate, consumer Consumer, onMissingApprovee OnMissingApprovee, onSolidEntryPoint OnSolidEntryPoint, forceRelease bool, traverseSolidEntryPoints bool, traverseTailsOnly bool, abortSignal <-chan struct{}) error {

	stack := list.New()

	// processed map with already processed transactions
	processed := make(map[string]struct{})

	// checked map with result of traverse condition
	checked := make(map[string]bool)

	stack.PushFront(startTxHash)
	for stack.Len() > 0 {
		if err := processStackApprovees(stack, processed, checked, condition, consumer, onMissingApprovee, onSolidEntryPoint, forceRelease, traverseSolidEntryPoints, traverseTailsOnly, abortSignal); err != nil {
			return err
		}
	}

	return nil
}

// processStackApprovers checks if the current element in the stack must be processed and traversed.
// current element gets consumed first, afterwards it's approvers get traversed in random order.
func processStackApprovers(stack *list.List, discovered map[string]struct{}, condition Predicate, consumer Consumer, forceRelease bool, abortSignal <-chan struct{}) error {

	select {
	case <-abortSignal:
		return tangle.ErrOperationAborted
	default:
	}

	// load candidate tx
	ele := stack.Front()
	currentTxHash := ele.Value.(hornet.Hash)

	// remove the transaction from the stack
	stack.Remove(ele)

	cachedTx := tangle.GetCachedTransactionOrNil(currentTxHash) // tx +1
	if cachedTx == nil {
		// there was an error, stop processing the stack
		return errors.Wrapf(tangle.ErrTransactionNotFound, "hash: %s", currentTxHash.Trytes())
	}

	defer cachedTx.Release(forceRelease) // tx -1

	// check condition to decide if tx should be consumed and traversed
	traverse, err := condition(cachedTx.Retain()) // tx + 1
	if err != nil {
		// there was an error, stop processing the stack
		return err
	}

	if !traverse {
		// transaction will not get consumed and approvers are not traversed
		return nil
	}

	// consume the transaction
	if err := consumer(cachedTx.Retain()); err != nil { // tx +1
		// there was an error, stop processing the stack
		return err
	}

	for _, approverHash := range tangle.GetApproverHashes(currentTxHash, forceRelease) {
		if _, approverDiscovered := discovered[string(approverHash)]; approverDiscovered {
			// approver was already discovered
			continue
		}

		// traverse the approver
		discovered[string(approverHash)] = struct{}{}
		stack.PushBack(approverHash)
	}

	return nil
}

// TraverseApprovers starts to traverse the approvers (future cone) of the given start transaction until
// the traversal stops due to no more transactions passing the given condition.
// It is unsorted BFS because the approvers are not ordered in the database.
func TraverseApprovers(startTxHash hornet.Hash, condition Predicate, consumer Consumer, forceRelease bool, abortSignal <-chan struct{}) error {

	stack := list.New()
	stack.PushFront(startTxHash)

	discovered := make(map[string]struct{})
	discovered[string(startTxHash)] = struct{}{}

	for stack.Len() > 0 {
		if err := processStackApprovers(stack, discovered, condition, consumer, forceRelease, abortSignal); err != nil {
			return err
		}
	}

	return nil
}
