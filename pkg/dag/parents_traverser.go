package dag

import (
	"bytes"
	"container/list"
	"fmt"
	"sync"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

type ParentTraverser struct {
	cachedMessageMetas map[string]*tangle.CachedMetadata
	cachedMessages     map[string]*tangle.CachedMessage

	// stack holding the ordered tx to process
	stack *list.List

	// processed map with already processed transactions
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
func NewParentTraverser(condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, abortSignal <-chan struct{}) *ParentTraverser {

	return &ParentTraverser{
		condition:         condition,
		consumer:          consumer,
		onMissingParent:   onMissingParent,
		onSolidEntryPoint: onSolidEntryPoint,
		abortSignal:       abortSignal,
	}
}

func (t *ParentTraverser) cleanup(forceRelease bool) {

	// release all bundles at the end
	for _, cachedMsg := range t.cachedMessages {
		cachedMsg.Release(forceRelease) // bundle -1
	}

	// release all tx metadata at the end
	for _, cachedMetadata := range t.cachedMessageMetas {
		cachedMetadata.Release(forceRelease) // meta -1
	}

	// Release lock after cleanup so the traverser can be reused
	t.traverserLock.Unlock()
}

func (t *ParentTraverser) reset() {

	t.cachedMessageMetas = make(map[string]*tangle.CachedMetadata)
	t.cachedMessages = make(map[string]*tangle.CachedMessage)
	t.processed = make(map[string]struct{})
	t.checked = make(map[string]bool)
	t.stack = list.New()
}

// Traverse starts to traverse the parents (past cone) of the given start message until
// the traversal stops due to no more messages passing the given condition.
// It is a DFS with trunk / branch.
// Caution: condition func is not in DFS order
func (t *ParentTraverser) Traverse(startMessageID hornet.Hash, traverseSolidEntryPoints bool) error {

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
// the traversal stops due to no more transactions passing the given condition.
// Afterwards it traverses the parents (past cone) of the given parent2.
// It is a DFS with parent1 / parent2.
// Caution: condition func is not in DFS order
func (t *ParentTraverser) TraverseParent1AndParent2(parent1MessageID hornet.Hash, parent2MessageID hornet.Hash, traverseSolidEntryPoints bool) error {

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

	// since we first feed the stack the trunk,
	// we need to make sure that we also examine the branch path.
	// however, we only need to do it if the branch wasn't processed yet.
	// the referenced branch transaction could for example already be processed
	// if it is directly/indirectly approved by the trunk.
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
		return tangle.ErrOperationAborted
	default:
	}

	// load candidate tx
	ele := t.stack.Front()
	currentMessageID := ele.Value.(hornet.Hash)

	if _, wasProcessed := t.processed[string(currentMessageID)]; wasProcessed {
		// transaction was already processed
		// remove the transaction from the stack
		t.stack.Remove(ele)
		return nil
	}

	// check if the transaction is a solid entry point
	if tangle.SolidEntryPointsContain(currentMessageID) {
		if t.onSolidEntryPoint != nil {
			t.onSolidEntryPoint(currentMessageID)
		}

		if !t.traverseSolidEntryPoints {
			// remove the transaction from the stack, trunk and branch are not traversed
			t.processed[string(currentMessageID)] = struct{}{}
			delete(t.checked, string(currentMessageID))
			t.stack.Remove(ele)
			return nil
		}
	}

	cachedMetadata, exists := t.cachedMessageMetas[string(currentMessageID)]
	if !exists {
		cachedMetadata = tangle.GetCachedMessageMetadataOrNil(currentMessageID) // meta +1
		if cachedMetadata == nil {
			// remove the transaction from the stack, trunk and branch are not traversed
			t.processed[string(currentMessageID)] = struct{}{}
			delete(t.checked, string(currentMessageID))
			t.stack.Remove(ele)

			if t.onMissingParent == nil {
				// stop processing the stack with an error
				return fmt.Errorf("%w: message %s", tangle.ErrMessageNotFound, currentMessageID.Hex())
			}

			// stop processing the stack if the caller returns an error
			return t.onMissingParent(currentMessageID)
		}
		t.cachedMessageMetas[string(currentMessageID)] = cachedMetadata
	}

	traverse, checkedBefore := t.checked[string(currentMessageID)]
	if !checkedBefore {
		var err error

		// check condition to decide if tx should be consumed and traversed
		traverse, err = t.condition(cachedMetadata.Retain()) // meta + 1
		if err != nil {
			// there was an error, stop processing the stack
			return err
		}

		// mark the transaction as checked and remember the result of the traverse condition
		t.checked[string(currentMessageID)] = traverse
	}

	if !traverse {
		// remove the message from the stack, parent1 and parent2 are not traversed
		// parent will not get consumed
		t.processed[string(currentMessageID)] = struct{}{}
		delete(t.checked, string(currentMessageID))
		t.stack.Remove(ele)
		return nil
	}

	parent1MessageID := cachedMetadata.GetMetadata().GetParent1MessageID()
	parent2MessageID := cachedMetadata.GetMetadata().GetParent2MessageID()

	parentMessageIDs := hornet.Hashes{parent1MessageID}
	if !bytes.Equal(parent1MessageID, parent2MessageID) {
		parentMessageIDs = append(parentMessageIDs, parent2MessageID)
	}

	for _, parentMessageID := range parentMessageIDs {
		if _, parentProcessed := t.processed[string(parentMessageID)]; !parentProcessed {
			// parent was not processed yet
			// traverse this message
			t.stack.PushFront(parentMessageID)
			return nil
		}
	}

	// remove the message from the stack
	t.processed[string(currentMessageID)] = struct{}{}
	delete(t.checked, string(currentMessageID))
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
