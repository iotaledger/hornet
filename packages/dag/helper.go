package dag

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

var (
	ErrFindAllTailsFailed = errors.New("Unable to find all tails")
	ErrWalkAborted        = errors.New("Traversing milestone was aborted")
)

func FindAllTails(txHash trinary.Hash, forceRelease bool) (map[string]struct{}, error) {

	txsToTraverse := make(map[string]struct{})
	txsChecked := make(map[string]struct{})
	tails := make(map[string]struct{})

	txsToTraverse[txHash] = struct{}{}

	// Collect all tx to check by traversing the tangle
	// Loop as long as new transactions are added in every loop cycle
	for len(txsToTraverse) != 0 {

		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)

			if _, checked := txsChecked[txHash]; checked {
				// Tx was already checked => ignore
				continue
			}
			txsChecked[txHash] = struct{}{}

			if tangle.SolidEntryPointsContain(txHash) {
				// Ignore solid entry points (snapshot milestone included)
				continue
			}

			cachedTx := tangle.GetCachedTransactionOrNil(txHash) // tx +1
			if cachedTx == nil {
				return nil, errors.Wrapf(ErrFindAllTailsFailed, "Transaction not found: %v", txHash)
			}

			if cachedTx.GetTransaction().IsTail() {
				tails[txHash] = struct{}{}
				cachedTx.Release(forceRelease) // tx -1
				continue
			}

			// Mark the approvees to be traversed
			txsToTraverse[cachedTx.GetTransaction().GetTrunk()] = struct{}{}
			txsToTraverse[cachedTx.GetTransaction().GetBranch()] = struct{}{}
			cachedTx.Release(forceRelease) // tx -1
		}
	}
	return tails, nil
}

// GetMilestoneApprovees traverses a milestone and collects all tx that were confirmed by that milestone or higher
func GetMilestoneApprovees(milestoneIndex milestone_index.MilestoneIndex, cachedMsTailTx *tangle.CachedTransaction, collectForPruning bool, abortSignal <-chan struct{}) ([]trinary.Hash, error) {

	defer cachedMsTailTx.Release(true) // tx -1

	txsToTraverse := make(map[trinary.Hash]struct{})
	txsChecked := make(map[trinary.Hash]struct{})
	var approvees []trinary.Hash
	txsToTraverse[cachedMsTailTx.GetTransaction().GetHash()] = struct{}{}

	// Collect all tx by traversing the tangle
	// Loop as long as new transactions are added in every loop cycle
	for len(txsToTraverse) != 0 {

		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)

			select {
			case <-abortSignal:
				return nil, ErrWalkAborted
			default:
			}

			if _, checked := txsChecked[txHash]; checked {
				// Tx was already checked => ignore
				continue
			}
			txsChecked[txHash] = struct{}{}

			if tangle.SolidEntryPointsContain(txHash) {
				// Ignore solid entry points (snapshot milestone included)
				continue
			}

			cachedTx := tangle.GetCachedTransactionOrNil(txHash) // tx +1
			if cachedTx == nil {
				if !collectForPruning {
					panic(fmt.Sprintf("GetMilestoneApprovees: Transaction not found: %v", txHash))
				}

				// Go on if the tx is missing (needed for pruning of the database)
				continue
			}

			if confirmed, at := cachedTx.GetMetadata().GetConfirmed(); confirmed {
				if at < milestoneIndex {
					// Ignore Tx that were confirmed by older milestones
					cachedTx.Release(true) // tx -1
					continue
				}
			} else {
				// Tx is not confirmed
				// ToDo: This shouldn't happen, but it does since tipselection allows it at the moment
				if !collectForPruning {
					if cachedTx.GetTransaction().IsTail() {
						cachedTx.Release(true) // tx -1
						panic(fmt.Sprintf("GetMilestoneApprovees: Transaction not confirmed: %v", txHash))
					}

					// Search all referenced tails of this Tx (needed for correct SolidEntryPoint calculation).
					// This non-tail tx was not confirmed by the milestone, and could be referenced by the future cone.
					// Thats why we have to search all tail txs that get referenced by this incomplete bundle, to mark them as SEPs.
					tailTxs, err := FindAllTails(txHash, true)
					if err != nil {
						cachedTx.Release(true) // tx -1
						return nil, err
					}

					for tailTx := range tailTxs {
						txsToTraverse[tailTx] = struct{}{}
					}

					// Ignore this transaction in the cone because it is not confirmed
					cachedTx.Release(true) // tx -1
					continue
				}

				// Check if the we can walk further => if not, it should be fine (only used for pruning anyway)
				if !tangle.ContainsTransaction(cachedTx.GetTransaction().GetTrunk()) {
					// Do not force release, since it is loaded again
					cachedTx.Release() // tx -1
					approvees = append(approvees, txHash)
					continue
				}

				if !tangle.ContainsTransaction(cachedTx.GetTransaction().GetBranch()) {
					// Do not force release, since it is loaded again
					cachedTx.Release() // tx -1
					approvees = append(approvees, txHash)
					continue
				}
			}

			approvees = append(approvees, txHash)

			// Traverse the approvee
			txsToTraverse[cachedTx.GetTransaction().GetTrunk()] = struct{}{}
			txsToTraverse[cachedTx.GetTransaction().GetBranch()] = struct{}{}

			// Do not force release, since it is loaded again
			cachedTx.Release() // tx -1
		}
	}

	return approvees, nil
}
