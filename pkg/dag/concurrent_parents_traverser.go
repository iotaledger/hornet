package dag

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"

	"go.uber.org/atomic"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/utils"
)

var (
	// ErrTraversalDone is returned when the traversal was already marked as done by another thread.
	ErrTraversalDone = errors.New("traversal is done")
)

// ConcurrentParentsTraverser can be used to walk the dag in a multihreaded
// but unsorted way in direction of the parents (past cone).
type ConcurrentParentsTraverser struct {
	// interface to the used storage.
	parentsTraverserStorage ParentsTraverserStorage

	// processed map with already processed messages.
	processed *sync.Map

	// used to count the remaining elements in the stack.
	stackCounter *atomic.Uint64

	// used to fill the pipeline with elements to traverse.
	stackChanIn chan<- (hornet.MessageID)

	// used to get the next element from the pipeline to traverse.
	stackChanOut <-chan (hornet.MessageID)

	ctx                      context.Context
	parallelism              int
	condition                Predicate
	consumer                 Consumer
	onMissingParent          OnMissingParent
	onSolidEntryPoint        OnSolidEntryPoint
	traverseSolidEntryPoints bool

	traverserLock sync.Mutex
}

// NewConcurrentParentsTraverser creates a new traverser that can be used to walk the
// dag in a multihreaded but unsorted way in direction of the parents (past cone).
func NewConcurrentParentsTraverser(parentsTraverserStorage ParentsTraverserStorage, parallelism ...int) *ConcurrentParentsTraverser {

	walkerParallelism := runtime.NumCPU()
	if len(parallelism) > 0 {
		walkerParallelism = parallelism[0]
	}

	t := &ConcurrentParentsTraverser{
		parentsTraverserStorage: parentsTraverserStorage,
		parallelism:             walkerParallelism,
	}

	return t
}

// reset the traverser for the next walk.
func (t *ConcurrentParentsTraverser) reset() {

	// create an unbuffered channel because we don't know the size of the cone to walk upfront
	unbufferedChannel := func() (chan<- hornet.MessageID, <-chan hornet.MessageID) {

		inbound := make(chan hornet.MessageID)
		outbound := make(chan hornet.MessageID)

		go func() {
			var inboundQueue hornet.MessageIDs

			nextValue := func() hornet.MessageID {
				// in case the inbound queue is empty, we return nil to block the nil channel
				// produced by "outboundChannel" until the next element flows in or the
				// inbound channel is closed.
				if len(inboundQueue) == 0 {
					return nil
				}
				return inboundQueue[0]
			}

			outboundChannel := func(nextItem bool) chan hornet.MessageID {
				// in case the inbound queue is empty, we return a nil channel to block
				// until the next element flows in or the inbound channel is closed.
				if len(inboundQueue) == 0 {
					return nil
				}
				return outbound
			}

			var out chan hornet.MessageID = nil

		inboundLoop:
			for {
				select {
				case item, ok := <-inbound:
					if !ok {
						// inbound channel was closed
						break inboundLoop
					}
					inboundQueue = append(inboundQueue, item)
					out = outboundChannel(true)
				case out <- nextValue():
					inboundQueue = inboundQueue[1:]
					out = outboundChannel(false)
				}
			}
			close(outbound)
		}()

		return inbound, outbound
	}

	t.stackCounter = atomic.NewUint64(0)
	t.stackChanIn, t.stackChanOut = unbufferedChannel()
	t.processed = &sync.Map{}
}

// traverseMessage adds the messageID to the pipeline and increases the counter of remaining elements.
func (t *ConcurrentParentsTraverser) traverseMessage(messageID hornet.MessageID) {
	t.stackCounter.Inc()
	t.stackChanIn <- messageID
}

// Traverse starts to traverse the parents (past cone) in a multihreaded but
// unsorted way in direction of the parents.
// the traversal stops due to no more messages passing the given condition.
// Caution: not in DFS order
func (t *ConcurrentParentsTraverser) Traverse(ctx context.Context, parents hornet.MessageIDs, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool) error {

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
		t.traverseMessage(parent)
	}

	doneChan := make(chan struct{})
	errChan := make(chan error)

	for i := 0; i < t.parallelism; i++ {
		go t.processStack(doneChan, errChan)
	}

	select {
	case <-doneChan:
		// traverser finished successfully
		close(errChan)
		close(t.stackChanIn)
		return nil

	case err := <-errChan:
		// traverser encountered an error
		close(doneChan)
		close(errChan)
		close(t.stackChanIn)
		return err
	}
}

// processStack processes elements from the pipeline until there are no elements left or an error occurs.
func (t *ConcurrentParentsTraverser) processStack(doneChan chan struct{}, errChan chan error) {

	wasProcessed := func(messageID hornet.MessageID) bool {

		_, wasProcessed := t.processed.Load(messageID.ToMapKey())
		return wasProcessed
	}

	markAsProcessed := func(messageID hornet.MessageID) bool {

		_, wasProcessed := t.processed.LoadOrStore(messageID.ToMapKey(), struct{}{})
		return wasProcessed
	}

	// processStackParents checks if the current element in the stack must be processed or traversed.
	// the logic in this walker is quite different.
	// we do not walk in any order, we just process every
	// single message and traverse their parents afterwards.
	processStackParents := func(currentMessageID hornet.MessageID) error {
		if err := utils.ReturnErrIfCtxDone(t.ctx, common.ErrOperationAborted); err != nil {
			return err
		}

		select {
		case <-doneChan:
			return ErrTraversalDone
		default:
		}

		if markAsProcessed(currentMessageID) {
			// message was already processed
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
				// the parents are not traversed
				return nil
			}
		}

		cachedMsgMeta, err := t.parentsTraverserStorage.CachedMessageMetadata(currentMessageID) // meta +1
		if err != nil {
			return err
		}

		if cachedMsgMeta == nil {
			// message does not exist, the parents are not traversed

			if t.onMissingParent == nil {
				// stop processing the stack with an error
				return fmt.Errorf("%w: message %s", common.ErrMessageNotFound, currentMessageID.ToHex())
			}

			// stop processing the stack if the caller returns an error
			if err := t.onMissingParent(currentMessageID); err != nil {
				return err
			}

			return nil
		}
		defer cachedMsgMeta.Release(true) // meta -1

		// check condition to decide if msg should be consumed and traversed
		traverse, err := t.condition(cachedMsgMeta.Retain()) // meta pass +1
		if err != nil {
			// there was an error, stop processing the stack
			return err
		}

		if !traverse {
			// the parents are not traversed
			return nil
		}

		if t.consumer != nil {
			// consume the message
			if err := t.consumer(cachedMsgMeta.Retain()); err != nil { // meta pass +1
				// there was an error, stop processing the stack
				return err
			}
		}

		for _, parentMessageID := range cachedMsgMeta.Metadata().Parents() {
			if !wasProcessed(parentMessageID) {
				// parent was not processed yet, add it to the pipeline
				t.traverseMessage(parentMessageID)
			}
		}

		return nil
	}

	// check if all elements from the pipeline got processed
	traversalFinishedSignal := func() bool {
		return t.stackCounter.Dec() == 0
	}

	for {
		select {
		case <-doneChan:
			return

		case messageID := <-t.stackChanOut:

			if err := processStackParents(messageID); err != nil {
				if errors.Is(err, ErrTraversalDone) {
					return
				}

				errChan <- err
				return
			}

			if traversalFinishedSignal() {
				close(doneChan)
				return
			}
		}
	}
}
