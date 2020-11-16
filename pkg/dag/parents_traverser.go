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
	cachedMessageMetas map[string]*storage.CachedMetadata

	storage *storage.Storage

	// stack holding the ordered msg to process
	stack *list.List

	// processed map with already processed messages
	processed map[string]struct{}

	// checked map with result of traverse condition
	checked map[string]bool

	condition         Predicate
	consumer          Consumer
	onMissingParent   OnMissingParent
	onSolidEntryPoint OnSolidEntryPoint
	abortSignal       <-chan struct{}

	traverseSolidEntryPoints bool

	traverserLock sync.Mutex
}

// NewParentTraverser create a new traverser to traverse the parents (past cone)
func NewParentTraverser(storage *storage.Storage, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, abortSignal <-chan struct{}) *ParentTraverser {

	return &ParentTraverser{
		storage:           storage,
		condition:         condition,
		consumer:          consumer,
		onMissingParent:   onMissingParent,
		onSolidEntryPoint: onSolidEntryPoint,
		abortSignal:       abortSignal,
	}
}

func (t *ParentTraverser) cleanup(forceRelease bool) {

	// release all msg metadata at the end
	for _, cachedMetadata := range t.cachedMessageMetas {
		cachedMetadata.Release(forceRelease) // meta -1
	}

	// Release lock after cleanup so the traverser can be reused
	t.traverserLock.Unlock()
}

func (t *ParentTraverser) reset() {

	t.cachedMessageMetas = make(map[string]*storage.CachedMetadata)
	t.processed = make(map[string]struct{})
	t.checked = make(map[string]bool)
	t.stack = list.New()
}

// Traverse starts to traverse the parents (past cone) of the given start message until
// the traversal stops due to no more messages passing the given condition.
// It is a DFS with parent1 / parent2.
// Caution: condition func is not in DFS order
func (t *ParentTraverser) Traverse(startMessageID *hornet.MessageID, traverseSolidEntryPoints bool) error {

	// make sure only one traversal is running
	t.traverserLock.Lock()

	// Prepare for a new traversal
	t.reset()

	t.traverseSolidEntryPoints = traverseSolidEntryPoints

	defer t.cleanup(true)

	t.stack.PushFront(startMessageID)
	for t.stack.Len() > 0 {
		if err := t.processStackParents(); err != nil {
			return err
		}
	}

	return nil
}

// TraverseParent1AndParent2 starts to traverse the parents (past cone) of the given parent1 until
// the traversal stops due to no more messages passing the given condition.
// Afterwards it traverses the parents (past cone) of the given parent2.
// It is a DFS with parent1 / parent2.
// Caution: condition func is not in DFS order
func (t *ParentTraverser) TraverseParent1AndParent2(parent1MessageID *hornet.MessageID, parent2MessageID *hornet.MessageID, traverseSolidEntryPoints bool) error {

	// make sure only one traversal is running
	t.traverserLock.Lock()

	// Prepare for a new traversal
	t.reset()

	t.traverseSolidEntryPoints = traverseSolidEntryPoints

	defer t.cleanup(true)

	t.stack.PushFront(parent1MessageID)
	for t.stack.Len() > 0 {
		if err := t.processStackParents(); err != nil {
			return err
		}
	}

	// since we first feed the stack the parent1,
	// we need to make sure that we also examine the parent2 path.
	// however, we only need to do it if the parent2 wasn't processed yet.
	// the referenced parent2 message could for example already be processed
	// if it is directly/indirectly approved by the parent1.
	t.stack.PushFront(parent2MessageID)
	for t.stack.Len() > 0 {
		if err := t.processStackParents(); err != nil {
			return err
		}
	}

	return nil
}

// processStackParents checks if the current element in the stack must be processed or traversed.
// first the parent1 is traversed, then the parent2.
func (t *ParentTraverser) processStackParents() error {

	select {
	case <-t.abortSignal:
		return common.ErrOperationAborted
	default:
	}

	// load candidate msg
	ele := t.stack.Front()
	currentMessageID := ele.Value.(*hornet.MessageID)
	currentMessageIDMapKey := currentMessageID.MapKey()

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
			// remove the message from the stack, parent1 and parent2 are not traversed
			t.processed[currentMessageIDMapKey] = struct{}{}
			delete(t.checked, currentMessageIDMapKey)
			t.stack.Remove(ele)
			return nil
		}
	}

	cachedMetadata, exists := t.cachedMessageMetas[currentMessageIDMapKey]
	if !exists {
		cachedMetadata = t.storage.GetCachedMessageMetadataOrNil(currentMessageID) // meta +1
		if cachedMetadata == nil {
			// remove the message from the stack, parent1 and parent2 are not traversed
			t.processed[currentMessageIDMapKey] = struct{}{}
			delete(t.checked, currentMessageIDMapKey)
			t.stack.Remove(ele)

			if t.onMissingParent == nil {
				// stop processing the stack with an error
				return fmt.Errorf("%w: message %s", common.ErrMessageNotFound, currentMessageID.Hex())
			}

			// stop processing the stack if the caller returns an error
			return t.onMissingParent(currentMessageID)
		}
		t.cachedMessageMetas[currentMessageIDMapKey] = cachedMetadata
	}

	traverse, checkedBefore := t.checked[currentMessageIDMapKey]
	if !checkedBefore {
		var err error

		// check condition to decide if msg should be consumed and traversed
		traverse, err = t.condition(cachedMetadata.Retain()) // meta + 1
		if err != nil {
			// there was an error, stop processing the stack
			return err
		}

		// mark the message as checked and remember the result of the traverse condition
		t.checked[currentMessageIDMapKey] = traverse
	}

	if !traverse {
		// remove the message from the stack, parent1 and parent2 are not traversed
		// parent will not get consumed
		t.processed[currentMessageIDMapKey] = struct{}{}
		delete(t.checked, currentMessageIDMapKey)
		t.stack.Remove(ele)
		return nil
	}

	parent1MessageID := cachedMetadata.GetMetadata().GetParent1MessageID()
	parent2MessageID := cachedMetadata.GetMetadata().GetParent2MessageID()

	parentMessageIDs := hornet.MessageIDs{parent1MessageID}
	if *parent1MessageID != *parent2MessageID {
		parentMessageIDs = append(parentMessageIDs, parent2MessageID)
	}

	for _, parentMessageID := range parentMessageIDs {
		if _, parentProcessed := t.processed[parentMessageID.MapKey()]; !parentProcessed {
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
		if err := t.consumer(cachedMetadata.Retain()); err != nil { // meta +1
			// there was an error, stop processing the stack
			return err
		}
	}

	return nil
}
