package dag

import (
	"container/list"
	"sync"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

type ApproversTraverser struct {
	cachedTxMetas map[string]*tangle.CachedMetadata

	// stack holding the ordered tx to process
	stack *list.List

	// discovers map with already found transactions
	discovered map[string]struct{}

	condition             Predicate
	consumer              Consumer
	walkAlreadyDiscovered bool
	abortSignal           <-chan struct{}

	traverserLock sync.Mutex
}

// NewApproversTraverser create a new traverser to traverse the approvers (future cone)
func NewApproversTraverser(condition Predicate, consumer Consumer, walkAlreadyDiscovered bool, abortSignal <-chan struct{}) *ApproversTraverser {

	return &ApproversTraverser{
		condition:             condition,
		consumer:              consumer,
		walkAlreadyDiscovered: walkAlreadyDiscovered,
		abortSignal:           abortSignal,
	}
}

func (t *ApproversTraverser) cleanup(forceRelease bool) {

	// release all tx metadata at the end
	for _, cachedTxMeta := range t.cachedTxMetas {
		cachedTxMeta.Release(forceRelease) // meta -1
	}

	// Release lock after cleanup so the traverser can be reused
	t.traverserLock.Unlock()
}

func (t *ApproversTraverser) reset() {

	t.cachedTxMetas = make(map[string]*tangle.CachedMetadata)
	t.discovered = make(map[string]struct{})
	t.stack = list.New()
}

// Traverse starts to traverse the approvers (future cone) of the given start transaction until
// the traversal stops due to no more transactions passing the given condition.
// It is unsorted BFS because the approvers are not ordered in the database.
func (t *ApproversTraverser) Traverse(startTxHash hornet.Hash) error {

	// make sure only one traversal is running
	t.traverserLock.Lock()

	// Prepare for a new traversal
	t.reset()

	defer t.cleanup(true)

	t.stack.PushFront(startTxHash)
	if !t.walkAlreadyDiscovered {
		t.discovered[string(startTxHash)] = struct{}{}
	}

	for t.stack.Len() > 0 {
		if err := t.processStackApprovers(); err != nil {
			return err
		}
	}

	return nil
}

// processStackApprovers checks if the current element in the stack must be processed and traversed.
// current element gets consumed first, afterwards it's approvers get traversed in random order.
func (t *ApproversTraverser) processStackApprovers() error {

	select {
	case <-t.abortSignal:
		return tangle.ErrOperationAborted
	default:
	}

	// load candidate tx
	ele := t.stack.Front()
	currentTxHash := ele.Value.(hornet.Hash)

	// remove the transaction from the stack
	t.stack.Remove(ele)

	cachedTxMeta, exists := t.cachedTxMetas[string(currentTxHash)]
	if !exists {
		cachedTxMeta = tangle.GetCachedTxMetadataOrNil(currentTxHash) // meta +1
		if cachedTxMeta == nil {
			// there was an error, stop processing the stack
			return errors.Wrapf(tangle.ErrTransactionNotFound, "hash: %s", currentTxHash.Trytes())
		}
		t.cachedTxMetas[string(currentTxHash)] = cachedTxMeta
	}

	// check condition to decide if tx should be consumed and traversed
	traverse, err := t.condition(cachedTxMeta.Retain()) // meta + 1
	if err != nil {
		// there was an error, stop processing the stack
		return err
	}

	if !traverse {
		// transaction will not get consumed and approvers are not traversed
		return nil
	}

	if t.consumer != nil {
		// consume the transaction
		if err := t.consumer(cachedTxMeta.Retain()); err != nil { // meta +1
			// there was an error, stop processing the stack
			return err
		}
	}

	for _, approverHash := range tangle.GetApproverHashes(currentTxHash) {
		if !t.walkAlreadyDiscovered {
			if _, approverDiscovered := t.discovered[string(approverHash)]; approverDiscovered {
				// approver was already discovered
				continue
			}

			t.discovered[string(approverHash)] = struct{}{}
		}

		// traverse the approver
		t.stack.PushBack(approverHash)
	}

	return nil
}
