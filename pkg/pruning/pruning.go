package pruning

import (
	"context"
	"math"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/contextutils"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/database"
	storagepkg "github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// AdditionalPruningThreshold is the additional threshold (to BMD), which is needed, because the blocks in the getMilestoneParents call in solidEntryPoints
	// can reference older blocks as well.
	AdditionalPruningThreshold = 5
)

var (
	ErrNotEnoughHistory                                        = errors.New("not enough history")
	ErrNoPruningNeeded                                         = errors.New("no pruning needed")
	ErrPruningAborted                                          = errors.New("pruning was aborted")
	ErrDatabaseCompactionNotSupported                          = errors.New("database compaction not supported")
	ErrDatabaseCompactionRunning                               = errors.New("database compaction is running")
	ErrExistingDeltaSnapshotWrongFullSnapshotTargetMilestoneID = errors.New("existing delta ledger snapshot has wrong full snapshot target milestone ID")
)

type getMinimumTangleHistoryFunc func() iotago.MilestoneIndex

// Manager handles pruning of the database.
type Manager struct {
	// the logger used to log events.
	*logger.WrappedLogger

	storage                 *storagepkg.Storage
	syncManager             *syncmanager.SyncManager
	tangleDatabase          *database.Database
	utxoDatabase            *database.Database
	getMinimumTangleHistory getMinimumTangleHistoryFunc

	additionalPruningThreshold           iotago.MilestoneIndex
	pruningMilestonesEnabled             bool
	pruningMilestonesMaxMilestonesToKeep iotago.MilestoneIndex
	pruningSizeEnabled                   bool
	pruningSizeTargetSizeBytes           int64
	pruningSizeThresholdPercentage       float64
	pruningSizeCooldownTime              time.Duration
	pruneReceipts                        bool

	snapshotLock          syncutils.Mutex
	statusLock            syncutils.RWMutex
	isPruning             bool
	lastPruningBySizeTime time.Time

	Events *Events
}

// NewPruningManager creates a new pruning manager instance.
func NewPruningManager(
	log *logger.Logger,
	storage *storagepkg.Storage,
	syncManager *syncmanager.SyncManager,
	tangleDatabase *database.Database,
	utxoDatabase *database.Database,
	getMinimumTangleHistory getMinimumTangleHistoryFunc,
	pruningMilestonesEnabled bool,
	pruningMilestonesMaxMilestonesToKeep syncmanager.MilestoneIndexDelta,
	pruningSizeEnabled bool,
	pruningSizeTargetSizeBytes int64,
	pruningSizeThresholdPercentage float64,
	pruningSizeCooldownTime time.Duration,
	pruneReceipts bool) *Manager {

	return &Manager{
		WrappedLogger:                        logger.NewWrappedLogger(log),
		storage:                              storage,
		syncManager:                          syncManager,
		tangleDatabase:                       tangleDatabase,
		utxoDatabase:                         utxoDatabase,
		getMinimumTangleHistory:              getMinimumTangleHistory,
		additionalPruningThreshold:           AdditionalPruningThreshold,
		pruningMilestonesEnabled:             pruningMilestonesEnabled,
		pruningMilestonesMaxMilestonesToKeep: pruningMilestonesMaxMilestonesToKeep,
		pruningSizeEnabled:                   pruningSizeEnabled,
		pruningSizeTargetSizeBytes:           pruningSizeTargetSizeBytes,
		pruningSizeThresholdPercentage:       pruningSizeThresholdPercentage,
		pruningSizeCooldownTime:              pruningSizeCooldownTime,
		pruneReceipts:                        pruneReceipts,
		Events: &Events{
			PruningMilestoneIndexChanged: events.NewEvent(storagepkg.MilestoneIndexCaller),
			PruningMetricsUpdated:        events.NewEvent(MetricsCaller),
		},
	}
}

func (p *Manager) setIsPruning(value bool) {
	p.statusLock.Lock()
	p.isPruning = value
	p.storage.Events.PruningStateChanged.Trigger(value)
	p.statusLock.Unlock()
}

func (p *Manager) IsPruning() bool {
	p.statusLock.RLock()
	defer p.statusLock.RUnlock()

	return p.isPruning
}

func (p *Manager) calcTargetIndexBySize(targetSizeBytes ...int64) (iotago.MilestoneIndex, error) {

	if !p.pruningSizeEnabled && len(targetSizeBytes) == 0 {
		// pruning by size deactivated
		return 0, ErrNoPruningNeeded
	}

	if !p.tangleDatabase.CompactionSupported() || !p.utxoDatabase.CompactionSupported() {
		return 0, ErrDatabaseCompactionNotSupported
	}

	if p.tangleDatabase.CompactionRunning() || p.utxoDatabase.CompactionRunning() {
		return 0, ErrDatabaseCompactionRunning
	}

	currentTangleDatabaseSizeBytes, err := p.tangleDatabase.Size()
	if err != nil {
		return 0, err
	}

	currentUTXODatabaseSizeBytes, err := p.utxoDatabase.Size()
	if err != nil {
		return 0, err
	}

	currentDatabaseSizeBytes := currentTangleDatabaseSizeBytes + currentUTXODatabaseSizeBytes

	targetDatabaseSizeBytes := p.pruningSizeTargetSizeBytes
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

	snapshotInfo := p.storage.SnapshotInfo()
	if snapshotInfo == nil {
		return 0, common.ErrSnapshotInfoNotFound
	}

	confirmedMilestoneIndex := p.syncManager.ConfirmedMilestoneIndex()
	milestoneRange := confirmedMilestoneIndex - snapshotInfo.PruningIndex()
	prunedDatabaseSizeBytes := float64(targetDatabaseSizeBytes) * ((100.0 - p.pruningSizeThresholdPercentage) / 100.0)
	diffPercentage := prunedDatabaseSizeBytes / float64(currentDatabaseSizeBytes)
	milestoneDiff := syncmanager.MilestoneIndexDelta(math.Ceil(float64(milestoneRange) * diffPercentage))

	return confirmedMilestoneIndex - milestoneDiff, nil
}

// pruneUnreferencedBlocks prunes all unreferenced blocks from the database for the given milestone.
func (p *Manager) pruneUnreferencedBlocks(targetIndex iotago.MilestoneIndex) (blocksCountDeleted int, blocksCountChecked int) {

	blockIDsToDeleteMap := make(map[iotago.BlockID]struct{})

	// Check if block is still unreferenced
	for _, blockID := range p.storage.UnreferencedBlockIDs(targetIndex) {
		if _, exists := blockIDsToDeleteMap[blockID]; exists {
			continue
		}

		cachedBlockMeta := p.storage.CachedBlockMetadataOrNil(blockID) // meta +1
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
		blockIDsToDeleteMap[blockID] = struct{}{}
	}

	blocksCountDeleted = p.pruneBlocks(blockIDsToDeleteMap)
	p.storage.DeleteUnreferencedBlocks(targetIndex)

	return blocksCountDeleted, len(blockIDsToDeleteMap)
}

// pruneMilestone prunes the milestone metadata and the ledger diffs from the database for the given milestone.
func (p *Manager) pruneMilestone(milestoneIndex iotago.MilestoneIndex, receiptMigratedAtIndex ...iotago.MilestoneIndex) error {

	if err := p.storage.UTXOManager().PruneMilestoneIndexWithoutLocking(milestoneIndex, p.pruneReceipts, receiptMigratedAtIndex...); err != nil {
		return err
	}

	p.storage.DeleteMilestone(milestoneIndex)

	return nil
}

// pruneBlocks removes all the associated data of the given block IDs from the database.
func (p *Manager) pruneBlocks(blockIDsToDeleteMap map[iotago.BlockID]struct{}) int {

	for blockID := range blockIDsToDeleteMap {

		cachedBlockMeta := p.storage.CachedBlockMetadataOrNil(blockID) // meta +1
		if cachedBlockMeta == nil {
			continue
		}

		cachedBlockMeta.ConsumeMetadata(func(metadata *storagepkg.BlockMetadata) { // meta -1
			// Delete the reference in the parents
			for _, parent := range metadata.Parents() {
				p.storage.DeleteChild(parent, blockID)
			}

			// We don't need to iterate through the children that reference this block,
			// since we will never start the walk from this block anymore (we only walk the future cone)
			// and the references will be deleted together with the children blocks when they are pruned.
		})

		p.storage.DeleteBlock(blockID)
	}

	return len(blockIDsToDeleteMap)
}

func (p *Manager) pruneDatabase(ctx context.Context, targetIndex iotago.MilestoneIndex) (iotago.MilestoneIndex, error) {

	if err := contextutils.ReturnErrIfCtxDone(ctx, common.ErrOperationAborted); err != nil {
		// do not prune the database if the node was shut down
		return 0, err
	}

	if p.tangleDatabase.CompactionRunning() || p.utxoDatabase.CompactionRunning() {
		return 0, ErrDatabaseCompactionRunning
	}

	targetIndexMax := p.getMinimumTangleHistory()
	if targetIndex > targetIndexMax {
		targetIndex = targetIndexMax
	}

	snapshotInfo := p.storage.SnapshotInfo()
	if snapshotInfo == nil {
		return 0, errors.Wrap(common.ErrCritical, common.ErrSnapshotInfoNotFound.Error())
	}

	if snapshotInfo.PruningIndex() >= targetIndex {
		// no pruning needed
		return 0, errors.Wrapf(ErrNoPruningNeeded, "pruning index: %d, target index: %d", snapshotInfo.PruningIndex(), targetIndex)
	}

	if snapshotInfo.EntryPointIndex()+p.additionalPruningThreshold+1 > targetIndex {
		// we prune in "additionalPruningThreshold" steps to recalculate the solidEntryPoints
		return 0, errors.Wrapf(ErrNotEnoughHistory, "minimum index: %d, target index: %d", snapshotInfo.EntryPointIndex()+p.additionalPruningThreshold+1, targetIndex)
	}

	p.setIsPruning(true)
	defer p.setIsPruning(false)

	// calculate solid entry points for the new end of the tangle history
	var solidEntryPoints []*storagepkg.SolidEntryPoint
	err := dag.ForEachSolidEntryPoint(
		ctx,
		p.storage,
		targetIndex,
		// TODO
		//p.solidEntryPointCheckThresholdPast,
		15,
		func(sep *storagepkg.SolidEntryPoint) bool {
			solidEntryPoints = append(solidEntryPoints, sep)

			return true
		})
	if err != nil {
		if errors.Is(err, common.ErrOperationAborted) {
			return 0, ErrPruningAborted
		}

		return 0, err
	}

	// temporarily add the new solid entry points and keep the old ones
	p.storage.WriteLockSolidEntryPoints()
	for _, sep := range solidEntryPoints {
		p.storage.SolidEntryPointsAddWithoutLocking(sep.BlockID, sep.Index)
	}
	if err = p.storage.StoreSolidEntryPointsWithoutLocking(); err != nil {
		p.LogPanic(err)
	}
	p.storage.WriteUnlockSolidEntryPoints()

	// we have to set the new solid entry point index.
	// this way we can cleanly prune even if the pruning was aborted last time
	if err = p.storage.SetEntryPointIndex(targetIndex); err != nil {
		p.LogPanic(err)
	}

	// unreferenced blocks have to be pruned for PruningIndex as well, since this could be CMI at startup of the node
	p.pruneUnreferencedBlocks(snapshotInfo.PruningIndex())

	// Iterate through all milestones that have to be pruned
	for milestoneIndex := snapshotInfo.PruningIndex() + 1; milestoneIndex <= targetIndex; milestoneIndex++ {

		if err := contextutils.ReturnErrIfCtxDone(ctx, ErrPruningAborted); err != nil {
			// stop pruning if node was shutdown
			return 0, err
		}

		p.LogInfof("Pruning milestone (%d)...", milestoneIndex)

		timeStart := time.Now()
		blocksCountDeleted, blocksCountChecked := p.pruneUnreferencedBlocks(milestoneIndex)
		timePruneUnreferencedBlocks := time.Now()

		// get all parents of that milestone
		cachedMilestone := p.storage.CachedMilestoneByIndexOrNil(milestoneIndex) // milestone +1
		if cachedMilestone == nil {
			// Milestone not found, pruning impossible
			p.LogWarnf("Pruning milestone (%d) failed! Milestone not found!", milestoneIndex)

			continue
		}

		blockIDsToDeleteMap := make(map[iotago.BlockID]struct{})

		if err := dag.TraverseParents(
			ctx,
			p.storage,
			cachedMilestone.Milestone().Parents(),
			// traversal stops if no more blocks pass the given condition
			// Caution: condition func is not in DFS order
			func(cachedBlockMeta *storagepkg.CachedMetadata) (bool, error) { // meta +1
				defer cachedBlockMeta.Release(true) // meta -1
				// everything that was referenced by that milestone can be pruned (even blocks of older milestones)
				return true, nil
			},
			// consumer
			func(cachedBlockMeta *storagepkg.CachedMetadata) error { // meta +1
				defer cachedBlockMeta.Release(true) // meta -1
				blockIDsToDeleteMap[cachedBlockMeta.Metadata().BlockID()] = struct{}{}

				return nil
			},
			// called on missing parents
			func(parentBlockID iotago.BlockID) error { return nil },
			// called on solid entry points
			// Ignore solid entry points (snapshot milestone included)
			nil,
			// the pruning target index is also a solid entry point => traverse it anyways
			true); err != nil {
			cachedMilestone.Release(true) // milestone -1
			p.LogWarnf("Pruning milestone (%d) failed! %s", milestoneIndex, err)

			continue
		}
		timeTraverseMilestoneCone := time.Now()

		// check whether milestone contained receipt and delete it accordingly
		var migratedAtIndex []iotago.MilestoneIndex

		opts, err := cachedMilestone.Milestone().Milestone().Opts.Set()
		if err == nil && opts != nil {
			if r := opts.Receipt(); r != nil {
				migratedAtIndex = append(migratedAtIndex, r.MigratedAt)
			}
		}

		cachedMilestone.Release(true) // milestone -1

		if err := p.pruneMilestone(milestoneIndex, migratedAtIndex...); err != nil {
			p.LogWarnf("Pruning milestone (%d) failed! %s", milestoneIndex, err)
		}
		timePruneMilestone := time.Now()

		blocksCountChecked += len(blockIDsToDeleteMap)
		blocksCountDeleted += p.pruneBlocks(blockIDsToDeleteMap)
		timePruneBlocks := time.Now()

		if err = p.storage.SetPruningIndex(milestoneIndex); err != nil {
			p.LogPanic(err)
		}
		timeSetSnapshotInfo := time.Now()

		p.LogInfof("Pruning milestone (%d) took %v. Pruned %d/%d blocks. ", milestoneIndex, time.Since(timeStart).Truncate(time.Millisecond), blocksCountDeleted, blocksCountChecked)

		p.Events.PruningMilestoneIndexChanged.Trigger(milestoneIndex)
		timePruningMilestoneIndexChanged := time.Now()

		p.Events.PruningMetricsUpdated.Trigger(&Metrics{
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
	p.storage.WriteLockSolidEntryPoints()
	p.storage.ResetSolidEntryPointsWithoutLocking()
	for _, sep := range solidEntryPoints {
		p.storage.SolidEntryPointsAddWithoutLocking(sep.BlockID, sep.Index)
	}
	if err = p.storage.StoreSolidEntryPointsWithoutLocking(); err != nil {
		p.LogPanic(err)
	}
	p.storage.WriteUnlockSolidEntryPoints()

	if err := p.storage.PruneProtocolParameterMilestoneOptions(targetIndex); err != nil {
		return 0, err
	}

	return targetIndex, nil
}

func (p *Manager) PruneDatabaseByDepth(ctx context.Context, depth iotago.MilestoneIndex) (iotago.MilestoneIndex, error) {
	p.snapshotLock.Lock()
	defer p.snapshotLock.Unlock()

	confirmedMilestoneIndex := p.syncManager.ConfirmedMilestoneIndex()

	if confirmedMilestoneIndex <= depth {
		// Not enough history
		return 0, ErrNotEnoughHistory
	}

	return p.pruneDatabase(ctx, confirmedMilestoneIndex-depth)
}

func (p *Manager) PruneDatabaseByTargetIndex(ctx context.Context, targetIndex iotago.MilestoneIndex) (iotago.MilestoneIndex, error) {
	p.snapshotLock.Lock()
	defer p.snapshotLock.Unlock()

	return p.pruneDatabase(ctx, targetIndex)
}

func (p *Manager) PruneDatabaseBySize(ctx context.Context, targetSizeBytes int64) (iotago.MilestoneIndex, error) {
	p.snapshotLock.Lock()
	defer p.snapshotLock.Unlock()

	targetIndex, err := p.calcTargetIndexBySize(targetSizeBytes)
	if err != nil {
		return 0, err
	}

	return p.pruneDatabase(ctx, targetIndex)
}

// HandleNewConfirmedMilestoneEvent handles new confirmed milestone events which may trigger a snapshot creation.
func (p *Manager) HandleNewConfirmedMilestoneEvent(ctx context.Context, confirmedMilestoneIndex iotago.MilestoneIndex) {

	if !p.syncManager.IsNodeSynced() {
		// do not prune while we are not synced
		return
	}

	var targetIndex iotago.MilestoneIndex
	if p.pruningMilestonesEnabled && confirmedMilestoneIndex > p.pruningMilestonesMaxMilestonesToKeep {
		targetIndex = confirmedMilestoneIndex - p.pruningMilestonesMaxMilestonesToKeep
	}

	pruningBySize := false
	if p.pruningSizeEnabled && (p.lastPruningBySizeTime.IsZero() || time.Since(p.lastPruningBySizeTime) > p.pruningSizeCooldownTime) {
		targetIndexSize, err := p.calcTargetIndexBySize()
		if err == nil && ((targetIndex == 0) || (targetIndex < targetIndexSize)) {
			targetIndex = targetIndexSize
			pruningBySize = true
		}
	}

	if targetIndex == 0 {
		// no pruning needed
		return
	}

	if _, err := p.pruneDatabase(ctx, targetIndex); err != nil {
		p.LogDebugf("pruning aborted: %v", err)
	}

	if pruningBySize {
		p.lastPruningBySizeTime = time.Now()
	}
}
