package dag

import (
	"container/list"
	"sync"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
)

type ChildrenTraverser struct {
	cachedMsgMetas map[string]*storage.CachedMetadata

	storage *storage.Storage

	// stack holding the ordered msg to process
	stack *list.List

	// discovers map with already found messages
	discovered map[string]struct{}

	condition             Predicate
	consumer              Consumer
	walkAlreadyDiscovered bool
	abortSignal           <-chan struct{}

	traverserLock sync.Mutex
}

// NewChildrenTraverser create a new traverser to traverse the children (future cone)
func NewChildrenTraverser(storage *storage.Storage, condition Predicate, consumer Consumer, walkAlreadyDiscovered bool, abortSignal <-chan struct{}) *ChildrenTraverser {

	return &ChildrenTraverser{
		storage:               storage,
		condition:             condition,
		consumer:              consumer,
		walkAlreadyDiscovered: walkAlreadyDiscovered,
		abortSignal:           abortSignal,
	}
}

func (t *ChildrenTraverser) cleanup(forceRelease bool) {

	// release all msg metadata at the end
	for _, cachedMsgMeta := range t.cachedMsgMetas {
		cachedMsgMeta.Release(forceRelease) // meta -1
	}

	// Release lock after cleanup so the traverser can be reused
	t.traverserLock.Unlock()
}

func (t *ChildrenTraverser) reset() {

	t.cachedMsgMetas = make(map[string]*storage.CachedMetadata)
	t.discovered = make(map[string]struct{})
	t.stack = list.New()
}

// Traverse starts to traverse the children (future cone) of the given start message until
// the traversal stops due to no more messages passing the given condition.
// It is unsorted BFS because the children are not ordered in the database.
func (t *ChildrenTraverser) Traverse(startMessageID *hornet.MessageID) error {

	// make sure only one traversal is running
	t.traverserLock.Lock()

	// Prepare for a new traversal
	t.reset()

	defer t.cleanup(true)

	t.stack.PushFront(startMessageID)
	if !t.walkAlreadyDiscovered {
		t.discovered[startMessageID.MapKey()] = struct{}{}
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

	select {
	case <-t.abortSignal:
		return common.ErrOperationAborted
	default:
	}

	// load candidate msg
	ele := t.stack.Front()
	currentMessageID := ele.Value.(*hornet.MessageID)
	currentMessageIDMapKey := currentMessageID.MapKey()

	// remove the message from the stack
	t.stack.Remove(ele)

	cachedMsgMeta, exists := t.cachedMsgMetas[currentMessageIDMapKey]
	if !exists {
		cachedMsgMeta = t.storage.GetCachedMessageMetadataOrNil(currentMessageID) // meta +1
		if cachedMsgMeta == nil {
			// there was an error, stop processing the stack
			return errors.Wrapf(common.ErrMessageNotFound, "message ID: %s", currentMessageID.Hex())
		}
		t.cachedMsgMetas[currentMessageIDMapKey] = cachedMsgMeta
	}

	// check condition to decide if msg should be consumed and traversed
	traverse, err := t.condition(cachedMsgMeta.Retain()) // meta + 1
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
		if err := t.consumer(cachedMsgMeta.Retain()); err != nil { // meta +1
			// there was an error, stop processing the stack
			return err
		}
	}

	for _, childMessageID := range t.storage.GetChildrenMessageIDs(currentMessageID) {
		if !t.walkAlreadyDiscovered {
			childMessageIDMapKey := childMessageID.MapKey()
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
