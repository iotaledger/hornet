package snapshot

import (
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/plugins/database"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

const (
	// AdditionalPruningThreshold is needed, because the transactions in the getMilestoneApprovees call in getSolidEntryPoints
	// can reference older transactions as well
	AdditionalPruningThreshold = 50
)

// pruneUnconfirmedTransactions prunes all unconfirmed tx from the database for the given milestone
func pruneUnconfirmedTransactions(targetIndex milestone.Index) (txCountDeleted int, txCountChecked int) {

	txsToCheckMap := make(map[string]struct{})

	// Check if tx is still unconfirmed
loopOverUnconfirmed:
	for _, txHash := range tangle.GetUnconfirmedTxHashes(targetIndex, true) {
		if _, exists := txsToCheckMap[string(txHash)]; exists {
			continue
		}

		cachedTx := tangle.GetCachedTransactionOrNil(txHash) // tx +1
		if cachedTx == nil {
			// transaction was already deleted or marked for deletion
			continue
		}

		if cachedTx.GetMetadata().IsConfirmed() {
			// transaction was already confirmed
			cachedTx.Release(true) // tx -1
			continue
		}

		if tangle.IsMaybeMilestoneTx(cachedTx.Retain()) {
			bundles := tangle.GetBundlesOfTransactionOrNil(txHash, true) // bundles +1
			if bundles != nil && len(bundles) > 0 {
				for _, bndl := range bundles {
					if bndl.GetBundle().IsMilestone() {
						cachedTx.Release(true) // tx -1
						bundles.Release(true)  // bundles -1
						// Do not prune unconfirmed tx that are part of a milestone
						continue loopOverUnconfirmed
					}
				}
				bundles.Release(true) // bundles -1
			}
		}

		// do not force release, since it is loaded again
		cachedTx.Release() // tx -1
		txsToCheckMap[string(txHash)] = struct{}{}
	}

	txCountDeleted = pruneTransactions(txsToCheckMap)
	tangle.DeleteUnconfirmedTxs(targetIndex)

	return txCountDeleted, len(txsToCheckMap)
}

// pruneMilestone prunes the milestone metadata and the ledger diffs from the database for the given milestone
func pruneMilestone(milestoneIndex milestone.Index) {

	// state diffs
	if err := tangle.DeleteLedgerDiffForMilestone(milestoneIndex); err != nil {
		log.Warn(err)
	}

	tangle.DeleteMilestone(milestoneIndex)
}

// pruneTransactions prunes the approvers, bundles, bundle txs, addresses, tags and transaction metadata from the database
func pruneTransactions(txsToCheckMap map[string]struct{}) int {

	txsToDeleteMap := make(map[string]struct{})

	for txHashToCheck := range txsToCheckMap {

		cachedTxMeta := tangle.GetCachedTxMetadataOrNil(hornet.Hash(txHashToCheck)) // tx +1
		if cachedTxMeta == nil {
			log.Warnf("pruneTransactions: Transaction not found: %s", txHashToCheck)
			continue
		}

		for txToRemove := range tangle.RemoveTransactionFromBundle(cachedTxMeta.GetMetadata()) {
			txsToDeleteMap[txToRemove] = struct{}{}
		}
		// since it gets loaded below again it doesn't make sense to force release here
		cachedTxMeta.Release() // tx -1
	}

	for txHashToDelete := range txsToDeleteMap {

		cachedTx := tangle.GetCachedTransactionOrNil(hornet.Hash(txHashToDelete)) // tx +1
		if cachedTx == nil {
			continue
		}

		cachedTx.ConsumeTransaction(func(tx *hornet.Transaction) { // tx -1
			// Delete the reference in the approvees
			tangle.DeleteApprover(tx.GetTrunkHash(), tx.GetTxHash())
			tangle.DeleteApprover(tx.GetBranchHash(), tx.GetTxHash())

			tangle.DeleteTag(tx.GetTag(), tx.GetTxHash())
			tangle.DeleteAddress(tx.GetAddress(), tx.GetTxHash())
			tangle.DeleteApprovers(tx.GetTxHash())
			tangle.DeleteTransaction(tx.GetTxHash())
		})
	}

	return len(txsToDeleteMap)
}

func setIsPruning(value bool) {
	statusLock.Lock()
	isPruning = value
	statusLock.Unlock()
}

func pruneDatabase(targetIndex milestone.Index, abortSignal <-chan struct{}) error {

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		log.Panic("No snapshotInfo found!")
	}

	if snapshotInfo.SnapshotIndex < SolidEntryPointCheckThresholdPast+AdditionalPruningThreshold+1 {
		// Not enough history
		return errors.Wrapf(ErrNotEnoughHistory, "minimum index: %d, target index: %d", SolidEntryPointCheckThresholdPast+AdditionalPruningThreshold+1, targetIndex)
	}

	targetIndexMax := snapshotInfo.SnapshotIndex - SolidEntryPointCheckThresholdPast - AdditionalPruningThreshold - 1
	if targetIndex > targetIndexMax {
		targetIndex = targetIndexMax
	}

	if snapshotInfo.PruningIndex >= targetIndex {
		// no pruning needed
		return errors.Wrapf(ErrNoPruningNeeded, "pruning index: %d, target index: %d", snapshotInfo.PruningIndex, targetIndex)
	}

	if snapshotInfo.EntryPointIndex+AdditionalPruningThreshold+1 > targetIndex {
		// we prune in "AdditionalPruningThreshold" steps to recalculate the solidEntryPoints
		return errors.Wrapf(ErrNotEnoughHistory, "minimum index: %d, target index: %d", snapshotInfo.EntryPointIndex+AdditionalPruningThreshold+1, targetIndex)
	}

	setIsPruning(true)
	defer setIsPruning(false)

	// calculate solid entry points for the new end of the tangle history
	newSolidEntryPoints, err := getSolidEntryPoints(targetIndex, abortSignal)
	if err != nil {
		return err
	}

	tangle.WriteLockSolidEntryPoints()
	tangle.ResetSolidEntryPoints()
	for solidEntryPoint, index := range newSolidEntryPoints {
		tangle.SolidEntryPointsAdd(hornet.Hash(solidEntryPoint), index)
	}
	tangle.StoreSolidEntryPoints()
	tangle.WriteUnlockSolidEntryPoints()

	// we have to set the new solid entry point index.
	// this way we can cleanly prune even if the pruning was aborted last time
	snapshotInfo.EntryPointIndex = targetIndex
	tangle.SetSnapshotInfo(snapshotInfo)

	// unconfirmed txs have to be pruned for PruningIndex as well, since this could be LSI at startup of the node
	pruneUnconfirmedTransactions(snapshotInfo.PruningIndex)

	// Iterate through all milestones that have to be pruned
	for milestoneIndex := snapshotInfo.PruningIndex + 1; milestoneIndex <= targetIndex; milestoneIndex++ {
		select {
		case <-abortSignal:
			// Stop pruning the next milestone
			return ErrPruningAborted
		default:
		}

		log.Infof("Pruning milestone (%d)...", milestoneIndex)

		ts := time.Now()
		txCountDeleted, txCountChecked := pruneUnconfirmedTransactions(milestoneIndex)

		cachedMs := tangle.GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
		if cachedMs == nil {
			// Milestone not found, pruning impossible
			log.Warnf("Pruning milestone (%d) failed! Milestone not found!", milestoneIndex)
			continue
		}

		txsToCheckMap := make(map[string]struct{})

		err := dag.TraverseApprovees(cachedMs.GetMilestone().Hash,
			// traversal stops if no more transactions pass the given condition
			// Caution: condition func is not in DFS order
			func(cachedTxMeta *tangle.CachedMetadata) (bool, error) { // tx +1
				defer cachedTxMeta.Release(true) // tx -1
				// everything that was referenced by that milestone can be pruned (even transactions of older milestones)
				return true, nil
			},
			// consumer
			func(cachedTxMeta *tangle.CachedMetadata) error { // tx +1
				defer cachedTxMeta.Release(true) // tx -1
				txsToCheckMap[string(cachedTxMeta.GetMetadata().GetTxHash())] = struct{}{}
				return nil
			},
			// called on missing approvees
			func(approveeHash hornet.Hash) error { return nil },
			// called on solid entry points
			// Ignore solid entry points (snapshot milestone included)
			nil,
			// the pruning target index is also a solid entry point => traverse it anyways
			true,
			false,
			nil)

		cachedMs.Release(true) // milestone -1
		if err != nil {
			log.Warnf("Pruning milestone (%d) failed! Error: %v", milestoneIndex, err)
			continue
		}

		txCountChecked += len(txsToCheckMap)
		txCountDeleted += pruneTransactions(txsToCheckMap)

		pruneMilestone(milestoneIndex)

		snapshotInfo.PruningIndex = milestoneIndex
		tangle.SetSnapshotInfo(snapshotInfo)

		log.Infof("Pruning milestone (%d) took %v. Pruned %d/%d transactions. ", milestoneIndex, time.Since(ts), txCountDeleted, txCountChecked)

		tanglePlugin.Events.PruningMilestoneIndexChanged.Trigger(milestoneIndex)
	}

	database.RunGarbageCollection()

	return nil
}
