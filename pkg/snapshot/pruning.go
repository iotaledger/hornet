package snapshot

import (
	"context"
	"math"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/contextutils"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
)

func (s *SnapshotManager) setIsPruning(value bool) {
	s.statusLock.Lock()
	s.isPruning = value
	s.storage.Events.PruningStateChanged.Trigger(value)
	s.statusLock.Unlock()
}

func (s *SnapshotManager) calcTargetIndexBySize(targetSizeBytes ...int64) (milestone.Index, error) {

	if !s.pruningSizeEnabled && len(targetSizeBytes) == 0 {
		// pruning by size deactivated
		return 0, ErrNoPruningNeeded
	}

	if !s.tangleDatabase.CompactionSupported() || !s.utxoDatabase.CompactionSupported() {
		return 0, ErrDatabaseCompactionNotSupported
	}

	if s.tangleDatabase.CompactionRunning() || s.utxoDatabase.CompactionRunning() {
		return 0, ErrDatabaseCompactionRunning
	}

	currentTangleDatabaseSizeBytes, err := s.tangleDatabase.Size()
	if err != nil {
		return 0, err
	}

	currentUTXODatabaseSizeBytes, err := s.utxoDatabase.Size()
	if err != nil {
		return 0, err
	}

	currentDatabaseSizeBytes := currentTangleDatabaseSizeBytes + currentUTXODatabaseSizeBytes

	targetDatabaseSizeBytes := s.pruningSizeTargetSizeBytes
	if len(targetSizeBytes) > 0 {
		targetDatabaseSizeBytes = targetSizeBytes[0]
	}

	if targetDatabaseSizeBytes <= 0 {
		// pruning by size deactivated
		return 0, ErrNoPruningNeeded
	}

	if currentDatabaseSizeBytes < targetDatabaseSizeBytes {
		return 0, ErrNoPruningNeeded
	}

	milestoneRange := s.syncManager.ConfirmedMilestoneIndex() - s.storage.SnapshotInfo().PruningIndex
	prunedDatabaseSizeBytes := float64(targetDatabaseSizeBytes) * ((100.0 - s.pruningSizeThresholdPercentage) / 100.0)
	diffPercentage := prunedDatabaseSizeBytes / float64(currentDatabaseSizeBytes)
	milestoneDiff := milestone.Index(math.Ceil(float64(milestoneRange) * diffPercentage))

	return s.syncManager.ConfirmedMilestoneIndex() - milestoneDiff, nil
}

// pruneUnreferencedBlocks prunes all unreferenced blocks from the database for the given milestone
func (s *SnapshotManager) pruneUnreferencedBlocks(targetIndex milestone.Index) (blocksCountDeleted int, blocksCountChecked int) {

	blockIDsToDeleteMap := make(map[string]struct{})

	// Check if block is still unreferenced
	for _, blockID := range s.storage.UnreferencedBlockIDs(targetIndex) {
		blockIDMapKey := blockID.ToMapKey()
		if _, exists := blockIDsToDeleteMap[blockIDMapKey]; exists {
			continue
		}

		cachedBlockMeta := s.storage.CachedBlockMetadataOrNil(blockID) // meta +1
		if cachedBlockMeta == nil {
			// block was already deleted or marked for deletion
			continue
		}

		if cachedBlockMeta.Metadata().IsReferenced() {
			// block was already referenced
			cachedBlockMeta.Release(true) // meta -1
			continue
		}

		cachedBlockMeta.Release(true) // meta -1
		blockIDsToDeleteMap[blockIDMapKey] = struct{}{}
	}

	blocksCountDeleted = s.pruneBlocks(blockIDsToDeleteMap)
	s.storage.DeleteUnreferencedBlocks(targetIndex)

	return blocksCountDeleted, len(blockIDsToDeleteMap)
}

// pruneMilestone prunes the milestone metadata and the ledger diffs from the database for the given milestone
func (s *SnapshotManager) pruneMilestone(milestoneIndex milestone.Index, receiptMigratedAtIndex ...uint32) error {

	if err := s.utxoManager.PruneMilestoneIndexWithoutLocking(milestoneIndex, s.pruneReceipts, receiptMigratedAtIndex...); err != nil {
		return err
	}

	s.storage.DeleteMilestone(milestoneIndex)

	return nil
}

// pruneBlocks removes all the associated data of the given block IDs from the database
func (s *SnapshotManager) pruneBlocks(blockIDsToDeleteMap map[string]struct{}) int {

	for blockIDToDelete := range blockIDsToDeleteMap {

		blockID := hornet.BlockIDFromMapKey(blockIDToDelete)

		cachedBlockMeta := s.storage.CachedBlockMetadataOrNil(blockID) // meta +1
		if cachedBlockMeta == nil {
			continue
		}

		cachedBlockMeta.ConsumeMetadata(func(metadata *storage.BlockMetadata) { // meta -1
			// Delete the reference in the parents
			for _, parent := range metadata.Parents() {
				s.storage.DeleteChild(parent, blockID)
			}

			// We don't need to iterate through the children that reference this block,
			// since we will never start the walk from this block anymore (we only walk the future cone)
			// and the references will be deleted together with the children blocks when they are pruned.
		})

		s.storage.DeleteBlock(blockID)
	}

	return len(blockIDsToDeleteMap)
}

func (s *SnapshotManager) pruneDatabase(ctx context.Context, targetIndex milestone.Index) (milestone.Index, error) {

	if err := contextutils.ReturnErrIfCtxDone(ctx, common.ErrOperationAborted); err != nil {
		// do not prune the database if the node was shut down
		return 0, err
	}

	if s.tangleDatabase.CompactionRunning() || s.utxoDatabase.CompactionRunning() {
		return 0, ErrDatabaseCompactionRunning
	}

	snapshotInfo := s.storage.SnapshotInfo()
	if snapshotInfo == nil {
		s.LogPanic("No snapshotInfo found!")
	}

	//lint:ignore SA5011 nil pointer is already checked before with a panic
	if snapshotInfo.SnapshotIndex < s.solidEntryPointCheckThresholdPast+s.additionalPruningThreshold+1 {
		// Not enough history
		return 0, errors.Wrapf(ErrNotEnoughHistory, "minimum index: %d, target index: %d", s.solidEntryPointCheckThresholdPast+s.additionalPruningThreshold+1, targetIndex)
	}

	targetIndexMax := snapshotInfo.SnapshotIndex - s.solidEntryPointCheckThresholdPast - s.additionalPruningThreshold - 1
	if targetIndex > targetIndexMax {
		targetIndex = targetIndexMax
	}

	if snapshotInfo.PruningIndex >= targetIndex {
		// no pruning needed
		return 0, errors.Wrapf(ErrNoPruningNeeded, "pruning index: %d, target index: %d", snapshotInfo.PruningIndex, targetIndex)
	}

	if snapshotInfo.EntryPointIndex+s.additionalPruningThreshold+1 > targetIndex {
		// we prune in "additionalPruningThreshold" steps to recalculate the solidEntryPoints
		return 0, errors.Wrapf(ErrNotEnoughHistory, "minimum index: %d, target index: %d", snapshotInfo.EntryPointIndex+s.additionalPruningThreshold+1, targetIndex)
	}

	s.setIsPruning(true)
	defer s.setIsPruning(false)

	// calculate solid entry points for the new end of the tangle history
	var solidEntryPoints []*storage.SolidEntryPoint
	err := forEachSolidEntryPoint(
		ctx,
		s.storage,
		targetIndex,
		s.solidEntryPointCheckThresholdPast,
		func(sep *storage.SolidEntryPoint) bool {
			solidEntryPoints = append(solidEntryPoints, sep)
			return true
		})
	if err != nil {
		return 0, err
	}

	// temporarily add the new solid entry points and keep the old ones
	s.storage.WriteLockSolidEntryPoints()
	for _, sep := range solidEntryPoints {
		s.storage.SolidEntryPointsAddWithoutLocking(sep.BlockID, sep.Index)
	}
	if err = s.storage.StoreSolidEntryPointsWithoutLocking(); err != nil {
		s.LogPanic(err)
	}
	s.storage.WriteUnlockSolidEntryPoints()

	// we have to set the new solid entry point index.
	// this way we can cleanly prune even if the pruning was aborted last time
	snapshotInfo.EntryPointIndex = targetIndex
	if err = s.storage.SetSnapshotInfo(snapshotInfo); err != nil {
		s.LogPanic(err)
	}

	// unreferenced blocks have to be pruned for PruningIndex as well, since this could be CMI at startup of the node
	s.pruneUnreferencedBlocks(snapshotInfo.PruningIndex)

	// Iterate through all milestones that have to be pruned
	for milestoneIndex := snapshotInfo.PruningIndex + 1; milestoneIndex <= targetIndex; milestoneIndex++ {

		if err := contextutils.ReturnErrIfCtxDone(ctx, ErrPruningAborted); err != nil {
			// stop pruning if node was shutdown
			return 0, err
		}

		s.LogInfof("Pruning milestone (%d)...", milestoneIndex)

		timeStart := time.Now()
		blocksCountDeleted, blocksCountChecked := s.pruneUnreferencedBlocks(milestoneIndex)
		timePruneUnreferencedBlocks := time.Now()

		// get all parents of that milestone
		cachedMilestone := s.storage.CachedMilestoneByIndexOrNil(milestoneIndex) // milestone +1
		if cachedMilestone == nil {
			// Milestone not found, pruning impossible
			s.LogWarnf("Pruning milestone (%d) failed! Milestone not found!", milestoneIndex)
			continue
		}

		blockIDsToDeleteMap := make(map[string]struct{})

		if err := dag.TraverseParents(
			ctx,
			s.storage,
			cachedMilestone.Milestone().Parents(),
			// traversal stops if no more blocks pass the given condition
			// Caution: condition func is not in DFS order
			func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
				defer cachedBlockMeta.Release(true) // meta -1
				// everything that was referenced by that milestone can be pruned (even blocks of older milestones)
				return true, nil
			},
			// consumer
			func(cachedBlockMeta *storage.CachedMetadata) error { // meta +1
				defer cachedBlockMeta.Release(true) // meta -1
				blockIDsToDeleteMap[cachedBlockMeta.Metadata().BlockID().ToMapKey()] = struct{}{}
				return nil
			},
			// called on missing parents
			func(parentBlockID hornet.BlockID) error { return nil },
			// called on solid entry points
			// Ignore solid entry points (snapshot milestone included)
			nil,
			// the pruning target index is also a solid entry point => traverse it anyways
			true); err != nil {
			cachedMilestone.Release(true) // milestone -1
			s.LogWarnf("Pruning milestone (%d) failed! %s", milestoneIndex, err)
			continue
		}
		timeTraverseMilestoneCone := time.Now()

		// check whether milestone contained receipt and delete it accordingly
		var migratedAtIndex []uint32

		opts, err := cachedMilestone.Milestone().Milestone().Opts.Set()
		if err == nil && opts != nil {
			if r := opts.Receipt(); r != nil {
				migratedAtIndex = append(migratedAtIndex, r.MigratedAt)
			}
		}

		cachedMilestone.Release(true) // milestone -1

		if err := s.pruneMilestone(milestoneIndex, migratedAtIndex...); err != nil {
			s.LogWarnf("Pruning milestone (%d) failed! %s", milestoneIndex, err)
		}
		timePruneMilestone := time.Now()

		blocksCountChecked += len(blockIDsToDeleteMap)
		blocksCountDeleted += s.pruneBlocks(blockIDsToDeleteMap)
		timePruneBlocks := time.Now()

		snapshotInfo.PruningIndex = milestoneIndex
		if err = s.storage.SetSnapshotInfo(snapshotInfo); err != nil {
			s.LogPanic(err)
		}
		timeSetSnapshotInfo := time.Now()

		s.LogInfof("Pruning milestone (%d) took %v. Pruned %d/%d blocks. ", milestoneIndex, time.Since(timeStart).Truncate(time.Millisecond), blocksCountDeleted, blocksCountChecked)

		s.Events.PruningMilestoneIndexChanged.Trigger(milestoneIndex)
		timePruningMilestoneIndexChanged := time.Now()

		s.Events.PruningMetricsUpdated.Trigger(&PruningMetrics{
			DurationPruneUnreferencedBlocks:      timePruneUnreferencedBlocks.Sub(timeStart),
			DurationTraverseMilestoneCone:        timeTraverseMilestoneCone.Sub(timePruneUnreferencedBlocks),
			DurationPruneMilestone:               timePruneMilestone.Sub(timeTraverseMilestoneCone),
			DurationPruneBlocks:                  timePruneBlocks.Sub(timePruneMilestone),
			DurationSetSnapshotInfo:              timeSetSnapshotInfo.Sub(timePruneBlocks),
			DurationPruningMilestoneIndexChanged: timePruningMilestoneIndexChanged.Sub(timeSetSnapshotInfo),
			DurationTotal:                        time.Since(timeStart),
		})
	}

	// finally set the new solid entry points and remove the old ones
	s.storage.WriteLockSolidEntryPoints()
	s.storage.ResetSolidEntryPointsWithoutLocking()
	for _, sep := range solidEntryPoints {
		s.storage.SolidEntryPointsAddWithoutLocking(sep.BlockID, sep.Index)
	}
	if err = s.storage.StoreSolidEntryPointsWithoutLocking(); err != nil {
		s.LogPanic(err)
	}
	s.storage.WriteUnlockSolidEntryPoints()

	return targetIndex, nil
}

func (s *SnapshotManager) PruneDatabaseByDepth(ctx context.Context, depth milestone.Index) (milestone.Index, error) {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	confirmedMilestoneIndex := s.syncManager.ConfirmedMilestoneIndex()

	if confirmedMilestoneIndex <= depth {
		// Not enough history
		return 0, ErrNotEnoughHistory
	}

	return s.pruneDatabase(ctx, confirmedMilestoneIndex-depth)
}

func (s *SnapshotManager) PruneDatabaseByTargetIndex(ctx context.Context, targetIndex milestone.Index) (milestone.Index, error) {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	return s.pruneDatabase(ctx, targetIndex)
}

func (s *SnapshotManager) PruneDatabaseBySize(ctx context.Context, targetSizeBytes int64) (milestone.Index, error) {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	targetIndex, err := s.calcTargetIndexBySize(targetSizeBytes)
	if err != nil {
		return 0, err
	}

	return s.pruneDatabase(ctx, targetIndex)
}
