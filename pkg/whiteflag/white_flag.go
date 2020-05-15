package whiteflag

import (
	"container/list"
	"errors"
	"fmt"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/math"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	// ErrMilestoneApprovedInvalidBundle is returned when a milestone approves an invalid bundle in its past cone.
	ErrMilestoneApprovedInvalidBundle = errors.New("the milestone approved an invalid bundle")
	// ErrMissingTransaction is returned when a transaction is missing in the past cone of a milestone.
	ErrMissingTransaction = errors.New("missing transaction")
	// ErrMissingBundle is returned when a bundle is missing in the past cone of a milestone even though a transaction
	// of it exists.
	ErrMissingBundle = errors.New("missing bundle")
)

// Confirmation represents a confirmation done via a milestone under the "white-flag" approach.
type Confirmation struct {
	// The tails of bundles which mutate the ledger in the order in which they were applied.
	TailsIncluded []trinary.Hash
	// The tails of bundles which were excluded as they were conflicting with the mutations.
	TailsExcludedConflicting []trinary.Hash
	// The tails which were excluded because they were part of a zero or spam value transfer.
	TailsExcludedZeroValue []trinary.Hash
	// Contains the updated state of the addresses which were mutated by the given confirmation.
	NewAddressState map[trinary.Hash]int64
	// The merkle tree root hash of all tails.
	MerkleTreeHash []byte
}

// ComputeConfirmation computes the ledger changes in accordance to the white-flag rules for the given milestone bundle.
// Via a post-order depth-first search the approved bundles of the given milestone are traversed and
// in their corresponding order applied/mutated against the previous ledger state, respectively previous applied mutations.
// Bundles within the approving cone must obey to strict schematics and be valid. Bundles causing conflicts are
// ignored but do not create an error.
// The ledger state must be write locked while this function is getting called in order to ensure consistency.
func ComputeConfirmation(cachedMsBundle *tangle.CachedBundle) (*Confirmation, error) {
	defer cachedMsBundle.Release()
	msBundle := cachedMsBundle.GetBundle()

	stack := list.New()
	visited := map[trinary.Hash]struct{}{}
	cachedMsTailTx := msBundle.GetTail()
	msTailTxHash := cachedMsTailTx.GetTransaction().GetHash()
	cachedMsTailTx.Release()
	stack.PushFront(msTailTxHash)

	milestoneIndex := msBundle.GetMilestoneIndex()
	wfConfirmation := &Confirmation{
		TailsIncluded:            make([]trinary.Hash, 0),
		TailsExcludedConflicting: make([]trinary.Hash, 0),
		TailsExcludedZeroValue:   make([]trinary.Hash, 0),
		NewAddressState:          make(map[trinary.Hash]int64),
	}

	for stack.Len() > 0 {
		if err := ProcessStack(stack, wfConfirmation, visited, milestoneIndex); err != nil {
			return nil, err
		}
	}

	// compute merkle tree root hash
	wfConfirmation.MerkleTreeHash = DefaultHasher.TreeHash(wfConfirmation.TailsIncluded)
	return wfConfirmation, nil
}

// ComputeMerkleTreeRootHash computes the merkle tree root hash consisting out of the tail transaction hashes
// of the bundles which are part of the set which mutated the ledger state when applying the white-flag approach.
// The ledger state must be write locked while this function is getting called in order to ensure consistency.
func ComputeMerkleTreeRootHash(trunkHash trinary.Hash, branchHash trinary.Hash, newMilestoneIndex milestone.Index) ([]byte, error) {
	stack := list.New()
	stack.PushFront(trunkHash)
	visited := make(map[trinary.Hash]struct{})
	wfConfirmation := &Confirmation{
		TailsIncluded:   make([]trinary.Hash, 0),
		NewAddressState: make(map[trinary.Hash]int64),
	}
	for stack.Len() > 0 {
		if err := ProcessStack(stack, wfConfirmation, visited, newMilestoneIndex); err != nil {
			return nil, err
		}
		// since we first feed the stack the trunk,
		// we need to make sure that we also examine the branch path.
		// however, we only need to do it if the branch wasn't visited yet.
		// the referenced branch transaction could for example already be visited
		// if it is directly/indirectly approved by the trunk.
		_, branchVisited := visited[branchHash]
		if stack.Len() == 0 && !branchVisited {
			stack.PushFront(branchHash)
		}
	}

	return DefaultHasher.TreeHash(wfConfirmation.TailsIncluded), nil
}

// ProcessStack retrieves the first element from the given stack, loads its bundle and then the trunk and
// branch transaction of the bundle head. If trunk and branch are both SEPs, already visited or already confirmed,
// then the mutations from the transaction retrieved from the stack are accumulated to the given Confirmation struct's mutations.
// This function must be called repeatedly to compute the mutations a white-flag confirmation would create.
// If the popped transaction was used to mutate the Confirmation struct, it will also be appended to Confirmation.TailsIncluded
// and it will be removed from the stack. If the head trunk doesn't meet any of the mentioned criteria, it is pushed onto the
// stack to be the next transaction to be examined on the subsequent ProcessStack() call (same with the branch
// but only if the trunk wasn't pushed onto the stack). The ledger state must be write locked while this function
// is getting called in order to ensure consistency.
func ProcessStack(stack *list.List, wfConf *Confirmation, visited map[trinary.Hash]struct{}, milestoneIndex milestone.Index) error {
	// load candidate tail tx
	ele := stack.Front()
	currentTxHash := ele.Value.(trinary.Hash)
	cachedTx := tangle.GetCachedTransactionOrNil(currentTxHash)
	if cachedTx == nil {
		return fmt.Errorf("%w: candidate tx %s doesn't exist", ErrMissingTransaction, currentTxHash)
	}
	defer cachedTx.Release()
	currentTx := cachedTx.GetTransaction()

	if !currentTx.IsTail() {
		return fmt.Errorf("%w: candidate tx %s is not a tail of a bundle", ErrMilestoneApprovedInvalidBundle, currentTx.GetHash())
	}

	// load up bundle to retrieve trunk and branch of the head tx
	cachedBundle := tangle.GetCachedBundleOrNil(currentTx.GetHash())
	if cachedBundle == nil {
		return fmt.Errorf("%w: bundle %s of candidate tx %s doesn't exist", ErrMissingBundle, currentTx.Tx.Bundle, currentTx.GetHash())
	}
	defer cachedBundle.Release()

	if !cachedBundle.GetBundle().IsValid() || !cachedBundle.GetBundle().ValidStrictSemantics() {
		return fmt.Errorf("%w: bundle %s is invalid", ErrMilestoneApprovedInvalidBundle, currentTx.Tx.Bundle)
	}

	cachedBundleHeadTx := cachedBundle.GetBundle().GetHead()
	defer cachedBundleHeadTx.Release()
	bundleHeadTx := cachedBundleHeadTx.GetTransaction()
	headTxTrunkHash := bundleHeadTx.GetTrunk()
	headTxBranchHash := bundleHeadTx.GetBranch()

	var cachedTrunkTx, cachedBranchTx *tangle.CachedTransaction
	var trunkVisited, trunkConfirmed, branchVisited, branchConfirmed bool

	if _, trunkVisited = visited[headTxTrunkHash]; !trunkVisited {
		if cachedTrunkTx = tangle.GetCachedTransactionOrNil(headTxTrunkHash); cachedTrunkTx == nil {
			return fmt.Errorf("%w: transaction %s", ErrMissingTransaction, headTxTrunkHash)
		}
		defer cachedTrunkTx.Release()
		trunkConfirmed, _ = cachedTrunkTx.GetMetadata().GetConfirmed()

		// auto. set branch trunk to branch data,
		// gets overwritten in case trunk != branch
		branchVisited = trunkVisited
		branchConfirmed = trunkConfirmed
		cachedBranchTx = cachedTrunkTx
	}

	if headTxTrunkHash != headTxBranchHash {
		if _, branchVisited = visited[headTxBranchHash]; !branchVisited {
			if cachedBranchTx = tangle.GetCachedTransactionOrNil(headTxBranchHash); cachedBranchTx == nil {
				return fmt.Errorf("%w: transaction %s", ErrMissingTransaction, headTxBranchHash)
			}
			defer cachedBranchTx.Release()
			branchConfirmed, _ = cachedBranchTx.GetMetadata().GetConfirmed()
		}
	}

	// verify that head and trunk txs are indeed tails
	if !trunkVisited && !cachedTrunkTx.GetTransaction().IsTail() {
		return fmt.Errorf("%w: trunk tx %s of bundle head tx %s is not a tail", ErrMilestoneApprovedInvalidBundle, headTxTrunkHash, bundleHeadTx.GetHash())
	}

	if !branchVisited && !cachedBranchTx.GetTransaction().IsTail() {
		return fmt.Errorf("%w: branch tx %s of bundle head tx %s is not a tail", ErrMilestoneApprovedInvalidBundle, headTxBranchHash, bundleHeadTx.GetHash())
	}

	// here we reached a tail of which its past cone was already visited or confirmed,
	// therefore we now can examine the bundle
	if (trunkVisited || tangle.SolidEntryPointsContain(headTxTrunkHash) || trunkConfirmed) &&
		(branchVisited || tangle.SolidEntryPointsContain(headTxBranchHash) || branchConfirmed) {

		// if the bundle is conflicting or a value spam bundle,
		// we don't incorporate it as part of the mutations
		bundle := cachedBundle.GetBundle()

		visited[currentTx.GetHash()] = struct{}{}
		stack.Remove(ele)

		// exclude zero or spam value bundles
		mutations := bundle.GetLedgerChanges()
		if bundle.IsValueSpam() || len(mutations) == 0 {
			wfConf.TailsExcludedZeroValue = append(wfConf.TailsExcludedZeroValue, currentTx.GetHash())
			return nil
		}

		var conflicting bool

		// contains the updated mutations from this bundle against the
		// current mutations of the milestone's confirming cone (or previous ledger state).
		// we only apply it to the milestone's confirming cone mutations if
		// the bundle doesn't create any conflict.
		patchedState := make(map[trinary.Hash]int64)

		for addr, change := range mutations {

			// load state from milestone cone mutation or previous milestone
			balance, has := wfConf.NewAddressState[addr]
			if !has {
				balanceStateFromPreviousMilestone, _, err := tangle.GetBalanceForAddressWithoutLocking(addr)
				if err != nil {
					return fmt.Errorf("%w: unable to retrieve balance of address %s", err, addr)
				}
				balance = int64(balanceStateFromPreviousMilestone)
			}

			// note that there's no overflow of int64 values here
			// as a valid bundle's transaction can not spend more than the total supply,
			// meaning that newBalance could be max 2*total_supply or min -total_supply.
			newBalance := balance + change

			// on below zero or above total supply the mutation is invalid
			if newBalance < 0 || math.AbsInt64(newBalance) > consts.TotalSupply {
				conflicting = true
				break
			}

			patchedState[addr] = newBalance
		}

		if conflicting {
			wfConf.TailsExcludedConflicting = append(wfConf.TailsExcludedConflicting, currentTx.GetHash())
			return nil
		}

		// mark the given tail to be part of milestone ledger changing tail inclusion set
		wfConf.TailsIncluded = append(wfConf.TailsIncluded, currentTx.GetHash())

		// incorporate the mutations in accordance with the previous mutations
		// in the milestone's confirming cone/previous ledger state.
		for addr, balance := range patchedState {
			wfConf.NewAddressState[addr] = balance
		}

		return nil
	}

	if !tangle.SolidEntryPointsContain(headTxTrunkHash) && !trunkVisited && !trunkConfirmed {
		stack.PushFront(headTxTrunkHash)
		return nil
	}

	if !tangle.SolidEntryPointsContain(headTxBranchHash) && !branchVisited && !branchConfirmed {
		stack.PushFront(headTxBranchHash)
		return nil
	}

	return nil
}
