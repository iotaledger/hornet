package consensus

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

// WhiteFlagConfirmation represents a confirmation done via milestone under the "white-flag" rules.
type WhiteFlagConfirmation struct {
	// The tails of bundles which mutate the ledger in the order in which they were applied.
	Tails []trinary.Hash
	// Contains the updated state of the addresses which were mutated by the given confirmation.
	NewAddressState map[trinary.Hash]int64
}

// WhiteFlagConfirm computes the ledger changes in accordance to the white-flag rules for the given milestone bundle.
// Via a post-order depth-first search the approved bundles of the given milestone are traversed and
// in their corresponding order applied/mutated against the previous ledger state, respectively previous applied mutations.
// Bundles within the approving cone must obey to strict schematics and be valid. Bundles causing conflicts are
// ignored but do not create an error.
func WhiteFlagConfirm(cachedMsBundle *tangle.CachedBundle) (*WhiteFlagConfirmation, error) {
	defer cachedMsBundle.Release()
	msBundle := cachedMsBundle.GetBundle()

	stack := list.New()
	visited := map[trinary.Hash]struct{}{}
	cachedMsHeadTx := msBundle.GetHead()
	msHeadTxHash := cachedMsHeadTx.GetTransaction().GetHash()
	cachedMsHeadTx.Release()
	stack.PushFront(msHeadTxHash)

	milestoneIndex := msBundle.GetMilestoneIndex()
	wfConfirmation := &WhiteFlagConfirmation{
		Tails:           make([]trinary.Hash, 0),
		NewAddressState: make(map[trinary.Hash]int64),
	}

	for stack.Len() > 0 {
		if err := examine(stack, wfConfirmation, visited, milestoneIndex); err != nil {
			return nil, err
		}
	}

	return wfConfirmation, nil
}

func examine(stack *list.List, wfConf *WhiteFlagConfirmation, visited map[trinary.Hash]struct{}, milestoneIndex milestone.Index) error {
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

	// load up bundle to retrieve trunk and branch of head tx
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
	var trunkConfirmedIndex, branchConfirmedIndex milestone.Index

	if _, trunkVisited = visited[headTxTrunkHash]; !trunkVisited {
		if cachedTrunkTx = tangle.GetCachedTransactionOrNil(headTxTrunkHash); cachedTrunkTx == nil {
			return fmt.Errorf("%w: transaction %s", ErrMissingTransaction, headTxTrunkHash)
		}
		defer cachedTrunkTx.Release()
		trunkConfirmed, trunkConfirmedIndex = cachedTrunkTx.GetMetadata().GetConfirmed()
	}

	if headTxTrunkHash == headTxBranchHash {
		branchVisited = trunkVisited
		branchConfirmedIndex = trunkConfirmedIndex
		branchConfirmed = trunkConfirmed
		cachedBranchTx = cachedTrunkTx
		// no need to load branch tx
	} else {
		if _, branchVisited = visited[headTxBranchHash]; !branchVisited {
			if cachedBranchTx = tangle.GetCachedTransactionOrNil(headTxBranchHash); cachedBranchTx == nil {
				return fmt.Errorf("%w: transaction %s", ErrMissingTransaction, headTxBranchHash)
			}
			defer cachedBranchTx.Release()
			branchConfirmed, branchConfirmedIndex = cachedBranchTx.GetMetadata().GetConfirmed()
		}
	}

	// verify that head and trunk txs are indeed tails
	if !trunkVisited && !cachedTrunkTx.GetTransaction().IsTail() {
		return fmt.Errorf("%w: trunk tx %s of bundle head tx %s is not a tail", ErrMilestoneApprovedInvalidBundle, headTxTrunkHash, bundleHeadTx.GetHash())
	}

	if !branchVisited && !cachedBranchTx.GetTransaction().IsTail() {
		return fmt.Errorf("%w: branch tx %s of bundle head tx %s is not a tail", ErrMilestoneApprovedInvalidBundle, headTxBranchHash, bundleHeadTx.GetHash())
	}

	// here we reached a tail of which its past cone was already visited, therefore we include its bundle
	if (trunkVisited || tangle.SolidEntryPointsContain(headTxTrunkHash) || trunkConfirmedIndex != milestoneIndex) ||
		(branchVisited || tangle.SolidEntryPointsContain(headTxBranchHash) || branchConfirmedIndex != milestoneIndex) {
		// if the bundle is invalid or a value spam bundle, we don't incorporate it as part of the mutations
		bundle := cachedBundle.GetBundle()

		// on a bundle not mutating any address, we simply mark it as visited
		if bundle.IsValueSpam() {
			visited[currentTx.GetHash()] = struct{}{}
			stack.Remove(ele)
			return nil
		}

		var conflicting bool

		// contains the updated mutations from this bundle against the
		// current mutations of the milestone's confirming cone (or previous ledger state).
		// we only apply it to the milestone's confirming cone mutations if
		// the bundle doesn't create any conflict.
		patchedState := make(map[trinary.Hash]int64)

		for addr, change := range bundle.GetLedgerChanges() {

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

		// incorporate the mutations in accordance with the previous mutations
		// in the milestone's confirming cone/previous ledger state.
		if !conflicting {
			// mark the given tail to be part of milestone ledger changing tail inclusion set
			wfConf.Tails = append(wfConf.Tails, currentTx.GetHash())
			for addr, balance := range patchedState {
				wfConf.NewAddressState[addr] = balance
			}
		}

		visited[currentTx.GetHash()] = struct{}{}
		stack.Remove(ele)
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
