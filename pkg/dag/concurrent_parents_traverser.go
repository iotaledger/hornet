package dag

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/atomic"

	"github.com/iotaledger/hive.go/core/contextutils"
	"github.com/iotaledger/hornet/v2/pkg/common"
	iotago "github.com/iotaledger/iota.go/v3"
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

	// processed map with already processed blocks.
	processed *sync.Map

	// used to count the remaining elements in the stack.
	stackCounter *atomic.Uint64

	// used to fill the pipeline with elements to traverse.
	stackChanIn chan<- (*iotago.BlockID)

	// used to get the next element from the pipeline to traverse.
	stackChanOut <-chan (*iotago.BlockID)

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
	if len(parallelism) > 0 && parallelism[0] > 0 {
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
	unbufferedChannel := func() (chan<- *iotago.BlockID, <-chan *iotago.BlockID) {

		inbound := make(chan *iotago.BlockID)
		outbound := make(chan *iotago.BlockID)

		go func() {
			var inboundQueue []*iotago.BlockID

			nextValue := func() *iotago.BlockID {
				// in case the inbound queue is empty, we return nil to block the nil channel
				// produced by "outboundChannel" until the next element flows in or the
				// inbound channel is closed.
				if len(inboundQueue) == 0 {
					return nil
				}

				return inboundQueue[0]
			}

			outboundChannel := func() chan *iotago.BlockID {
				// in case the inbound queue is empty, we return a nil channel to block
				// until the next element flows in or the inbound channel is closed.
				if len(inboundQueue) == 0 {
					return nil
				}

				return outbound
			}

			var out chan *iotago.BlockID

		inboundLoop:
			for {
				select {
				case item, ok := <-inbound:
					if !ok {
						// inbound channel was closed
						break inboundLoop
					}
					inboundQueue = append(inboundQueue, item)
					out = outboundChannel()
				case out <- nextValue():
					inboundQueue = inboundQueue[1:]
					out = outboundChannel()
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

// traverseBlock adds the blockID to the pipeline and increases the counter of remaining elements.
func (t *ConcurrentParentsTraverser) traverseBlock(blockID iotago.BlockID) {
	t.stackCounter.Inc()
	t.stackChanIn <- &blockID
}

// Traverse starts to traverse the parents (past cone) in a multihreaded but
// unsorted way in direction of the parents.
// the traversal stops due to no more blocks passing the given condition.
// Caution: not in DFS order.
func (t *ConcurrentParentsTraverser) Traverse(ctx context.Context, parents iotago.BlockIDs, condition Predicate, consumer Consumer, onMissingParent OnMissingParent, onSolidEntryPoint OnSolidEntryPoint, traverseSolidEntryPoints bool) error {

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
		t.traverseBlock(parent)
	}

	doneChan := make(chan struct{})
	errChan := make(chan error, t.parallelism)

	wg := &sync.WaitGroup{}
	wg.Add(t.parallelism)

	defer func() {
		// wait until the traverser is done
		wg.Wait()
	}()

	for i := 0; i < t.parallelism; i++ {
		go t.processStack(wg, doneChan, errChan)
	}

	select {
	case <-doneChan:
		// traverser finished successfully
		close(t.stackChanIn)

		return nil

	case err := <-errChan:
		// traverser encountered an error
		close(doneChan)
		close(t.stackChanIn)

		return err
	}
}

// processStack processes elements from the pipeline until there are no elements left or an error occurs.
func (t *ConcurrentParentsTraverser) processStack(wg *sync.WaitGroup, doneChan chan struct{}, errChan chan error) {
	defer wg.Done()

	wasProcessed := func(blockID iotago.BlockID) bool {
		_, wasProcessed := t.processed.Load(blockID)

		return wasProcessed
	}

	markAsProcessed := func(blockID iotago.BlockID) bool {
		_, wasProcessed := t.processed.LoadOrStore(blockID, struct{}{})

		return wasProcessed
	}

	// processStackParents checks if the current element in the stack must be processed or traversed.
	// the logic in this walker is quite different.
	// we don't walk in any order, we just process every
	// single block and traverse their parents afterwards.
	processStackParents := func(currentBlockID iotago.BlockID) error {
		if err := contextutils.ReturnErrIfCtxDone(t.ctx, common.ErrOperationAborted); err != nil {
			return err
		}

		if err := contextutils.ReturnErrIfChannelClosed(doneChan, ErrTraversalDone); err != nil {
			return err
		}

		if markAsProcessed(currentBlockID) {
			// block was already processed
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
				// the parents are not traversed
				return nil
			}
		}

		cachedBlockMeta, err := t.parentsTraverserStorage.CachedBlockMetadata(currentBlockID) // meta +1
		if err != nil {
			return err
		}

		if cachedBlockMeta == nil {
			// block does not exist, the parents are not traversed

			if t.onMissingParent == nil {
				// stop processing the stack with an error
				return fmt.Errorf("%w: block %s", common.ErrBlockNotFound, currentBlockID.ToHex())
			}

			// stop processing the stack if the caller returns an error
			if err := t.onMissingParent(currentBlockID); err != nil {
				return err
			}

			return nil
		}
		defer cachedBlockMeta.Release(true) // meta -1

		// check condition to decide if block should be consumed and traversed
		traverse, err := t.condition(cachedBlockMeta.Retain()) // meta pass +1
		if err != nil {
			// there was an error, stop processing the stack
			return err
		}

		if !traverse {
			// the parents are not traversed
			return nil
		}

		if t.consumer != nil {
			// consume the block
			if err := t.consumer(cachedBlockMeta.Retain()); err != nil { // meta pass +1
				// there was an error, stop processing the stack
				return err
			}
		}

		for _, parentBlockID := range cachedBlockMeta.Metadata().Parents() {
			if !wasProcessed(parentBlockID) {
				// do not walk further parents if the traversal was already done
				if err := contextutils.ReturnErrIfChannelClosed(doneChan, ErrTraversalDone); err != nil {
					return err
				}

				// parent was not processed yet, add it to the pipeline
				t.traverseBlock(parentBlockID)
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

		case blockID := <-t.stackChanOut:
			if blockID == nil {
				return
			}

			if err := processStackParents(*blockID); err != nil {
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
