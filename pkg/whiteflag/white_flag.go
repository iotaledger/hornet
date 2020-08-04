package whiteflag

import (
	"crypto"
	"errors"
	"fmt"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/math"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

var (
	// ErrMilestoneApprovedInvalidBundle is returned when a milestone approves an invalid bundle in its past cone.
	ErrMilestoneApprovedInvalidBundle = errors.New("the milestone approved an invalid bundle")
)

// Confirmation represents a confirmation done via a milestone under the "white-flag" approach.
type Confirmation struct {
	// The index of the milestone that got confirmed.
	MilestoneIndex milestone.Index
	// The transaction hash of the tail transaction of the milestone that got confirmed.
	MilestoneHash hornet.Hash
	// The ledger mutations and referenced transactions of this milestone.
	Mutations *WhiteFlagMutations
}

// WhiteFlagMutations contains the ledger mutations and referenced transactions applied to a cone under the "white-flag" approach.
type WhiteFlagMutations struct {
	// The tails of bundles which mutate the ledger in the order in which they were applied.
	TailsIncluded hornet.Hashes
	// The tails of bundles which were excluded as they were conflicting with the mutations.
	TailsExcludedConflicting hornet.Hashes
	// The tails which were excluded because they were part of a zero or spam value transfer.
	TailsExcludedZeroValue hornet.Hashes
	// The tails which were referenced by the milestone (should be the sum of TailsIncluded + TailsExcludedConflicting + TailsExcludedZeroValue).
	TailsReferenced hornet.Hashes
	// Contains the updated state of the addresses which were mutated by the given confirmation.
	NewAddressState map[string]int64
	// Contains the mutations to the state of the addresses for the given confirmation.
	AddressMutations map[string]int64
	// The merkle tree root hash of all tails.
	MerkleTreeHash []byte
}

// ComputeConfirmation computes the ledger changes in accordance to the white-flag rules for the cone referenced by trunk and branch.
// Via a post-order depth-first search the approved bundles of the given cone are traversed and
// in their corresponding order applied/mutated against the previous ledger state, respectively previous applied mutations.
// Bundles within the approving cone must obey to strict schematics and be valid. Bundles causing conflicts are
// ignored but do not create an error.
// It also computes the merkle tree root hash consisting out of the tail transaction hashes
// of the bundles which are part of the set which mutated the ledger state when applying the white-flag approach.
// The ledger state must be write locked while this function is getting called in order to ensure consistency.
func ComputeWhiteFlagMutations(merkleTreeHashFunc crypto.Hash, trunkHash hornet.Hash, branchHash ...hornet.Hash) (*WhiteFlagMutations, error) {
	wfConf := &WhiteFlagMutations{
		TailsIncluded:            make(hornet.Hashes, 0),
		TailsExcludedConflicting: make(hornet.Hashes, 0),
		TailsExcludedZeroValue:   make(hornet.Hashes, 0),
		TailsReferenced:          make(hornet.Hashes, 0),
		NewAddressState:          make(map[string]int64),
		AddressMutations:         make(map[string]int64),
	}

	// traversal stops if no more transactions pass the given condition
	// Caution: condition func is not in DFS order
	condition := func(cachedTx *tangle.CachedTransaction) (bool, error) { // tx +1
		defer cachedTx.Release(true) // tx -1

		if !cachedTx.GetTransaction().IsTail() {
			return false, fmt.Errorf("%w: candidate tx %s is not a tail of a bundle", ErrMilestoneApprovedInvalidBundle, cachedTx.GetTransaction().GetTxHash().Trytes())
		}

		// load up bundle
		cachedBundle := tangle.GetCachedBundleOrNil(cachedTx.GetTransaction().GetTxHash())
		if cachedBundle == nil {
			return false, fmt.Errorf("%w: bundle %s of candidate tx %s doesn't exist", tangle.ErrBundleNotFound, cachedTx.GetTransaction().Tx.Bundle, cachedTx.GetTransaction().GetTxHash().Trytes())
		}
		defer cachedBundle.Release(true)

		// check validty and correct strict semantics
		if !cachedBundle.GetBundle().IsValid() || !cachedBundle.GetBundle().ValidStrictSemantics() {
			return false, fmt.Errorf("%w: bundle %s is invalid", ErrMilestoneApprovedInvalidBundle, cachedBundle.GetBundle().GetBundleHash().Trytes())
		}

		// only traverse and process the transaction if it was not confirmed yet
		return !cachedTx.GetMetadata().IsConfirmed(), nil
	}

	// consumer
	consumer := func(cachedTx *tangle.CachedTransaction) error { // tx +1
		defer cachedTx.Release(true) // tx -1

		// load up bundle
		cachedBundle := tangle.GetCachedBundleOrNil(cachedTx.GetTransaction().GetTxHash())
		if cachedBundle == nil {
			return fmt.Errorf("%w: bundle %s of candidate tx %s doesn't exist", tangle.ErrBundleNotFound, cachedTx.GetTransaction().Tx.Bundle, cachedTx.GetTransaction().GetTxHash().Trytes())
		}
		defer cachedBundle.Release(true)

		// exclude zero or spam value bundles
		bundle := cachedBundle.GetBundle()
		mutations := bundle.GetLedgerChanges()
		if bundle.IsValueSpam() || len(mutations) == 0 {
			wfConf.TailsReferenced = append(wfConf.TailsReferenced, cachedTx.GetTransaction().GetTxHash())
			wfConf.TailsExcludedZeroValue = append(wfConf.TailsExcludedZeroValue, cachedTx.GetTransaction().GetTxHash())
			return nil
		}

		var conflicting bool

		// contains the updated mutations from this bundle against the
		// current mutations of the milestone's confirming cone (or previous ledger state).
		// we only apply it to the milestone's confirming cone mutations if
		// the bundle doesn't create any conflict.
		patchedState := make(map[string]int64)
		validMutations := make(map[string]int64)

		for addr, change := range mutations {

			// load state from milestone cone mutation or previous milestone
			balance, has := wfConf.NewAddressState[addr]
			if !has {
				balanceStateFromPreviousMilestone, _, err := tangle.GetBalanceForAddressWithoutLocking(hornet.Hash(addr))
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
			validMutations[addr] = validMutations[addr] + change
		}

		wfConf.TailsReferenced = append(wfConf.TailsReferenced, cachedTx.GetTransaction().GetTxHash())

		if conflicting {
			wfConf.TailsExcludedConflicting = append(wfConf.TailsExcludedConflicting, cachedTx.GetTransaction().GetTxHash())
			return nil
		}

		// mark the given tail to be part of milestone ledger changing tail inclusion set
		wfConf.TailsIncluded = append(wfConf.TailsIncluded, cachedTx.GetTransaction().GetTxHash())

		// incorporate the mutations in accordance with the previous mutations
		// in the milestone's confirming cone/previous ledger state.
		for addr, balance := range patchedState {
			wfConf.NewAddressState[addr] = balance
		}

		// incorporate the mutations in accordance with the previous mutations
		for addr, mutation := range validMutations {
			wfConf.AddressMutations[addr] = wfConf.AddressMutations[addr] + mutation
		}

		return nil
	}

	// called on missing approvees
	onMissingApprovee := func(approveeHash hornet.Hash) error {
		return fmt.Errorf("%w: transaction %s", tangle.ErrTransactionNotFound, approveeHash.Trytes())
	}

	// called on solid entry points
	onSolidEntryPoint := func(txHash hornet.Hash) {
		// Ignore solid entry points (snapshot milestone included)
	}

	// This function does the DFS and computes the mutations a white-flag confirmation would create.
	// If trunk and branch of a bundle head transaction are both SEPs, are already processed or already confirmed,
	// then the mutations from the transaction retrieved from the stack are accumulated to the given Confirmation struct's mutations.
	// If the popped transaction was used to mutate the Confirmation struct, it will also be appended to Confirmation.TailsIncluded.
	if len(branchHash) == 0 {
		// no branch hash given, only walk trunk
		if err := dag.TraverseApprovees(trunkHash,
			condition,
			consumer,
			onMissingApprovee,
			onSolidEntryPoint,
			true, false, true, nil); err != nil {
			return nil, err
		}
	} else {
		// branch hash given, first walk trunk then branch
		if err := dag.TraverseApproveesTrunkBranch(trunkHash, branchHash[0],
			condition,
			consumer,
			onMissingApprovee,
			onSolidEntryPoint,
			true, false, true, nil); err != nil {
			return nil, err
		}
	}

	// compute merkle tree root hash
	wfConf.MerkleTreeHash = NewHasher(merkleTreeHashFunc).TreeHash(wfConf.TailsIncluded)

	return wfConf, nil
}
