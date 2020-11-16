package snapshot

import (
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
)

const (
	// AdditionalPruningThreshold is needed, because the messages in the getMilestoneParents call in getSolidEntryPoints
	// can reference older messages as well
	AdditionalPruningThreshold = 50
)

func setIsPruning(value bool) {
	statusLock.Lock()
	isPruning = value
	statusLock.Unlock()
}

// pruneUnreferencedMessages prunes all unreferenced messages from the database for the given milestone
func pruneUnreferencedMessages(targetIndex milestone.Index) (msgCountDeleted int, msgCountChecked int) {

	messageIDsToDeleteMap := make(map[string]struct{})

	// Check if message is still unreferenced
	for _, messageID := range deps.Storage.GetUnreferencedMessageIDs(targetIndex, true) {
		messageIDMapKey := messageID.MapKey()
		if _, exists := messageIDsToDeleteMap[messageIDMapKey]; exists {
			continue
		}

		cachedMsgMeta := deps.Storage.GetCachedMessageMetadataOrNil(messageID) // meta +1
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

	msgCountDeleted = pruneMessages(messageIDsToDeleteMap)
	deps.Storage.DeleteUnreferencedMessages(targetIndex)

	return msgCountDeleted, len(messageIDsToDeleteMap)
}

// pruneMilestone prunes the milestone metadata and the ledger diffs from the database for the given milestone
func pruneMilestone(milestoneIndex milestone.Index) error {

	err := deps.UTXO.PruneMilestoneIndex(milestoneIndex)
	if err != nil {
		return err
	}

	deps.Storage.DeleteMilestone(milestoneIndex)

	return nil
}

// pruneMessages removes all the associated data of the given message IDs from the database
func pruneMessages(messageIDsToDeleteMap map[string]struct{}) int {

	for messageIDToDelete := range messageIDsToDeleteMap {

		cachedMsg := deps.Storage.GetCachedMessageOrNil(hornet.MessageIDFromMapKey(messageIDToDelete)) // msg +1
		if cachedMsg == nil {
			continue
		}

		cachedMsg.ConsumeMessage(func(msg *storage.Message) { // msg -1
			// Delete the reference in the parents
			deps.Storage.DeleteChild(msg.GetParent1MessageID(), msg.GetMessageID())
			deps.Storage.DeleteChild(msg.GetParent2MessageID(), msg.GetMessageID())

			// delete all children of this message
			deps.Storage.DeleteChildren(msg.GetMessageID())

			indexationPayload := storage.CheckIfIndexation(msg)
			if indexationPayload != nil {
				// delete indexation if the message contains an indexation payload
				deps.Storage.DeleteIndexation(indexationPayload.Index, msg.GetMessageID())
			}

			deps.Storage.DeleteMessage(msg.GetMessageID())
		})
	}

	return len(messageIDsToDeleteMap)
}

func pruneDatabase(targetIndex milestone.Index, abortSignal <-chan struct{}) (milestone.Index, error) {

	snapshotInfo := deps.Storage.GetSnapshotInfo()
	if snapshotInfo == nil {
		log.Panic("No snapshotInfo found!")
	}

	if snapshotInfo.SnapshotIndex < SolidEntryPointCheckThresholdPast+AdditionalPruningThreshold+1 {
		// Not enough history
		return 0, errors.Wrapf(ErrNotEnoughHistory, "minimum index: %d, target index: %d", SolidEntryPointCheckThresholdPast+AdditionalPruningThreshold+1, targetIndex)
	}

	targetIndexMax := snapshotInfo.SnapshotIndex - SolidEntryPointCheckThresholdPast - AdditionalPruningThreshold - 1
	if targetIndex > targetIndexMax {
		targetIndex = targetIndexMax
	}

	if snapshotInfo.PruningIndex >= targetIndex {
		// no pruning needed
		return 0, errors.Wrapf(ErrNoPruningNeeded, "pruning index: %d, target index: %d", snapshotInfo.PruningIndex, targetIndex)
	}

	if snapshotInfo.EntryPointIndex+AdditionalPruningThreshold+1 > targetIndex {
		// we prune in "AdditionalPruningThreshold" steps to recalculate the solidEntryPoints
		return 0, errors.Wrapf(ErrNotEnoughHistory, "minimum index: %d, target index: %d", snapshotInfo.EntryPointIndex+AdditionalPruningThreshold+1, targetIndex)
	}

	setIsPruning(true)
	defer setIsPruning(false)

	// calculate solid entry points for the new end of the tangle history
	var solidEntryPoints []*solidEntryPoint
	err := forEachSolidEntryPoint(targetIndex, abortSignal, func(sep *solidEntryPoint) bool {
		solidEntryPoints = append(solidEntryPoints, sep)
		return true
	})

	deps.Storage.WriteLockSolidEntryPoints()
	deps.Storage.ResetSolidEntryPoints()
	for _, sep := range solidEntryPoints {
		deps.Storage.SolidEntryPointsAdd(sep.messageID, sep.index)
	}
	deps.Storage.StoreSolidEntryPoints()
	deps.Storage.WriteUnlockSolidEntryPoints()

	if err != nil {
		return 0, err
	}

	// we have to set the new solid entry point index.
	// this way we can cleanly prune even if the pruning was aborted last time
	snapshotInfo.EntryPointIndex = targetIndex
	deps.Storage.SetSnapshotInfo(snapshotInfo)

	// unreferenced msgs have to be pruned for PruningIndex as well, since this could be LSI at startup of the node
	pruneUnreferencedMessages(snapshotInfo.PruningIndex)

	// Iterate through all milestones that have to be pruned
	for milestoneIndex := snapshotInfo.PruningIndex + 1; milestoneIndex <= targetIndex; milestoneIndex++ {
		select {
		case <-abortSignal:
			// Stop pruning the next milestone
			return 0, ErrPruningAborted
		default:
		}

		log.Infof("Pruning milestone (%d)...", milestoneIndex)

		ts := time.Now()
		txCountDeleted, msgCountChecked := pruneUnreferencedMessages(milestoneIndex)

		cachedMs := deps.Storage.GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
		if cachedMs == nil {
			// Milestone not found, pruning impossible
			log.Warnf("Pruning milestone (%d) failed! Milestone not found!", milestoneIndex)
			continue
		}

		messageIDsToDeleteMap := make(map[string]struct{})

		err := dag.TraverseParents(deps.Storage, cachedMs.GetMilestone().MessageID,
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
				messageIDsToDeleteMap[cachedMsgMeta.GetMetadata().GetMessageID().MapKey()] = struct{}{}
				return nil
			},
			// called on missing parents
			func(parentMessageID *hornet.MessageID) error { return nil },
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

		err = pruneMilestone(milestoneIndex)
		if err != nil {
			log.Warnf("Pruning milestone (%d) failed! %s", err)
		}

		msgCountChecked += len(messageIDsToDeleteMap)
		txCountDeleted += pruneMessages(messageIDsToDeleteMap)

		snapshotInfo.PruningIndex = milestoneIndex
		deps.Storage.SetSnapshotInfo(snapshotInfo)

		log.Infof("Pruning milestone (%d) took %v. Pruned %d/%d messages. ", milestoneIndex, time.Since(ts), txCountDeleted, msgCountChecked)

		deps.Tangle.Events.PruningMilestoneIndexChanged.Trigger(milestoneIndex)
	}

	database.RunGarbageCollection()

	return targetIndex, nil
}
