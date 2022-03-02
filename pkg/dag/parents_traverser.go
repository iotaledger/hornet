package dag

import (
	"container/list"
	"context"
	"fmt"
	"sync"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/utils"
)

type ParentsTraverserInterface interface {
	Traverse(ctx context.Context, parents hornet.MessageIDs, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool) error
}

// ParentsTraverser can be used to walk the dag in direction of the parents (past cone).
type ParentsTraverser struct {
	// interface to the used storage
	parentsTraverserStorage ParentsTraverserStorage

	// stack holding the ordered msg to process
	stack *list.List

	// processed map with already processed messages
	processed map[string]struct{}

	// checked map with result of traverse condition
	checked map[string]bool

	ctx                      context.Context
	condition                Predicate
	consumer                 Consumer
	onMissingParent          OnMissingParent
	onSolidEntryPoint        OnSolidEntryPoint
	traverseSolidEntryPoints bool

	traverserLock sync.Mutex
}

// NewParentsTraverser create a new traverser to traverse the parents (past cone)
func NewParentsTraverser(parentsTraverserStorage ParentsTraverserStorage) *ParentsTraverser {

	t := &ParentsTraverser{
		parentsTraverserStorage: parentsTraverserStorage,
		stack:                   list.New(),
		processed:               make(map[string]struct{}),
		checked:                 make(map[string]bool),
	}

	return t
}

func (t *ParentsTraverser) reset() {

	t.processed = make(map[string]struct{})
	t.checked = make(map[string]bool)
	t.stack = list.New()
}

// Traverse starts to traverse the parents (past cone) in the given order until
// the traversal stops due to no more messages passing the given condition.
// It is a DFS of the paths of the parents one after another.
// Caution: condition func is not in DFS order
func (t *ParentsTraverser) Traverse(ctx context.Context, parents hornet.MessageIDs, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool) error {

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
	// the referenced parent message could for example already be processed
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

	if err := utils.ReturnErrIfCtxDone(t.ctx, common.ErrOperationAborted); err != nil {
		return err
	}

	// load candidate msg
	ele := t.stack.Front()
	currentMessageID := ele.Value.(hornet.MessageID)
	currentMessageIDMapKey := currentMessageID.ToMapKey()

	if _, wasProcessed := t.processed[currentMessageIDMapKey]; wasProcessed {
		// message was already processed
		// remove the message from the stack
		t.stack.Remove(ele)
		return nil
	}

	// check if the message is a solid entry point
	contains, err := t.parentsTraverserStorage.SolidEntryPointsContain(currentMessageID)
	if err != nil {
		return err
	}

	if contains {
		if t.onSolidEntryPoint != nil {
			t.onSolidEntryPoint(currentMessageID)
		}

		if !t.traverseSolidEntryPoints {
			// remove the message from the stack, the parents are not traversed
			t.processed[currentMessageIDMapKey] = struct{}{}
			delete(t.checked, currentMessageIDMapKey)
			t.stack.Remove(ele)
			return nil
		}
	}

	cachedMsgMeta, err := t.parentsTraverserStorage.CachedMessageMetadata(currentMessageID) // meta +1
	if err != nil {
		return err
	}

	if cachedMsgMeta == nil {
		// remove the message from the stack, the parents are not traversed
		t.processed[currentMessageIDMapKey] = struct{}{}
		delete(t.checked, currentMessageIDMapKey)
		t.stack.Remove(ele)

		if t.onMissingParent == nil {
			// stop processing the stack with an error
			return fmt.Errorf("%w: message %s", common.ErrMessageNotFound, currentMessageID.ToHex())
		}

		// stop processing the stack if the caller returns an error
		return t.onMissingParent(currentMessageID)
	}
	defer cachedMsgMeta.Release(true) // meta -1

	traverse, checkedBefore := t.checked[currentMessageIDMapKey]
	if !checkedBefore {
		var err error

		// check condition to decide if msg should be consumed and traversed
		traverse, err = t.condition(cachedMsgMeta.Retain()) // meta pass +1
		if err != nil {
			// there was an error, stop processing the stack
			return err
		}

		// mark the message as checked and remember the result of the traverse condition
		t.checked[currentMessageIDMapKey] = traverse
	}

	if !traverse {
		// remove the message from the stack, the parents are not traversed
		// parent will not get consumed
		t.processed[currentMessageIDMapKey] = struct{}{}
		delete(t.checked, currentMessageIDMapKey)
		t.stack.Remove(ele)
		return nil
	}

	for _, parentMessageID := range cachedMsgMeta.Metadata().Parents() {
		if _, parentProcessed := t.processed[parentMessageID.ToMapKey()]; !parentProcessed {
			// parent was not processed yet
			// traverse this message
			t.stack.PushFront(parentMessageID)
			return nil
		}
	}

	// remove the message from the stack
	t.processed[currentMessageIDMapKey] = struct{}{}
	delete(t.checked, currentMessageIDMapKey)
	t.stack.Remove(ele)

	if t.consumer != nil {
		// consume the message
		if err := t.consumer(cachedMsgMeta.Retain()); err != nil { // meta pass +1
			// there was an error, stop processing the stack
			return err
		}
	}

	return nil
}
