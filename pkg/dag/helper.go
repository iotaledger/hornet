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
func FindAllTails(startTxHash hornet.Hash, skipStartTx bool) (map[string]struct{}, error) {

	tails := make(map[string]struct{})

	err := TraverseApprovees(startTxHash,
		// traversal stops if no more transactions pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedTxMeta *tangle.CachedMetadata) (bool, error) { // meta +1
			defer cachedTxMeta.Release(true) // meta -1

			if skipStartTx && bytes.Equal(startTxHash, cachedTxMeta.GetMetadata().GetTxHash()) {
				// skip the start tx
				return true, nil
			}

			if cachedTxMeta.GetMetadata().IsTail() {
				// transaction is a tail, do not traverse further
				tails[string(cachedTxMeta.GetMetadata().GetTxHash())] = struct{}{}
				return false, nil
			}

			return true, nil
		},
		// consumer
		func(cachedTxMeta *tangle.CachedMetadata) error { // meta +1
			defer cachedTxMeta.Release(true) // meta -1
			return nil
		},
		// called on missing approvees
		// return error on missing approvees
		nil,
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false, false, nil)

	return tails, err
}

// Predicate defines whether a traversal should continue or not.
type Predicate func(cachedTxMeta *tangle.CachedMetadata) (bool, error)

// Consumer consumes the given transaction metadata during traversal.
type Consumer func(cachedTxMeta *tangle.CachedMetadata) error

// OnMissingApprovee gets called when during traversal an approvee is missing.
type OnMissingApprovee func(approveeHash hornet.Hash) error

// OnSolidEntryPoint gets called when during traversal the startTx or approvee is a solid entry point.
type OnSolidEntryPoint func(txHash hornet.Hash)

// TraverseApproveesTrunkBranch starts to traverse the approvees (past cone) of the given trunk transaction until
// the traversal stops due to no more transactions passing the given condition.
// Afterwards it traverses the approvees (past cone) of the given branch transaction.
// It is a DFS with trunk / branch.
// Caution: condition func is not in DFS order
func TraverseApproveesTrunkBranch(trunkTxHash hornet.Hash, branchTxHash hornet.Hash, condition Predicate, consumer Consumer, onMissingApprovee OnMissingApprovee, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool, traverseTailsOnly bool, abortSignal <-chan struct{}) error {

	t := NewApproveesTraverser(condition, consumer, onMissingApprovee, onSolidEntryPoint, abortSignal)
	return t.TraverseTrunkAndBranch(trunkTxHash, branchTxHash, traverseSolidEntryPoints, traverseTailsOnly)
}

// TraverseApprovees starts to traverse the approvees (past cone) of the given start transaction until
// the traversal stops due to no more transactions passing the given condition.
// It is a DFS with trunk / branch.
// Caution: condition func is not in DFS order
func TraverseApprovees(startTxHash hornet.Hash, condition Predicate, consumer Consumer, onMissingApprovee OnMissingApprovee, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool, traverseTailsOnly bool, abortSignal <-chan struct{}) error {

	t := NewApproveesTraverser(condition, consumer, onMissingApprovee, onSolidEntryPoint, abortSignal)
	return t.Traverse(startTxHash, traverseSolidEntryPoints, traverseTailsOnly)
}

// TraverseApprovers starts to traverse the approvers (future cone) of the given start transaction until
// the traversal stops due to no more transactions passing the given condition.
// It is unsorted BFS because the approvers are not ordered in the database.
func TraverseApprovers(startTxHash hornet.Hash, condition Predicate, consumer Consumer, walkAlreadyDiscovered bool, abortSignal <-chan struct{}) error {

	t := NewApproversTraverser(condition, consumer, walkAlreadyDiscovered, abortSignal)
	return t.Traverse(startTxHash)
}
