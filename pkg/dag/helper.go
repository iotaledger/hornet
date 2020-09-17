package dag

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

// Predicate defines whether a traversal should continue or not.
type Predicate func(cachedMetadata *tangle.CachedMetadata) (bool, error)

// Consumer consumes the given transaction metadata during traversal.
type Consumer func(cachedMetadata *tangle.CachedMetadata) error

// OnMissingParent gets called when during traversal a parent is missing.
type OnMissingParent func(parentMessageID hornet.Hash) error

// OnSolidEntryPoint gets called when during traversal the startMsg or parent is a solid entry point.
type OnSolidEntryPoint func(messageID hornet.Hash)

// TraverseParent1AndParent2 starts to traverse the approvees (past cone) of the given trunk transaction until
// the traversal stops due to no more transactions passing the given condition.
// Afterwards it traverses the approvees (past cone) of the given branch transaction.
// It is a DFS with trunk / branch.
// Caution: condition func is not in DFS order
func TraverseParent1AndParent2(parent1MessageID hornet.Hash, parent2MessageID hornet.Hash, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool, abortSignal <-chan struct{}) error {

	t := NewParentTraverser(condition, consumer, onMissingParent, onSolidEntryPoint, abortSignal)
	return t.TraverseParent1AndParent2(parent1MessageID, parent2MessageID, traverseSolidEntryPoints)
}

// TraverseParents starts to traverse the approvees (past cone) of the given start transaction until
// the traversal stops due to no more transactions passing the given condition.
// It is a DFS with trunk / branch.
// Caution: condition func is not in DFS order
func TraverseParents(startMessageID hornet.Hash, condition Predicate, consumer Consumer, onMissingApprovee OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool, abortSignal <-chan struct{}) error {

	t := NewParentTraverser(condition, consumer, onMissingApprovee, onSolidEntryPoint, abortSignal)
	return t.Traverse(startMessageID, traverseSolidEntryPoints)
}

// TraverseChildren starts to traverse the approvers (future cone) of the given start transaction until
// the traversal stops due to no more transactions passing the given condition.
// It is unsorted BFS because the approvers are not ordered in the database.
func TraverseChildren(startMessageID hornet.Hash, condition Predicate, consumer Consumer, walkAlreadyDiscovered bool, abortSignal <-chan struct{}) error {

	t := NewChildrenTraverser(condition, consumer, walkAlreadyDiscovered, abortSignal)
	return t.Traverse(startMessageID)
}
