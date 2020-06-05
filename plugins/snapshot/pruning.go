package snapshot

import (
	"time"

	"github.com/iotaledger/iota.go/consts"

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
func pruneUnconfirmedTransactions(targetIndex milestone.Index) int {

	txsToCheckMap := make(map[string]struct{})

	// Check if tx is still unconfirmed
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

		// do not force release, since it is loaded again
		cachedTx.Release() // tx -1
		txsToCheckMap[string(txHash)] = struct{}{}
	}

	txCount := pruneTransactions(txsToCheckMap)
	tangle.DeleteUnconfirmedTxs(targetIndex)

	return txCount
}

// pruneMilestone prunes the milestone metadata and the ledger diffs from the database for the given milestone
func pruneMilestone(milestoneIndex milestone.Index) {

	// state diffs
	if err := tangle.DeleteLedgerDiffForMilestone(milestoneIndex); err != nil {
		log.Error(err)
	}

	tangle.DeleteMilestone(milestoneIndex)
}

// pruneTransactions prunes the approvers, bundles, bundle txs, addresses, tags and transaction metadata from the database
func pruneTransactions(txsToCheckMap map[string]struct{}) int {

	txsToDeleteMap := make(map[string]struct{})

	for txHashToCheck := range txsToCheckMap {

		cachedTx := tangle.GetCachedTransactionOrNil(hornet.Hash(txHashToCheck)) // tx +1
		if cachedTx == nil {
			log.Warnf("pruneTransactions: Transaction not found: %s", txHashToCheck)
			continue
		}

		for txToRemove := range tangle.RemoveTransactionFromBundle(cachedTx.GetTransaction()) {
			txsToDeleteMap[txToRemove] = struct{}{}
		}
		// since it gets loaded below again it doesn't make sense to force release here
		cachedTx.Release() // tx -1
	}

	for txHashToDelete := range txsToDeleteMap {

		if txHashToDelete == consts.NullHashTrytes {
			// do not delete genesis transaction
			continue
		}

		cachedTx := tangle.GetCachedTransactionOrNil(hornet.Hash(txHashToDelete)) // tx +1
		if cachedTx == nil {
			continue
		}

		tx := cachedTx.GetTransaction()
		cachedTx.Release(true) // tx -1

		// Delete the reference in the approvees
		tangle.DeleteApprover(tx.GetTrunkHash(), cachedTx.GetTransaction().GetTxHash())
		tangle.DeleteApprover(tx.GetBranchHash(), cachedTx.GetTransaction().GetTxHash())

		tangle.DeleteTag(cachedTx.GetTransaction().GetTag(), cachedTx.GetTransaction().GetTxHash())
		tangle.DeleteAddress(cachedTx.GetTransaction().GetAddress(), cachedTx.GetTransaction().GetTxHash())
		tangle.DeleteApprovers(cachedTx.GetTransaction().GetTxHash())
		tangle.DeleteTransaction(cachedTx.GetTransaction().GetTxHash())
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
		return ErrNotEnoughHistory
	}

	targetIndexMax := snapshotInfo.SnapshotIndex - SolidEntryPointCheckThresholdPast - AdditionalPruningThreshold - 1
	if targetIndex > targetIndexMax {
		targetIndex = targetIndexMax
	}

	if snapshotInfo.PruningIndex >= targetIndex {
		// no pruning needed
		return ErrNoPruningNeeded
	}

	if snapshotInfo.EntryPointIndex+AdditionalPruningThreshold > targetIndex {
		// we prune in "AdditionalPruningThreshold" steps to recalculate the solidEntryPoints
		return ErrNotEnoughHistory
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
		txCount := pruneUnconfirmedTransactions(milestoneIndex)

		cachedMs := tangle.GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
		if cachedMs == nil {
			// Milestone not found, pruning impossible
			log.Warnf("Pruning milestone (%d) failed! Milestone not found!", milestoneIndex)
			continue
		}

		// Get all approvees of that milestone
		cachedMsTailTx := tangle.GetCachedTransactionOrNil(cachedMs.GetMilestone().Hash) // tx +1
		cachedMs.Release(true)                                                           // milestone -1

		if cachedMsTailTx == nil {
			// Milestone tail not found, pruning impossible
			log.Warnf("Pruning milestone (%d) failed! Milestone tail tx not found!", milestoneIndex)
			continue
		}
		approvees, err := getMilestoneApprovees(milestoneIndex, cachedMsTailTx.Retain(), true, nil)
		cachedMsTailTx.Release(true) // tx -1

		if err != nil {
			log.Warnf("Pruning milestone (%d) failed! Error: %v", milestoneIndex, err)
			continue
		}

		txsToCheckMap := make(map[string]struct{})
		for _, approvee := range approvees {
			txsToCheckMap[string(approvee)] = struct{}{}
		}

		txCount += pruneTransactions(txsToCheckMap)

		pruneMilestone(milestoneIndex)

		snapshotInfo.PruningIndex = milestoneIndex
		tangle.SetSnapshotInfo(snapshotInfo)

		log.Infof("Pruning milestone (%d) took %v. Pruned %d transactions. ", milestoneIndex, time.Since(ts), txCount)

		tanglePlugin.Events.PruningMilestoneIndexChanged.Trigger(milestoneIndex)
	}

	database.RunGarbageCollection()

	return nil
}
