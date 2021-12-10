package dag

import (
	"context"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
)

// Predicate defines whether a traversal should continue or not.
type Predicate func(cachedMetadata *storage.CachedMetadata) (bool, error)

// Consumer consumes the given message metadata during traversal.
type Consumer func(cachedMetadata *storage.CachedMetadata) error

// OnMissingParent gets called when during traversal a parent is missing.
type OnMissingParent func(parentMessageID hornet.MessageID) error

// OnSolidEntryPoint gets called when during traversal the startMsg or parent is a solid entry point.
type OnSolidEntryPoint func(messageID hornet.MessageID)

// TraverseParents starts to traverse the parents (past cone) in the given order until
// the traversal stops due to no more messages passing the given condition.
// It is a DFS of the paths of the parents one after another.
// Caution: condition func is not in DFS order
func TraverseParents(ctx context.Context, dbStorage *storage.Storage, parents hornet.MessageIDs, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool) error {

	t := NewParentTraverser(dbStorage)
	defer t.Cleanup(true)

	return t.Traverse(ctx, parents, condition, consumer, onMissingParent, onSolidEntryPoint, traverseSolidEntryPoints)
}

// TraverseParentsOfMessage starts to traverse the parents (past cone) of the given start message until
// the traversal stops due to no more messages passing the given condition.
// It is a DFS of the paths of the parents one after another.
// Caution: condition func is not in DFS order
func TraverseParentsOfMessage(ctx context.Context, dbStorage *storage.Storage, startMessageID hornet.MessageID, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool) error {

	t := NewParentTraverser(dbStorage)
	defer t.Cleanup(true)

	return t.Traverse(ctx, hornet.MessageIDs{startMessageID}, condition, consumer, onMissingParent, onSolidEntryPoint, traverseSolidEntryPoints)
}

// TraverseChildren starts to traverse the children (future cone) of the given start message until
// the traversal stops due to no more messages passing the given condition.
// It is unsorted BFS because the children are not ordered in the database.
func TraverseChildren(ctx context.Context, dbStorage *storage.Storage, startMessageID hornet.MessageID, condition Predicate, consumer Consumer, walkAlreadyDiscovered bool, iteratorOptions ...storage.IteratorOption) error {

	t := NewChildrenTraverser(dbStorage)
	defer t.Cleanup(true)

	return t.Traverse(ctx, startMessageID, condition, consumer, walkAlreadyDiscovered, iteratorOptions...)
}
