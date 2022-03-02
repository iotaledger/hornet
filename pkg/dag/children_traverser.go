package dag

import (
	"container/list"
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/utils"
)

// ChildrenTraverser can be used to walk the dag in direction of the tips (future cone).
type ChildrenTraverser struct {
	// interface to the used storage.
	childrenTraverserStorage ChildrenTraverserStorage

	// stack holding the ordered msg to process.
	stack *list.List

	// discovers map with already found messages.
	discovered map[string]struct{}

	ctx                   context.Context
	condition             Predicate
	consumer              Consumer
	walkAlreadyDiscovered bool

	traverserLock sync.Mutex
}

// NewChildrenTraverser create a new traverser to traverse the children (future cone)
func NewChildrenTraverser(childrenTraverserStorage ChildrenTraverserStorage) *ChildrenTraverser {

	t := &ChildrenTraverser{
		childrenTraverserStorage: childrenTraverserStorage,
		stack:                    list.New(),
		discovered:               make(map[string]struct{}),
	}

	return t
}

// reset the traverser for the next walk.
func (t *ChildrenTraverser) reset() {

	t.discovered = make(map[string]struct{})
	t.stack = list.New()
}

// Traverse starts to traverse the children (future cone) of the given start message until
// the traversal stops due to no more messages passing the given condition.
// It is unsorted BFS because the children are not ordered in the database.
func (t *ChildrenTraverser) Traverse(ctx context.Context, startMessageID hornet.MessageID, condition Predicate, consumer Consumer, walkAlreadyDiscovered bool) error {

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

	t.stack.PushFront(startMessageID)
	if !t.walkAlreadyDiscovered {
		t.discovered[startMessageID.ToMapKey()] = struct{}{}
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

	if err := utils.ReturnErrIfCtxDone(t.ctx, common.ErrOperationAborted); err != nil {
		return err
	}

	// load candidate msg
	ele := t.stack.Front()
	currentMessageID := ele.Value.(hornet.MessageID)

	// remove the message from the stack
	t.stack.Remove(ele)

	cachedMsgMeta, err := t.childrenTraverserStorage.CachedMessageMetadata(currentMessageID) // meta +1
	if err != nil {
		return err
	}

	if cachedMsgMeta == nil {
		// there was an error, stop processing the stack
		return errors.Wrapf(common.ErrMessageNotFound, "message ID: %s", currentMessageID.ToHex())
	}
	defer cachedMsgMeta.Release(true) // meta -1

	// check condition to decide if msg should be consumed and traversed
	traverse, err := t.condition(cachedMsgMeta.Retain()) // meta pass +1
	if err != nil {
		// there was an error, stop processing the stack
		return err
	}

	if !traverse {
		// message will not get consumed and children are not traversed
		return nil
	}

	if t.consumer != nil {
		// consume the message
		if err := t.consumer(cachedMsgMeta.Retain()); err != nil { // meta pass +1
			// there was an error, stop processing the stack
			return err
		}
	}

	childrenMessageIDs, err := t.childrenTraverserStorage.ChildrenMessageIDs(currentMessageID)
	if err != nil {
		return err
	}

	for _, childMessageID := range childrenMessageIDs {
		if !t.walkAlreadyDiscovered {
			childMessageIDMapKey := childMessageID.ToMapKey()
			if _, childDiscovered := t.discovered[childMessageIDMapKey]; childDiscovered {
				// child was already discovered
				continue
			}

			t.discovered[childMessageIDMapKey] = struct{}{}
		}

		// traverse the child
		t.stack.PushBack(childMessageID)
	}

	return nil
}
