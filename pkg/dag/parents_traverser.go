package dag

import (
	"container/list"
	"context"
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/core/contextutils"
	"github.com/iotaledger/hornet/v2/pkg/common"
	iotago "github.com/iotaledger/iota.go/v3"
)

type ParentsTraverserInterface interface {
	Traverse(ctx context.Context, parents iotago.BlockIDs, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool) error
}

// ParentsTraverser can be used to walk the dag in direction of the parents (past cone).
type ParentsTraverser struct {
	// interface to the used storage.
	parentsTraverserStorage ParentsTraverserStorage

	// stack holding the ordered blocks to process.
	stack *list.List

	// processed map with already processed blocks.
	processed map[iotago.BlockID]struct{}

	// checked map with result of traverse condition.
	checked map[iotago.BlockID]bool

	ctx                      context.Context
	condition                Predicate
	consumer                 Consumer
	onMissingParent          OnMissingParent
	onSolidEntryPoint        OnSolidEntryPoint
	traverseSolidEntryPoints bool

	traverserLock sync.Mutex
}

// NewParentsTraverser create a new traverser to traverse the parents (past cone).
func NewParentsTraverser(parentsTraverserStorage ParentsTraverserStorage) *ParentsTraverser {

	t := &ParentsTraverser{
		parentsTraverserStorage: parentsTraverserStorage,
		stack:                   list.New(),
		processed:               make(map[iotago.BlockID]struct{}),
		checked:                 make(map[iotago.BlockID]bool),
	}

	return t
}

// reset the traverser for the next walk.
func (t *ParentsTraverser) reset() {

	t.processed = make(map[iotago.BlockID]struct{})
	t.checked = make(map[iotago.BlockID]bool)
	t.stack = list.New()
}

// Traverse starts to traverse the parents (past cone) in the given order until
// the traversal stops due to no more blocks passing the given condition.
// It is a DFS of the paths of the parents one after another.
// Caution: condition func is not in DFS order.
func (t *ParentsTraverser) Traverse(ctx context.Context, parents iotago.BlockIDs, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool) error {

	// make sure only one traversal is running
	t.traverserLock.Lock()

	// release lock so the traverser can be reused
	defer t.traverserLock.Unlock()

	t.ctx = ctx
	t.condition = condition
	t.consumer = consumer
	t.onMissingParent = onMissingParent
	t.onSolidEntryPoint = onSolidEntryPoint
	t.traverseSolidEntryPoints = traverseSolidEntryPoints

	// Prepare for a new traversal
	t.reset()

	// we feed the stack with the parents one after another,
	// to make sure that we examine all paths.
	// however, we only need to do it if the parent wasn't processed yet.
	// the referenced parent block could for example already be processed
	// if it is directly/indirectly approved by former parents.
	for _, parent := range parents {
		t.stack.PushFront(parent)

		for t.stack.Len() > 0 {
			if err := t.processStackParents(); err != nil {
				return err
			}
		}
	}

	return nil
}

// processStackParents checks if the current element in the stack must be processed or traversed.
// the paths of the parents are traversed one after another.
func (t *ParentsTraverser) processStackParents() error {

	if err := contextutils.ReturnErrIfCtxDone(t.ctx, common.ErrOperationAborted); err != nil {
		return err
	}

	// load candidate block
	ele := t.stack.Front()
	currentBlockID, ok := ele.Value.(iotago.BlockID)
	if !ok {
		return fmt.Errorf("expected iotago.BlockID, got %T", ele.Value)
	}

	if _, wasProcessed := t.processed[currentBlockID]; wasProcessed {
		// block was already processed
		// remove the block from the stack
		t.stack.Remove(ele)

		return nil
	}

	// check if the block is a solid entry point
	contains, err := t.parentsTraverserStorage.SolidEntryPointsContain(currentBlockID)
	if err != nil {
		return err
	}

	if contains {
		if t.onSolidEntryPoint != nil {
			if err := t.onSolidEntryPoint(currentBlockID); err != nil {
				return err
			}
		}

		if !t.traverseSolidEntryPoints {
			// remove the block from the stack, the parents are not traversed
			t.processed[currentBlockID] = struct{}{}
			delete(t.checked, currentBlockID)
			t.stack.Remove(ele)

			return nil
		}
	}

	cachedBlockMeta, err := t.parentsTraverserStorage.CachedBlockMetadata(currentBlockID) // meta +1
	if err != nil {
		return err
	}

	if cachedBlockMeta == nil {
		// remove the block from the stack, the parents are not traversed
		t.processed[currentBlockID] = struct{}{}
		delete(t.checked, currentBlockID)
		t.stack.Remove(ele)

		if t.onMissingParent == nil {
			// stop processing the stack with an error
			return fmt.Errorf("%w: block %s", common.ErrBlockNotFound, currentBlockID.ToHex())
		}

		// stop processing the stack if the caller returns an error
		return t.onMissingParent(currentBlockID)
	}
	defer cachedBlockMeta.Release(true) // meta -1

	traverse, checkedBefore := t.checked[currentBlockID]
	if !checkedBefore {
		var err error

		// check condition to decide if block should be consumed and traversed
		traverse, err = t.condition(cachedBlockMeta.Retain()) // meta pass +1
		if err != nil {
			// there was an error, stop processing the stack
			return err
		}

		// mark the block as checked and remember the result of the traverse condition
		t.checked[currentBlockID] = traverse
	}

	if !traverse {
		// remove the block from the stack, the parents are not traversed
		// parent will not get consumed
		t.processed[currentBlockID] = struct{}{}
		delete(t.checked, currentBlockID)
		t.stack.Remove(ele)

		return nil
	}

	for _, parentBlockID := range cachedBlockMeta.Metadata().Parents() {
		if _, parentProcessed := t.processed[parentBlockID]; !parentProcessed {
			// parent was not processed yet
			// traverse this block
			t.stack.PushFront(parentBlockID)

			return nil
		}
	}

	// remove the block from the stack
	t.processed[currentBlockID] = struct{}{}
	delete(t.checked, currentBlockID)
	t.stack.Remove(ele)

	if t.consumer != nil {
		// consume the block
		if err := t.consumer(cachedBlockMeta.Retain()); err != nil { // meta pass +1
			// there was an error, stop processing the stack
			return err
		}
	}

	return nil
}
