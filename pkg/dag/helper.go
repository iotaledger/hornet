package dag

import (
	"bytes"
	"container/list"

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

// processStackApprovees checks if the current element in the stack must be processed or traversed.
// first the trunk is traversed, then the branch.
func processStackApprovees(stack *list.List, visited map[string]struct{}, condition Predicate, consumer Consumer, onMissingApprovee OnMissingApprovee, onSolidEntryPoint OnSolidEntryPoint, forceRelease bool, traverseSolidEntryPoints bool, abortSignal <-chan struct{}) error {

	select {
	case <-abortSignal:
		return tangle.ErrOperationAborted
	default:
	}

	// load candidate tx
	ele := stack.Front()
	currentTxHash := ele.Value.(hornet.Hash)
	cachedTx := tangle.GetCachedTransactionOrNil(currentTxHash) // tx +1
	if cachedTx == nil {
		// remove the transaction from the stack, trunk and branch are not traversed
		visited[string(currentTxHash)] = struct{}{}
		stack.Remove(ele)

		// stop processing the stack if the caller returns an error
		return onMissingApprovee(currentTxHash)
	}

	defer cachedTx.Release(forceRelease) // tx -1

	// check condition to decide if tx should be consumed and traversed
	traverse, err := condition(cachedTx.Retain()) // tx + 1
	if err != nil {
		// there was an error, stop processing the stack
		return err
	}

	if !traverse {
		// remove the transaction from the stack, trunk and branch are not traversed
		// transaction will not get consumed
		visited[string(currentTxHash)] = struct{}{}
		stack.Remove(ele)
		return nil
	}

	trunkHash := cachedTx.GetTransaction().GetTrunkHash()
	branchHash := cachedTx.GetTransaction().GetBranchHash()

	if _, trunkVisited := visited[string(trunkHash)]; !trunkVisited {
		if !tangle.SolidEntryPointsContain(trunkHash) {
			// traverse this transaction
			stack.PushFront(trunkHash)
			return nil
		}

		onSolidEntryPoint(trunkHash)
		if traverseSolidEntryPoints {
			// traverse this transaction
			stack.PushFront(trunkHash)
			return nil
		}
	}

	if !bytes.Equal(trunkHash, branchHash) {
		if _, branchVisited := visited[string(branchHash)]; !branchVisited {
			if !tangle.SolidEntryPointsContain(branchHash) {
				// traverse this transaction
				stack.PushFront(branchHash)
				return nil
			}

			onSolidEntryPoint(branchHash)
			if traverseSolidEntryPoints {
				// traverse this transaction
				stack.PushFront(branchHash)
				return nil
			}
		}
	}

	// remove the transaction from the stack
	visited[string(currentTxHash)] = struct{}{}
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
func TraverseApprovees(startTxHash hornet.Hash, condition Predicate, consumer Consumer, onMissingApprovee OnMissingApprovee, onSolidEntryPoint OnSolidEntryPoint, forceRelease bool, traverseSolidEntryPoints bool, abortSignal <-chan struct{}) error {

	stack := list.New()
	stack.PushFront(startTxHash)

	visited := make(map[string]struct{})

	for stack.Len() > 0 {
		if err := processStackApprovees(stack, visited, condition, consumer, onMissingApprovee, onSolidEntryPoint, forceRelease, traverseSolidEntryPoints, abortSignal); err != nil {
			return err
		}
	}

	return nil
}

// processStackApprovers checks if the current element in the stack must be processed and traversed.
// current element gets consumed first, afterwards it's approvers get traversed in random order.
func processStackApprovers(stack *list.List, visited map[string]struct{}, condition Predicate, consumer Consumer, forceRelease bool, abortSignal <-chan struct{}) error {

	select {
	case <-abortSignal:
		return tangle.ErrOperationAborted
	default:
	}

	// load candidate tx
	ele := stack.Front()
	currentTxHash := ele.Value.(hornet.Hash)
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

	// remove the transaction from the stack
	visited[string(currentTxHash)] = struct{}{}
	stack.Remove(ele)

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
		if _, approverVisited := visited[string(approverHash)]; approverVisited {
			// approver was already visited
			continue
		}

		// traverse the approver
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

	visited := make(map[string]struct{})

	for stack.Len() > 0 {
		if err := processStackApprovers(stack, visited, condition, consumer, forceRelease, abortSignal); err != nil {
			return err
		}
	}

	return nil
}
