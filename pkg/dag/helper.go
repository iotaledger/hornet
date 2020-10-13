package dag

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

// Predicate defines whether a traversal should continue or not.
type Predicate func(cachedMetadata *tangle.CachedMetadata) (bool, error)

// Consumer consumes the given message metadata during traversal.
type Consumer func(cachedMetadata *tangle.CachedMetadata) error

// OnMissingParent gets called when during traversal a parent is missing.
type OnMissingParent func(parentMessageID *hornet.MessageID) error

// OnSolidEntryPoint gets called when during traversal the startMsg or parent is a solid entry point.
type OnSolidEntryPoint func(messageID *hornet.MessageID)

// TraverseParent1AndParent2 starts to traverse the parents (past cone) of the given parent1 message until
// the traversal stops due to no more messages passing the given condition.
// Afterwards it traverses the parents (past cone) of the given parent2 message.
// It is a DFS with parent1 / parent2.
// Caution: condition func is not in DFS order
func TraverseParent1AndParent2(parent1MessageID *hornet.MessageID, parent2MessageID *hornet.MessageID, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool, abortSignal <-chan struct{}) error {

	t := NewParentTraverser(condition, consumer, onMissingParent, onSolidEntryPoint, abortSignal)
	return t.TraverseParent1AndParent2(parent1MessageID, parent2MessageID, traverseSolidEntryPoints)
}

// TraverseParents starts to traverse the parents (past cone) of the given start message until
// the traversal stops due to no more messages passing the given condition.
// It is a DFS with parent1 / parent2.
// Caution: condition func is not in DFS order
func TraverseParents(startMessageID *hornet.MessageID, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool, abortSignal <-chan struct{}) error {

	t := NewParentTraverser(condition, consumer, onMissingParent, onSolidEntryPoint, abortSignal)
	return t.Traverse(startMessageID, traverseSolidEntryPoints)
}

// TraverseChildren starts to traverse the children (future cone) of the given start message until
// the traversal stops due to no more messages passing the given condition.
// It is unsorted BFS because the children are not ordered in the database.
func TraverseChildren(startMessageID *hornet.MessageID, condition Predicate, consumer Consumer, walkAlreadyDiscovered bool, abortSignal <-chan struct{}) error {

	t := NewChildrenTraverser(condition, consumer, walkAlreadyDiscovered, abortSignal)
	return t.Traverse(startMessageID)
}
