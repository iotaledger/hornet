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
	storage          *storage.Storage
	metadataMemcache *storage.MetadataMemcache

	// stack holding the ordered msg to process
	stack *list.List

	// discovers map with already found messages
	discovered map[string]struct{}

	iteratorOptions       []storage.IteratorOption
	condition             Predicate
	consumer              Consumer
	walkAlreadyDiscovered bool
	abortSignal           <-chan struct{}

	traverserLock sync.Mutex
}

// NewChildrenTraverser create a new traverser to traverse the children (future cone)
func NewChildrenTraverser(s *storage.Storage, metadataMemcache ...*storage.MetadataMemcache) *ChildrenTraverser {

	t := &ChildrenTraverser{
		storage:          s,
		metadataMemcache: storage.NewMetadataMemcache(s),
		stack:            list.New(),
		discovered:       make(map[string]struct{}),
	}

	if len(metadataMemcache) > 0 && metadataMemcache[0] != nil {
		// use the memcache from outside to share the same cached metadata
		t.metadataMemcache = metadataMemcache[0]
	}

	return t
}

func (t *ChildrenTraverser) reset() {

	t.discovered = make(map[string]struct{})
	t.stack = list.New()
}

// Cleanup releases all the cached objects that have been traversed.
// This MUST be called by the user at the end.
func (t *ChildrenTraverser) Cleanup(forceRelease bool) {
	t.metadataMemcache.Cleanup(forceRelease)
}

// Traverse starts to traverse the children (future cone) of the given start message until
// the traversal stops due to no more messages passing the given condition.
// It is unsorted BFS because the children are not ordered in the database.
func (t *ChildrenTraverser) Traverse(startMessageID hornet.MessageID, condition Predicate, consumer Consumer, walkAlreadyDiscovered bool, abortSignal <-chan struct{}, iteratorOptions ...storage.IteratorOption) error {

	// make sure only one traversal is running
	t.traverserLock.Lock()

	// release lock so the traverser can be reused
	defer t.traverserLock.Unlock()

	t.iteratorOptions = iteratorOptions
	t.condition = condition
	t.consumer = consumer
	t.walkAlreadyDiscovered = walkAlreadyDiscovered
	t.abortSignal = abortSignal

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

	select {
	case <-t.abortSignal:
		return common.ErrOperationAborted
	default:
	}

	// load candidate msg
	ele := t.stack.Front()
	currentMessageID := ele.Value.(hornet.MessageID)

	// remove the message from the stack
	t.stack.Remove(ele)

	cachedMsgMeta := t.metadataMemcache.CachedMetadataOrNil(currentMessageID) // meta +1
	if cachedMsgMeta == nil {
		// there was an error, stop processing the stack
		return errors.Wrapf(common.ErrMessageNotFound, "message ID: %s", currentMessageID.ToHex())
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

	for _, childMessageID := range t.storage.ChildrenMessageIDs(currentMessageID, t.iteratorOptions...) {
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
