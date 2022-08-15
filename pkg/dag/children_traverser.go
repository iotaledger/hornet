package dag

import (
	"container/list"
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/contextutils"
	"github.com/iotaledger/hornet/v2/pkg/common"
	iotago "github.com/iotaledger/iota.go/v3"
)

// ChildrenTraverser can be used to walk the dag in direction of the tips (future cone).
type ChildrenTraverser struct {
	// interface to the used storage.
	childrenTraverserStorage ChildrenTraverserStorage

	// stack holding the ordered blocks to process.
	stack *list.List

	// discovers map with already found blocks.
	discovered map[iotago.BlockID]struct{}

	ctx                   context.Context
	condition             Predicate
	consumer              Consumer
	walkAlreadyDiscovered bool

	traverserLock sync.Mutex
}

// NewChildrenTraverser create a new traverser to traverse the children (future cone).
func NewChildrenTraverser(childrenTraverserStorage ChildrenTraverserStorage) *ChildrenTraverser {

	t := &ChildrenTraverser{
		childrenTraverserStorage: childrenTraverserStorage,
		stack:                    list.New(),
		discovered:               make(map[iotago.BlockID]struct{}),
	}

	return t
}

// reset the traverser for the next walk.
func (t *ChildrenTraverser) reset() {

	t.discovered = make(map[iotago.BlockID]struct{})
	t.stack = list.New()
}

// Traverse starts to traverse the children (future cone) of the given start block until
// the traversal stops due to no more blocks passing the given condition.
// It is unsorted BFS because the children are not ordered in the database.
func (t *ChildrenTraverser) Traverse(ctx context.Context, startBlockID iotago.BlockID, condition Predicate, consumer Consumer, walkAlreadyDiscovered bool) error {

	// make sure only one traversal is running
	t.traverserLock.Lock()

	// release lock so the traverser can be reused
	defer t.traverserLock.Unlock()

	t.ctx = ctx
	t.condition = condition
	t.consumer = consumer
	t.walkAlreadyDiscovered = walkAlreadyDiscovered

	// Prepare for a new traversal
	t.reset()

	t.stack.PushFront(startBlockID)
	if !t.walkAlreadyDiscovered {
		t.discovered[startBlockID] = struct{}{}
	}

	for t.stack.Len() > 0 {
		if err := t.processStackChildren(); err != nil {
			return err
		}
	}

	return nil
}

// processStackChildren checks if the current element in the stack must be processed and traversed.
// current element gets consumed first, afterwards it's children get traversed in random order.
func (t *ChildrenTraverser) processStackChildren() error {

	if err := contextutils.ReturnErrIfCtxDone(t.ctx, common.ErrOperationAborted); err != nil {
		return err
	}

	// load candidate block
	ele := t.stack.Front()
	currentBlockID, ok := ele.Value.(iotago.BlockID)
	if !ok {
		return fmt.Errorf("expected iotago.BlockID, got %T", ele.Value)
	}

	// remove the block from the stack
	t.stack.Remove(ele)

	// we also need to walk the children of solid entry points, but we don't consume them
	contains, err := t.childrenTraverserStorage.SolidEntryPointsContain(currentBlockID)
	if err != nil {
		return err
	}

	if !contains {
		cachedBlockMeta, err := t.childrenTraverserStorage.CachedBlockMetadata(currentBlockID) // meta +1
		if err != nil {
			return err
		}

		if cachedBlockMeta == nil {
			// there was an error, stop processing the stack
			return errors.Wrapf(common.ErrBlockNotFound, "block ID: %s", currentBlockID.ToHex())
		}
		defer cachedBlockMeta.Release(true) // meta -1

		// check condition to decide if block should be consumed and traversed
		traverse, err := t.condition(cachedBlockMeta.Retain()) // meta pass +1
		if err != nil {
			// there was an error, stop processing the stack
			return err
		}

		if !traverse {
			// block will not get consumed and children are not traversed
			return nil
		}

		if t.consumer != nil {
			// consume the block
			if err := t.consumer(cachedBlockMeta.Retain()); err != nil { // meta pass +1
				// there was an error, stop processing the stack
				return err
			}
		}
	}

	childrenBlockIDs, err := t.childrenTraverserStorage.ChildrenBlockIDs(currentBlockID)
	if err != nil {
		return err
	}

	for _, childBlockID := range childrenBlockIDs {
		if !t.walkAlreadyDiscovered {
			if _, childDiscovered := t.discovered[childBlockID]; childDiscovered {
				// child was already discovered
				continue
			}

			t.discovered[childBlockID] = struct{}{}
		}

		// traverse the child
		t.stack.PushBack(childBlockID)
	}

	return nil
}
