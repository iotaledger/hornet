package snapshot

import (
	"math"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/utils"
	iotago "github.com/iotaledger/iota.go/v2"
)

func (s *Snapshot) setIsPruning(value bool) {
	s.statusLock.Lock()
	s.isPruning = value
	s.storage.Events.PruningStateChanged.Trigger(value)
	s.statusLock.Unlock()
}

func (s *Snapshot) calcTargetIndexBySize(targetSizeBytes ...int64) (milestone.Index, error) {

	if !s.pruningSizeEnabled && len(targetSizeBytes) == 0 {
		// pruning by size deactivated
		return 0, ErrNoPruningNeeded
	}

	if !s.database.CompactionSupported() {
		return 0, ErrDatabaseCompactionNotSupported
	}

	if s.database.CompactionRunning() {
		return 0, ErrDatabaseCompactionRunning
	}

	currentDatabaseSizeBytes, err := s.storage.DatabaseSize()
	if err != nil {
		return 0, err
	}

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

	milestoneRange := s.storage.ConfirmedMilestoneIndex() - s.storage.SnapshotInfo().PruningIndex
	prunedDatabaseSizeBytes := float64(targetDatabaseSizeBytes) * ((100.0 - s.pruningSizeThresholdPercentage) / 100.0)
	diffPercentage := (prunedDatabaseSizeBytes / float64(currentDatabaseSizeBytes))
	milestoneDiff := milestone.Index(math.Ceil(float64(milestoneRange) * diffPercentage))

	return s.storage.ConfirmedMilestoneIndex() - milestoneDiff, nil
}

// pruneUnreferencedMessages prunes all unreferenced messages from the database for the given milestone
func (s *Snapshot) pruneUnreferencedMessages(targetIndex milestone.Index) (msgCountDeleted int, msgCountChecked int) {

	messageIDsToDeleteMap := make(map[string]struct{})

	// Check if message is still unreferenced
	for _, messageID := range s.storage.UnreferencedMessageIDs(targetIndex) {
		messageIDMapKey := messageID.ToMapKey()
		if _, exists := messageIDsToDeleteMap[messageIDMapKey]; exists {
			continue
		}

		cachedMsgMeta := s.storage.CachedMessageMetadataOrNil(messageID) // meta +1
		if cachedMsgMeta == nil {
			// message was already deleted or marked for deletion
			continue
		}

		if cachedMsgMeta.Metadata().IsReferenced() {
			// message was already referenced
			cachedMsgMeta.Release(true) // meta -1
			continue
		}

		cachedMsgMeta.Release(true) // meta -1
		messageIDsToDeleteMap[messageIDMapKey] = struct{}{}
	}

	msgCountDeleted = s.pruneMessages(messageIDsToDeleteMap)
	s.storage.DeleteUnreferencedMessages(targetIndex)

	return msgCountDeleted, len(messageIDsToDeleteMap)
}

// pruneMilestone prunes the milestone metadata and the ledger diffs from the database for the given milestone
func (s *Snapshot) pruneMilestone(milestoneIndex milestone.Index, receiptMigratedAtIndex ...uint32) error {

	if err := s.utxo.PruneMilestoneIndexWithoutLocking(milestoneIndex, s.pruneReceipts, receiptMigratedAtIndex...); err != nil {
		return err
	}

	s.storage.DeleteMilestone(milestoneIndex)

	return nil
}

// pruneMessages removes all the associated data of the given message IDs from the database
func (s *Snapshot) pruneMessages(messageIDsToDeleteMap map[string]struct{}) int {

	for messageIDToDelete := range messageIDsToDeleteMap {

		msgID := hornet.MessageIDFromMapKey(messageIDToDelete)

		cachedMsg := s.storage.CachedMessageOrNil(msgID) // msg +1
		if cachedMsg == nil {
			continue
		}

		cachedMsg.ConsumeMessage(func(msg *storage.Message) { // msg -1
			// Delete the reference in the parents
			for _, parent := range msg.Parents() {
				s.storage.DeleteChild(parent, msgID)
			}

			// We don't need to iterate through the children that reference this message,
			// since we will never start the walk from this message anymore (we only walk the future cone)
			// and the references will be deleted together with the children messages when they are pruned.

			indexationPayload := storage.CheckIfIndexation(msg)
			if indexationPayload != nil {
				// delete indexation if the message contains an indexation payload
				s.storage.DeleteIndexation(indexationPayload.Index, msgID)
			}
		})

		s.storage.DeleteMessage(msgID)
	}

	return len(messageIDsToDeleteMap)
}

func (s *Snapshot) pruneDatabase(targetIndex milestone.Index, abortSignal <-chan struct{}) (milestone.Index, error) {

	if err := utils.ReturnErrIfCtxDone(s.shutdownCtx, common.ErrOperationAborted); err != nil {
		// do not prune the database if the node was shut down
		return 0, common.ErrOperationAborted
	}

	if s.database.CompactionRunning() {
		return 0, ErrDatabaseCompactionRunning
	}

	snapshotInfo := s.storage.SnapshotInfo()
	if snapshotInfo == nil {
		s.log.Panic("No snapshotInfo found!")
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
	var solidEntryPoints []*solidEntryPoint
	err := s.forEachSolidEntryPoint(targetIndex, abortSignal, func(sep *solidEntryPoint) bool {
		solidEntryPoints = append(solidEntryPoints, sep)
		return true
	})
	if err != nil {
		return 0, err
	}

	// temporarily add the new solid entry points and keep the old ones
	s.storage.WriteLockSolidEntryPoints()
	for _, sep := range solidEntryPoints {
		s.storage.SolidEntryPointsAddWithoutLocking(sep.messageID, sep.index)
	}
	if err = s.storage.StoreSolidEntryPointsWithoutLocking(); err != nil {
		s.log.Panic(err)
	}
	s.storage.WriteUnlockSolidEntryPoints()

	// we have to set the new solid entry point index.
	// this way we can cleanly prune even if the pruning was aborted last time
	snapshotInfo.EntryPointIndex = targetIndex
	if err = s.storage.SetSnapshotInfo(snapshotInfo); err != nil {
		s.log.Panic(err)
	}

	// unreferenced msgs have to be pruned for PruningIndex as well, since this could be CMI at startup of the node
	s.pruneUnreferencedMessages(snapshotInfo.PruningIndex)

	// Iterate through all milestones that have to be pruned
	for milestoneIndex := snapshotInfo.PruningIndex + 1; milestoneIndex <= targetIndex; milestoneIndex++ {
		select {
		case <-abortSignal:
			// Stop pruning the next milestone
			return 0, ErrPruningAborted
		default:
		}

		s.log.Infof("Pruning milestone (%d)...", milestoneIndex)

		timeStart := time.Now()
		txCountDeleted, msgCountChecked := s.pruneUnreferencedMessages(milestoneIndex)
		timePruneUnreferencedMessages := time.Now()

		cachedMs := s.storage.CachedMilestoneOrNil(milestoneIndex) // milestone +1
		if cachedMs == nil {
			// Milestone not found, pruning impossible
			s.log.Warnf("Pruning milestone (%d) failed! Milestone not found!", milestoneIndex)
			continue
		}

		messageIDsToDeleteMap := make(map[string]struct{})

		err := dag.TraverseParentsOfMessage(s.storage, cachedMs.Milestone().MessageID,
			// traversal stops if no more messages pass the given condition
			// Caution: condition func is not in DFS order
			func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // msg +1
				defer cachedMsgMeta.Release(true) // msg -1
				// everything that was referenced by that milestone can be pruned (even messages of older milestones)
				return true, nil
			},
			// consumer
			func(cachedMsgMeta *storage.CachedMetadata) error { // msg +1
				defer cachedMsgMeta.Release(true) // msg -1
				messageIDsToDeleteMap[cachedMsgMeta.Metadata().MessageID().ToMapKey()] = struct{}{}
				return nil
			},
			// called on missing parents
			func(parentMessageID hornet.MessageID) error { return nil },
			// called on solid entry points
			// Ignore solid entry points (snapshot milestone included)
			nil,
			// the pruning target index is also a solid entry point => traverse it anyways
			true,
			nil)
		timeTraverseMilestoneCone := time.Now()

		cachedMs.Release(true) // milestone -1
		if err != nil {
			s.log.Warnf("Pruning milestone (%d) failed! %s", milestoneIndex, err)
			continue
		}

		// check whether milestone contained receipt and delete it accordingly
		cachedMsMsg := s.storage.MilestoneCachedMessageOrNil(milestoneIndex) // milestone msg +1
		if cachedMsMsg == nil {
			// no message for milestone persisted
			s.log.Warnf("Pruning milestone (%d) failed! Milestone message not found!", milestoneIndex)
			continue
		}

		var migratedAtIndex []uint32
		if r, ok := cachedMsMsg.Message().Milestone().Receipt.(*iotago.Receipt); ok {
			migratedAtIndex = append(migratedAtIndex, r.MigratedAt)
		}

		if err := s.pruneMilestone(milestoneIndex, migratedAtIndex...); err != nil {
			s.log.Warnf("Pruning milestone (%d) failed! %s", milestoneIndex, err)
		}
		timePruneMilestone := time.Now()

		cachedMsMsg.Release(true) // milestone msg -1

		msgCountChecked += len(messageIDsToDeleteMap)
		txCountDeleted += s.pruneMessages(messageIDsToDeleteMap)
		timePruneMessages := time.Now()

		snapshotInfo.PruningIndex = milestoneIndex
		if err = s.storage.SetSnapshotInfo(snapshotInfo); err != nil {
			s.log.Panic(err)
		}
		timeSetSnapshotInfo := time.Now()

		s.log.Infof("Pruning milestone (%d) took %v. Pruned %d/%d messages. ", milestoneIndex, time.Since(timeStart).Truncate(time.Millisecond), txCountDeleted, msgCountChecked)

		s.Events.PruningMilestoneIndexChanged.Trigger(milestoneIndex)
		timePruningMilestoneIndexChanged := time.Now()

		s.Events.PruningMetricsUpdated.Trigger(&PruningMetrics{
			DurationPruneUnreferencedMessages:    timePruneUnreferencedMessages.Sub(timeStart),
			DurationTraverseMilestoneCone:        timeTraverseMilestoneCone.Sub(timePruneUnreferencedMessages),
			DurationPruneMilestone:               timePruneMilestone.Sub(timeTraverseMilestoneCone),
			DurationPruneMessages:                timePruneMessages.Sub(timePruneMilestone),
			DurationSetSnapshotInfo:              timeSetSnapshotInfo.Sub(timePruneMessages),
			DurationPruningMilestoneIndexChanged: timePruningMilestoneIndexChanged.Sub(timeSetSnapshotInfo),
			DurationTotal:                        time.Since(timeStart),
		})
	}

	// finally set the new solid entry points and remove the old ones
	s.storage.WriteLockSolidEntryPoints()
	s.storage.ResetSolidEntryPointsWithoutLocking()
	for _, sep := range solidEntryPoints {
		s.storage.SolidEntryPointsAddWithoutLocking(sep.messageID, sep.index)
	}
	if err = s.storage.StoreSolidEntryPointsWithoutLocking(); err != nil {
		s.log.Panic(err)
	}
	s.storage.WriteUnlockSolidEntryPoints()

	s.database.RunGarbageCollection()

	return targetIndex, nil
}

func (s *Snapshot) PruneDatabaseByDepth(depth milestone.Index) (milestone.Index, error) {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	confirmedMilestoneIndex := s.storage.ConfirmedMilestoneIndex()

	if confirmedMilestoneIndex <= depth {
		// Not enough history
		return 0, ErrNotEnoughHistory
	}

	return s.pruneDatabase(confirmedMilestoneIndex-depth, nil)
}

func (s *Snapshot) PruneDatabaseByTargetIndex(targetIndex milestone.Index) (milestone.Index, error) {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	return s.pruneDatabase(targetIndex, nil)
}

func (s *Snapshot) PruneDatabaseBySize(targetSizeBytes int64) (milestone.Index, error) {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	targetIndex, err := s.calcTargetIndexBySize(targetSizeBytes)
	if err != nil {
		return 0, err
	}

	return s.pruneDatabase(targetIndex, nil)
}
