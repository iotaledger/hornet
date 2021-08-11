package dag

import (
	"container/list"
	"fmt"
	"sync"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
)

type ParentTraverser struct {
	storage          *storage.Storage
	metadataMemcache *storage.MetadataMemcache

	// stack holding the ordered msg to process
	stack *list.List

	// processed map with already processed messages
	processed map[string]struct{}

	// checked map with result of traverse condition
	checked map[string]bool

	condition                Predicate
	consumer                 Consumer
	onMissingParent          OnMissingParent
	onSolidEntryPoint        OnSolidEntryPoint
	traverseSolidEntryPoints bool
	abortSignal              <-chan struct{}

	traverserLock sync.Mutex
}

// NewParentTraverser create a new traverser to traverse the parents (past cone)
func NewParentTraverser(s *storage.Storage, metadataMemcache ...*storage.MetadataMemcache) *ParentTraverser {

	t := &ParentTraverser{
		storage:          s,
		metadataMemcache: storage.NewMetadataMemcache(s),
		stack:            list.New(),
		processed:        make(map[string]struct{}),
		checked:          make(map[string]bool),
	}

	if len(metadataMemcache) > 0 && metadataMemcache[0] != nil {
		// use the memcache from outside to share the same cached metadata
		t.metadataMemcache = metadataMemcache[0]
	}

	return t
}

func (t *ParentTraverser) reset() {

	t.processed = make(map[string]struct{})
	t.checked = make(map[string]bool)
	t.stack = list.New()
}

// Cleanup releases all the cached objects that have been traversed.
// This MUST be called by the user at the end.
func (t *ParentTraverser) Cleanup(forceRelease bool) {
	t.metadataMemcache.Cleanup(forceRelease)
}

// Traverse starts to traverse the parents (past cone) in the given order until
// the traversal stops due to no more messages passing the given condition.
// It is a DFS of the paths of the parents one after another.
// Caution: condition func is not in DFS order
func (t *ParentTraverser) Traverse(parents hornet.MessageIDs, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool, abortSignal <-chan struct{}) error {

	// make sure only one traversal is running
	t.traverserLock.Lock()

	// release lock so the traverser can be reused
	defer t.traverserLock.Unlock()

	t.condition = condition
	t.consumer = consumer
	t.onMissingParent = onMissingParent
	t.onSolidEntryPoint = onSolidEntryPoint
	t.traverseSolidEntryPoints = traverseSolidEntryPoints
	t.abortSignal = abortSignal

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
func (t *ParentTraverser) processStackParents() error {

	select {
	case <-t.abortSignal:
		return common.ErrOperationAborted
	default:
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
	if t.storage.SolidEntryPointsContain(currentMessageID) {
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

	cachedMsgMeta := t.metadataMemcache.CachedMetadataOrNil(currentMessageID) // meta +1
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

	traverse, checkedBefore := t.checked[currentMessageIDMapKey]
	if !checkedBefore {
		var err error

		// check condition to decide if msg should be consumed and traversed
		traverse, err = t.condition(cachedMsgMeta.Retain()) // meta + 1
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
		if err := t.consumer(cachedMsgMeta.Retain()); err != nil { // meta +1
			// there was an error, stop processing the stack
			return err
		}
	}

	return nil
}
