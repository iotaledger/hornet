package dag

import (
	"context"

	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	iotago "github.com/iotaledger/iota.go/v3"
)

// Predicate defines whether a traversal should continue or not.
type Predicate func(cachedBlockMeta *storage.CachedMetadata) (bool, error)

// Consumer consumes the given block metadata during traversal.
type Consumer func(cachedBlockMeta *storage.CachedMetadata) error

// OnMissingParent gets called when during traversal a parent is missing.
type OnMissingParent func(parentBlockID iotago.BlockID) error

// OnSolidEntryPoint gets called when during traversal the startBlock or parent is a solid entry point.
type OnSolidEntryPoint func(blockID iotago.BlockID) error

// TraverseParents starts to traverse the parents (past cone) in the given order until
// the traversal stops due to no more blocks passing the given condition.
// It is a DFS of the paths of the parents one after another.
// Caution: condition func is not in DFS order.
func TraverseParents(ctx context.Context, parentsTraverserStorage ParentsTraverserStorage, parents iotago.BlockIDs, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool) error {
	t := NewParentsTraverser(parentsTraverserStorage)

	return t.Traverse(ctx, parents, condition, consumer, onMissingParent, onSolidEntryPoint, traverseSolidEntryPoints)
}

// TraverseParentsOfBlock starts to traverse the parents (past cone) of the given start block until
// the traversal stops due to no more blocks passing the given condition.
// It is a DFS of the paths of the parents one after another.
// Caution: condition func is not in DFS order.
func TraverseParentsOfBlock(ctx context.Context, parentsTraverserStorage ParentsTraverserStorage, startBlockID iotago.BlockID, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool) error {
	t := NewParentsTraverser(parentsTraverserStorage)

	return t.Traverse(ctx, iotago.BlockIDs{startBlockID}, condition, consumer, onMissingParent, onSolidEntryPoint, traverseSolidEntryPoints)
}

// TraverseChildren starts to traverse the children (future cone) of the given start block until
// the traversal stops due to no more blocks passing the given condition.
// It is unsorted BFS because the children are not ordered in the database.
func TraverseChildren(ctx context.Context, childrenTraverserStorage ChildrenTraverserStorage, startBlockID iotago.BlockID, condition Predicate, consumer Consumer, walkAlreadyDiscovered bool) error {
	t := NewChildrenTraverser(childrenTraverserStorage)

	return t.Traverse(ctx, startBlockID, condition, consumer, walkAlreadyDiscovered)
}
