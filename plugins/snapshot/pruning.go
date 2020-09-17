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
	// AdditionalPruningThreshold is needed, because the messages in the getMilestoneParents call in getSolidEntryPoints
	// can reference older messages as well
	AdditionalPruningThreshold = 50
)

// pruneUnconfirmedTransactions prunes all unconfirmed tx from the database for the given milestone
func pruneUnconfirmedTransactions(targetIndex milestone.Index) (txCountDeleted int, txCountChecked int) {

	txsToCheckMap := make(map[string]struct{})

	// Check if tx is still unconfirmed
loopOverUnconfirmed:
	for _, txHash := range tangle.GetUnconfirmedMessageIDs(targetIndex, true) {
		if _, exists := txsToCheckMap[string(txHash)]; exists {
			continue
		}

		cachedTx := tangle.GetCachedMessageOrNil(txHash) // tx +1
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
					if bndl.GetMessage().IsMilestone() {
						cachedTx.Release(true) // tx -1
						bundles.Release(true)  // bundles -1
						// Do not prune unconfirmed tx that are part of a milestone
						continue loopOverUnconfirmed
					}
				}
				bundles.Release(true) // bundles -1
			}
		}

		cachedTx.Release(true) // tx -1
		txsToCheckMap[string(txHash)] = struct{}{}
	}

	txCountDeleted = pruneTransactions(txsToCheckMap)
	tangle.DeleteUnconfirmedMessages(targetIndex)

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

// pruneTransactions prunes the children, bundles, bundle txs, addresses, tags and transaction metadata from the database
func pruneTransactions(txsToCheckMap map[string]struct{}) int {

	txsToDeleteMap := make(map[string]struct{})

	for txHashToCheck := range txsToCheckMap {

		cachedMsgMeta := tangle.GetCachedMessageMetadataOrNil(hornet.Hash(txHashToCheck)) // meta +1
		if cachedMsgMeta == nil {
			log.Warnf("pruneTransactions: Transaction not found: %s", hornet.Hash(txHashToCheck).Trytes())
			continue
		}

		for txToRemove := range tangle.RemoveTransactionFromBundle(cachedMsgMeta.GetMetadata()) {
			txsToDeleteMap[txToRemove] = struct{}{}
		}
		// metadata can be force release, since we work with transaction objects at deletion
		cachedMsgMeta.Release(true) // tx -1
	}

	for txHashToDelete := range txsToDeleteMap {

		cachedTx := tangle.GetCachedMessageOrNil(hornet.Hash(txHashToDelete)) // tx +1
		if cachedTx == nil {
			continue
		}

		cachedTx.ConsumeMessage(func(tx *hornet.Transaction) { // tx -1
			// Delete the reference in the parents
			tangle.DeleteChild(tx.GetTrunkHash(), tx.GetMessageID())
			tangle.DeleteChild(tx.GetBranchHash(), tx.GetMessageID())

			tangle.DeleteTag(tx.GetTag(), tx.GetMessageID())
			tangle.DeleteAddress(tx.GetAddress(), tx.GetMessageID())
			tangle.DeleteChildren(tx.GetMessageID())
			tangle.DeleteMessage(tx.GetMessageID())
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
		txCountDeleted, msgCountChecked := pruneUnconfirmedTransactions(milestoneIndex)

		cachedMs := tangle.GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
		if cachedMs == nil {
			// Milestone not found, pruning impossible
			log.Warnf("Pruning milestone (%d) failed! Milestone not found!", milestoneIndex)
			continue
		}

		msgsToCheckMap := make(map[string]struct{})

		err := dag.TraverseParents(cachedMs.GetMilestone().MessageID,
			// traversal stops if no more messages pass the given condition
			// Caution: condition func is not in DFS order
			func(cachedMsgMeta *tangle.CachedMetadata) (bool, error) { // tx +1
				defer cachedMsgMeta.Release(true) // tx -1
				// everything that was referenced by that milestone can be pruned (even messages of older milestones)
				return true, nil
			},
			// consumer
			func(cachedMsgMeta *tangle.CachedMetadata) error { // tx +1
				defer cachedMsgMeta.Release(true) // tx -1
				msgsToCheckMap[string(cachedMsgMeta.GetMetadata().GetMessageID())] = struct{}{}
				return nil
			},
			// called on missing parents
			func(parentHash hornet.Hash) error { return nil },
			// called on solid entry points
			// Ignore solid entry points (snapshot milestone included)
			nil,
			// the pruning target index is also a solid entry point => traverse it anyways
			true,
			nil)

		cachedMs.Release(true) // milestone -1
		if err != nil {
			log.Warnf("Pruning milestone (%d) failed! Error: %v", milestoneIndex, err)
			continue
		}

		msgCountChecked += len(msgsToCheckMap)
		txCountDeleted += pruneTransactions(msgsToCheckMap)

		pruneMilestone(milestoneIndex)

		snapshotInfo.PruningIndex = milestoneIndex
		tangle.SetSnapshotInfo(snapshotInfo)

		log.Infof("Pruning milestone (%d) took %v. Pruned %d/%d messages. ", milestoneIndex, time.Since(ts), txCountDeleted, msgCountChecked)

		tanglePlugin.Events.PruningMilestoneIndexChanged.Trigger(milestoneIndex)
	}

	database.RunGarbageCollection()

	return nil
}
