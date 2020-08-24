package dag

import (
	"bytes"
	"container/list"
	"fmt"
	"sync"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

type ApproveesTraverser struct {
	cachedTxMetas map[string]*tangle.CachedMetadata
	cachedBundles map[string]*tangle.CachedBundle

	// stack holding the ordered tx to process
	stack *list.List

	// processed map with already processed transactions
	processed map[string]struct{}

	// checked map with result of traverse condition
	checked map[string]bool

	condition         Predicate
	consumer          Consumer
	onMissingApprovee OnMissingApprovee
	onSolidEntryPoint OnSolidEntryPoint
	abortSignal       <-chan struct{}

	traverseSolidEntryPoints bool
	traverseTailsOnly        bool

	traverserLock sync.Mutex
}

// NewApproveesTraverser create a new traverser to traverse the approvees (past cone)
func NewApproveesTraverser(condition Predicate, consumer Consumer, onMissingApprovee OnMissingApprovee, onSolidEntryPoint OnSolidEntryPoint, abortSignal <-chan struct{}) *ApproveesTraverser {

	return &ApproveesTraverser{
		condition:         condition,
		consumer:          consumer,
		onMissingApprovee: onMissingApprovee,
		onSolidEntryPoint: onSolidEntryPoint,
		abortSignal:       abortSignal,
	}
}

func (t *ApproveesTraverser) cleanup(forceRelease bool) {

	// release all bundles at the end
	for _, cachedBundle := range t.cachedBundles {
		cachedBundle.Release(forceRelease) // bundle -1
	}

	// release all tx metadata at the end
	for _, cachedTxMeta := range t.cachedTxMetas {
		cachedTxMeta.Release(forceRelease) // meta -1
	}

	// Release lock after cleanup so the traverser can be reused
	t.traverserLock.Unlock()
}

func (t *ApproveesTraverser) reset() {

	t.cachedTxMetas = make(map[string]*tangle.CachedMetadata)
	t.cachedBundles = make(map[string]*tangle.CachedBundle)
	t.processed = make(map[string]struct{})
	t.checked = make(map[string]bool)
	t.stack = list.New()
}

// Traverse starts to traverse the approvees (past cone) of the given start transaction until
// the traversal stops due to no more transactions passing the given condition.
// It is a DFS with trunk / branch.
// Caution: condition func is not in DFS order
func (t *ApproveesTraverser) Traverse(startTxHash hornet.Hash, traverseSolidEntryPoints bool, traverseTailsOnly bool) error {

	// make sure only one traversal is running
	t.traverserLock.Lock()

	// Prepare for a new traversal
	t.reset()

	t.traverseSolidEntryPoints = traverseSolidEntryPoints
	t.traverseTailsOnly = traverseTailsOnly

	defer t.cleanup(true)

	t.stack.PushFront(startTxHash)
	for t.stack.Len() > 0 {
		if err := t.processStackApprovees(); err != nil {
			return err
		}
	}

	return nil
}

// TraverseTrunkAndBranch starts to traverse the approvees (past cone) of the given trunk transaction until
// the traversal stops due to no more transactions passing the given condition.
// Afterwards it traverses the approvees (past cone) of the given branch transaction.
// It is a DFS with trunk / branch.
// Caution: condition func is not in DFS order
func (t *ApproveesTraverser) TraverseTrunkAndBranch(trunkTxHash hornet.Hash, branchTxHash hornet.Hash, traverseSolidEntryPoints bool, traverseTailsOnly bool) error {

	// make sure only one traversal is running
	t.traverserLock.Lock()

	// Prepare for a new traversal
	t.reset()

	t.traverseSolidEntryPoints = traverseSolidEntryPoints
	t.traverseTailsOnly = traverseTailsOnly

	defer t.cleanup(true)

	t.stack.PushFront(trunkTxHash)
	for t.stack.Len() > 0 {
		if err := t.processStackApprovees(); err != nil {
			return err
		}
	}

	// since we first feed the stack the trunk,
	// we need to make sure that we also examine the branch path.
	// however, we only need to do it if the branch wasn't processed yet.
	// the referenced branch transaction could for example already be processed
	// if it is directly/indirectly approved by the trunk.
	t.stack.PushFront(branchTxHash)
	for t.stack.Len() > 0 {
		if err := t.processStackApprovees(); err != nil {
			return err
		}
	}

	return nil
}

// processStackApprovees checks if the current element in the stack must be processed or traversed.
// first the trunk is traversed, then the branch.
func (t *ApproveesTraverser) processStackApprovees() error {

	select {
	case <-t.abortSignal:
		return tangle.ErrOperationAborted
	default:
	}

	// load candidate tx
	ele := t.stack.Front()
	currentTxHash := ele.Value.(hornet.Hash)

	if _, wasProcessed := t.processed[string(currentTxHash)]; wasProcessed {
		// transaction was already processed
		// remove the transaction from the stack
		t.stack.Remove(ele)
		return nil
	}

	// check if the transaction is a solid entry point
	if tangle.SolidEntryPointsContain(currentTxHash) {
		if t.onSolidEntryPoint != nil {
			t.onSolidEntryPoint(currentTxHash)
		}

		if !t.traverseSolidEntryPoints {
			// remove the transaction from the stack, trunk and branch are not traversed
			t.processed[string(currentTxHash)] = struct{}{}
			delete(t.checked, string(currentTxHash))
			t.stack.Remove(ele)
			return nil
		}
	}

	cachedTxMeta, exists := t.cachedTxMetas[string(currentTxHash)]
	if !exists {
		cachedTxMeta = tangle.GetCachedTxMetadataOrNil(currentTxHash) // meta +1
		if cachedTxMeta == nil {
			// remove the transaction from the stack, trunk and branch are not traversed
			t.processed[string(currentTxHash)] = struct{}{}
			delete(t.checked, string(currentTxHash))
			t.stack.Remove(ele)

			if t.onMissingApprovee == nil {
				// stop processing the stack with an error
				return fmt.Errorf("%w: transaction %s", tangle.ErrTransactionNotFound, currentTxHash.Trytes())
			}

			// stop processing the stack if the caller returns an error
			return t.onMissingApprovee(currentTxHash)
		}
		t.cachedTxMetas[string(currentTxHash)] = cachedTxMeta
	}

	traverse, checkedBefore := t.checked[string(currentTxHash)]
	if !checkedBefore {
		var err error

		// check condition to decide if tx should be consumed and traversed
		traverse, err = t.condition(cachedTxMeta.Retain()) // meta + 1
		if err != nil {
			// there was an error, stop processing the stack
			return err
		}

		// mark the transaction as checked and remember the result of the traverse condition
		t.checked[string(currentTxHash)] = traverse
	}

	if !traverse {
		// remove the transaction from the stack, trunk and branch are not traversed
		// transaction will not get consumed
		t.processed[string(currentTxHash)] = struct{}{}
		delete(t.checked, string(currentTxHash))
		t.stack.Remove(ele)
		return nil
	}

	var trunkHash, branchHash hornet.Hash

	if !t.traverseTailsOnly {
		trunkHash = cachedTxMeta.GetMetadata().GetTrunkHash()
		branchHash = cachedTxMeta.GetMetadata().GetBranchHash()
	} else {
		// load up bundle to retrieve trunk and branch of the head tx
		cachedBundle, exists := t.cachedBundles[string(currentTxHash)]
		if !exists {
			cachedBundle = tangle.GetCachedBundleOrNil(currentTxHash) // bundle +1
			if cachedBundle == nil {
				return fmt.Errorf("%w: bundle %s of candidate tx %s doesn't exist", tangle.ErrBundleNotFound, cachedTxMeta.GetMetadata().GetBundleHash().Trytes(), currentTxHash.Trytes())
			}
			t.cachedBundles[string(currentTxHash)] = cachedBundle
		}

		trunkHash = cachedBundle.GetBundle().GetTrunkHash(true)
		branchHash = cachedBundle.GetBundle().GetBranchHash(true)
	}

	approveeHashes := hornet.Hashes{trunkHash}
	if !bytes.Equal(trunkHash, branchHash) {
		approveeHashes = append(approveeHashes, branchHash)
	}

	for _, approveeHash := range approveeHashes {
		if _, approveeProcessed := t.processed[string(approveeHash)]; !approveeProcessed {
			// approvee was not processed yet
			// traverse this transaction
			t.stack.PushFront(approveeHash)
			return nil
		}
	}

	// remove the transaction from the stack
	t.processed[string(currentTxHash)] = struct{}{}
	delete(t.checked, string(currentTxHash))
	t.stack.Remove(ele)

	if t.consumer != nil {
		// consume the transaction
		if err := t.consumer(cachedTxMeta.Retain()); err != nil { // meta +1
			// there was an error, stop processing the stack
			return err
		}
	}

	return nil
}
