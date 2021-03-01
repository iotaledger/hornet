package snapshot

import (
	"time"

	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
)

func (s *Snapshot) setIsPruning(value bool) {
	s.statusLock.Lock()
	s.isPruning = value
	s.statusLock.Unlock()
}

// pruneUnreferencedMessages prunes all unreferenced messages from the database for the given milestone
func (s *Snapshot) pruneUnreferencedMessages(targetIndex milestone.Index) (msgCountDeleted int, msgCountChecked int) {

	messageIDsToDeleteMap := make(map[string]struct{})

	// Check if message is still unreferenced
	for _, messageID := range s.storage.GetUnreferencedMessageIDs(targetIndex, true) {
		messageIDMapKey := messageID.ToMapKey()
		if _, exists := messageIDsToDeleteMap[messageIDMapKey]; exists {
			continue
		}

		cachedMsgMeta := s.storage.GetCachedMessageMetadataOrNil(messageID) // meta +1
		if cachedMsgMeta == nil {
			// message was already deleted or marked for deletion
			continue
		}

		if cachedMsgMeta.GetMetadata().IsReferenced() {
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

	if err := s.utxo.PruneMilestoneIndex(milestoneIndex, receiptMigratedAtIndex...); err != nil {
		return err
	}

	s.storage.DeleteMilestone(milestoneIndex)

	return nil
}

// pruneMessages removes all the associated data of the given message IDs from the database
func (s *Snapshot) pruneMessages(messageIDsToDeleteMap map[string]struct{}) int {

	for messageIDToDelete := range messageIDsToDeleteMap {

		cachedMsg := s.storage.GetCachedMessageOrNil(hornet.MessageIDFromMapKey(messageIDToDelete)) // msg +1
		if cachedMsg == nil {
			continue
		}

		cachedMsg.ConsumeMessage(func(msg *storage.Message) { // msg -1
			// Delete the reference in the parents
			for _, parent := range msg.GetParents() {
				s.storage.DeleteChild(parent, msg.GetMessageID())
			}

			// delete all children of this message
			s.storage.DeleteChildren(msg.GetMessageID())

			indexationPayload := storage.CheckIfIndexation(msg)
			if indexationPayload != nil {
				// delete indexation if the message contains an indexation payload
				s.storage.DeleteIndexation(indexationPayload.Index, msg.GetMessageID())
			}

			s.storage.DeleteMessage(msg.GetMessageID())
		})
	}

	return len(messageIDsToDeleteMap)
}

func (s *Snapshot) pruneDatabase(targetIndex milestone.Index, abortSignal <-chan struct{}) (milestone.Index, error) {

	snapshotInfo := s.storage.GetSnapshotInfo()
	if snapshotInfo == nil {
		s.log.Panic("No snapshotInfo found!")
	}

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

	s.storage.WriteLockSolidEntryPoints()
	s.storage.ResetSolidEntryPoints()
	for _, sep := range solidEntryPoints {
		s.storage.SolidEntryPointsAdd(sep.messageID, sep.index)
	}
	s.storage.StoreSolidEntryPoints()
	s.storage.WriteUnlockSolidEntryPoints()

	if err != nil {
		return 0, err
	}

	// we have to set the new solid entry point index.
	// this way we can cleanly prune even if the pruning was aborted last time
	snapshotInfo.EntryPointIndex = targetIndex
	s.storage.SetSnapshotInfo(snapshotInfo)

	// unreferenced msgs have to be pruned for PruningIndex as well, since this could be LSI at startup of the node
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

		ts := time.Now()
		txCountDeleted, msgCountChecked := s.pruneUnreferencedMessages(milestoneIndex)

		cachedMs := s.storage.GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
		if cachedMs == nil {
			// Milestone not found, pruning impossible
			s.log.Warnf("Pruning milestone (%d) failed! Milestone not found!", milestoneIndex)
			continue
		}

		messageIDsToDeleteMap := make(map[string]struct{})

		err := dag.TraverseParentsOfMessage(s.storage, cachedMs.GetMilestone().MessageID,
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
				messageIDsToDeleteMap[cachedMsgMeta.GetMetadata().GetMessageID().ToMapKey()] = struct{}{}
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

		cachedMs.Release(true) // milestone -1
		if err != nil {
			s.log.Warnf("Pruning milestone (%d) failed! Error: %v", milestoneIndex, err)
			continue
		}

		// check whether milestone contained receipt and delete it accordingly
		cachedMsMsg := s.storage.GetMilestoneCachedMessageOrNil(milestoneIndex) // milestone msg +1
		if cachedMsMsg == nil {
			// no message for milestone persisted
			s.log.Warnf("Pruning milestone (%d) failed! Milestone message not found!", milestoneIndex)
			continue
		}

		var migratedAtIndex []uint32
		if r, ok := cachedMsMsg.GetMessage().GetMilestone().Receipt.(*iotago.Receipt); ok {
			migratedAtIndex = append(migratedAtIndex, r.MigratedAt)
		}

		if err := s.pruneMilestone(milestoneIndex, migratedAtIndex...); err != nil {
			s.log.Warnf("Pruning milestone (%d) failed! %s", milestoneIndex, err)
		}

		cachedMsMsg.Release(true) // milestone msg -1

		msgCountChecked += len(messageIDsToDeleteMap)
		txCountDeleted += s.pruneMessages(messageIDsToDeleteMap)

		snapshotInfo.PruningIndex = milestoneIndex
		s.storage.SetSnapshotInfo(snapshotInfo)

		s.log.Infof("Pruning milestone (%d) took %v. Pruned %d/%d messages. ", milestoneIndex, time.Since(ts), txCountDeleted, msgCountChecked)

		s.tangle.Events.PruningMilestoneIndexChanged.Trigger(milestoneIndex)
	}

	database.RunGarbageCollection()

	return targetIndex, nil
}

func (s *Snapshot) PruneDatabaseByDepth(depth milestone.Index) (milestone.Index, error) {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	solidMilestoneIndex := s.storage.GetSolidMilestoneIndex()

	if solidMilestoneIndex <= depth {
		// Not enough history
		return 0, ErrNotEnoughHistory
	}

	return s.pruneDatabase(solidMilestoneIndex-depth, nil)
}

func (s *Snapshot) PruneDatabaseByTargetIndex(targetIndex milestone.Index) (milestone.Index, error) {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	return s.pruneDatabase(targetIndex, nil)
}
