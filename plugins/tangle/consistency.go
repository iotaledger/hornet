package tangle

import (
	"errors"

	"github.com/iotaledger/iota.go/trinary"
	"github.com/gohornet/hornet/packages/datastructure"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

const (
	RefsAnInvalidBundleCacheSize = 10000
)

var (
	ErrRefBundleNotValid    = errors.New("a referenced bundle is invalid")
	ErrRefBundleNotComplete = errors.New("a referenced bundle is not complete")

	RefsAnInvalidBundleCache = datastructure.NewLRUCache(RefsAnInvalidBundleCacheSize)
)

// CheckConsistencyOfConeAndMutateDiff checks whether cone referenced by the given tail transaction is consistent with the current diff.
// this function mutates the approved, respectively walked transaction hashes and the diff with the cone diff,
// in case the tail transaction is consistent with the latest ledger state.
func CheckConsistencyOfConeAndMutateDiff(tailTxHash trinary.Hash, approved map[trinary.Hash]struct{}, diff map[trinary.Hash]int64) bool {

	// make a copy of approved, respectively visited transactions
	visited := make(map[trinary.Hash]struct{}, len(approved))
	for k := range approved {
		visited[k] = struct{}{}
	}

	// compute the diff of the cone which the transaction references
	coneDiff, err := computeConeDiff(visited, tailTxHash, tangle.GetSolidMilestoneIndex())
	if err != nil {
		if err == ErrRefBundleNotValid {
			// memorize for a certain time that this transaction references an invalid bundle
			// to short circuit validation during a subsequent tip-sel on it again
			RefsAnInvalidBundleCache.Set(tailTxHash, true)
		}
		return false
	}

	// if the cone didn't create any mutations, it is automatically consistent with our current diff
	if len(coneDiff) == 0 {
		// we still need to add the visited txs during the cone diff computation
		for k := range visited {
			approved[k] = struct{}{}
		}
		return true
	}

	// apply the walker diff to the cone diff
	for addr, change := range diff {
		coneDiff[addr] += change
	}

	// the cone diff is now an aggregated mutation of the current walker plus the newly walked transaction's cone

	// compute a patched state of the ledger where we would have applied the cone diff to it
	for addr, change := range coneDiff {
		currentLedgerBalance, _, err := tangle.GetBalanceForAddressWithoutLocking(addr)
		if err != nil {
			log.Panic(err)
		}

		// apply the latest ledger state's balance of the given address to the cone diff
		change += int64(currentLedgerBalance)

		// the change reflects now a patched state representing the changes from the latest
		// ledger state to the given transaction. if the balance is now negative, the cone diff is not
		// consistent with the latest ledger state
		if change < 0 {
			return false
		}
	}

	// replace our diff with entries from the cone diff (which now represents the aggregated mutation).
	// we can't just take the cone diff, as we might be in the second walk and therefore would lose the diffs
	// from the first walk, which are not part of this tail transaction's cone
	for addr, change := range coneDiff {
		diff[addr] = change
	}

	// add all visited txs to the approved set
	for k := range visited {
		approved[k] = struct{}{}
	}

	return true
}

// computes the diff of the cone by collecting all mutations of transactions directly/indirectly referenced by the given tail.
// only the non yet visited transactions are collected
func computeConeDiff(visited map[trinary.Hash]struct{}, tailTxHash trinary.Hash, latestSolidMilestoneIndex milestone_index.MilestoneIndex) (map[trinary.Trytes]int64, error) {

	coneDiff := map[trinary.Trytes]int64{}
	txsToTraverse := make(map[string]struct{})
	txsToTraverse[tailTxHash] = struct{}{}

	for len(txsToTraverse) != 0 {
		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)

			// visited contains the solid entry points
			if _, alreadyVisited := visited[txHash]; alreadyVisited {
				continue
			}
			visited[txHash] = struct{}{}

			// check whether we previously checked that this referenced tx references an invalid bundle
			if RefsAnInvalidBundleCache.Contains(txHash) {
				return nil, ErrRefBundleNotValid
			}

			tx, err := tangle.GetTransaction(txHash)
			if err != nil {
				log.Panic(err)
			}

			// ledger update process is write locked
			confirmed, at := tx.GetConfirmed()
			if confirmed {
				if at > latestSolidMilestoneIndex {
					log.Panicf("transaction %s was confirmed by a newer milestone %d", tx.GetHash(), at)
				}
				// only take transactions into account that have not been confirmed by the referenced or older milestones
				continue
			}

			// we only load up bundles when we're traversing tails, so we don't
			// check the same bundle twice, however, we still add the trunk and branch of the
			// bundle transaction to ensure, that if a transaction within the bundle would reference
			// another trunk (as seen from the view of the bundle), we'd get that cone too.
			if !tx.IsTail() {
				txsToTraverse[tx.GetTrunk()] = struct{}{}
				txsToTraverse[tx.GetBranch()] = struct{}{}
				continue
			}

			bundleBucket, err := tangle.GetBundleBucket(tx.Tx.Bundle)
			if err != nil {
				return nil, err
			}

			bundle := bundleBucket.GetBundleOfTailTransaction(tx.GetHash())
			if bundle == nil || !bundle.IsComplete() {
				return nil, ErrRefBundleNotComplete
			}

			if !bundle.IsValid() {
				return nil, ErrRefBundleNotValid
			}

			ledgerChanges, isValueSpamBundle := bundle.GetLedgerChanges()
			if !isValueSpamBundle {
				for addr, change := range ledgerChanges {
					coneDiff[addr] += change
				}
			}

			txsToTraverse[tx.GetTrunk()] = struct{}{}
			txsToTraverse[tx.GetBranch()] = struct{}{}
		}
	}

	return coneDiff, nil
}
