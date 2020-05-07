package snapshot

import (
	"bytes"
	"time"

	"github.com/iotaledger/iota.go/trinary"

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

var (
	NullHashBytes = make([]byte, 49)
)

// pruneUnconfirmedTransactions prunes all unconfirmed tx from the database for the given milestone
func pruneUnconfirmedTransactions(targetIndex milestone.Index) int {

	txsBytesToCheckMap := make(map[string]struct{})

	// Check if tx is still unconfirmed
	for _, txHashBytes := range tangle.GetUnconfirmedTxHashBytes(targetIndex, true) {
		if _, exists := txsBytesToCheckMap[string(txHashBytes)]; exists {
			continue
		}

		storedTx := tangle.GetStoredTransactionOrNil(txHashBytes)
		if storedTx == nil {
			// Tx was already pruned
			continue
		}

		storedTxMeta := tangle.GetStoredMetadataOrNil(txHashBytes)
		if storedTxMeta.IsConfirmed() {
			// Tx was confirmed => skip
			continue
		}

		txsBytesToCheckMap[string(txHashBytes)] = struct{}{}
	}

	txCount := pruneTransactions(txsBytesToCheckMap)
	tangle.DeleteUnconfirmedTxsFromBadger(targetIndex)

	return txCount
}

// pruneMilestone prunes the milestone metadata and the ledger diffs from the database for the given milestone
func pruneMilestone(milestoneIndex milestone.Index) {

	// state diffs
	if err := tangle.DeleteLedgerDiffForMilestone(milestoneIndex); err != nil {
		log.Error(err)
	}

	tangle.DeleteMilestoneFromBadger(milestoneIndex)
}

// pruneTransactions prunes the approvers, bundles, bundle txs, addresses, tags and transaction metadata from the database
func pruneTransactions(txsBytesToCheckMap map[string]struct{}) int {

	txsBytesToDeleteMap := make(map[string]struct{})

	for txHashToCheck := range txsBytesToCheckMap {
		txHashBytesToCheck := []byte(txHashToCheck)

		storedTx := tangle.GetStoredTransactionOrNil(txHashBytesToCheck) // tx +1
		if storedTx == nil {
			log.Warnf("pruneTransactions: Transaction not found: %v", trinary.MustBytesToTrytes(txHashBytesToCheck, 81))
			continue
		}

		for txToRemove := range tangle.RemoveTransactionFromBundle(storedTx.Tx) {
			txsBytesToDeleteMap[string(trinary.MustTrytesToBytes(txToRemove)[:49])] = struct{}{}
		}
	}

	for txHashToDelete := range txsBytesToDeleteMap {

		txHashBytesToDelete := []byte(txHashToDelete)
		if bytes.Equal(txHashBytesToDelete, NullHashBytes) {
			// do not delete genesis transaction
			continue
		}

		storedTx := tangle.GetStoredTransactionOrNil(txHashBytesToDelete)
		if storedTx == nil {
			continue
		}

		// Delete the reference in the approvees
		tangle.DeleteApproverFromBadger(storedTx.GetTrunk(), txHashBytesToDelete)
		tangle.DeleteApproverFromBadger(storedTx.GetBranch(), txHashBytesToDelete)

		tangle.DeleteTagFromBadger(storedTx.Tx.Tag, txHashBytesToDelete)
		tangle.DeleteAddressFromBadger(storedTx.Tx.Address, txHashBytesToDelete)
		tangle.DeleteApproversFromBadger(txHashBytesToDelete)
		tangle.DeleteTransactionFromBadger(txHashBytesToDelete)
	}

	return len(txsBytesToDeleteMap)
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
		tangle.SolidEntryPointsAdd(solidEntryPoint, index)
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

		txsBytesToCheckMap := make(map[string]struct{})
		for _, approvee := range approvees {
			txsBytesToCheckMap[string(trinary.MustTrytesToBytes(approvee)[:49])] = struct{}{}
		}

		txCount += pruneTransactions(txsBytesToCheckMap)

		pruneMilestone(milestoneIndex)

		snapshotInfo.PruningIndex = milestoneIndex
		tangle.SetSnapshotInfo(snapshotInfo)

		log.Infof("Pruning milestone (%d) took %v. Pruned %d transactions. ", milestoneIndex, time.Since(ts), txCount)

		tanglePlugin.Events.PruningMilestoneIndexChanged.Trigger(milestoneIndex)
	}

	database.RunFullGarbageCollection()

	return nil
}
