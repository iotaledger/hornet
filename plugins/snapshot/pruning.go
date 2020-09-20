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

func setIsPruning(value bool) {
	statusLock.Lock()
	isPruning = value
	statusLock.Unlock()
}

// pruneUnconfirmedMessages prunes all unconfirmed messages from the database for the given milestone
func pruneUnconfirmedMessages(targetIndex milestone.Index) (msgCountDeleted int, msgCountChecked int) {

	messagesToCheckMap := make(map[string]struct{})

	// Check if message is still unconfirmed
	for _, messageID := range tangle.GetUnconfirmedMessageIDs(targetIndex, true) {
		if _, exists := messagesToCheckMap[string(messageID)]; exists {
			continue
		}

		cachedMsg := tangle.GetCachedMessageOrNil(messageID) // msg +1
		if cachedMsg == nil {
			// message was already deleted or marked for deletion
			continue
		}

		if cachedMsg.GetMetadata().IsConfirmed() {
			// message was already confirmed
			cachedMsg.Release(true) // msg -1
			continue
		}

		cachedMsg.Release(true) // msg -1
		messagesToCheckMap[string(messageID)] = struct{}{}
	}

	msgCountDeleted = pruneMessages(messagesToCheckMap)
	tangle.DeleteUnconfirmedMessages(targetIndex)

	return msgCountDeleted, len(messagesToCheckMap)
}

// pruneMilestone prunes the milestone metadata and the ledger diffs from the database for the given milestone
func pruneMilestone(milestoneIndex milestone.Index) {

	// state diffs
	/*
		if err := tangle.DeleteLedgerDiffForMilestone(milestoneIndex); err != nil {
			log.Warn(err)
		}
	*/

	tangle.DeleteMilestone(milestoneIndex)
}

// pruneMessages removes all the associated data of the given message IDs from the database
func pruneMessages(messageIDsToDelete map[string]struct{}) int {

	for messageIDToDelete := range messageIDsToDelete {

		cachedMsg := tangle.GetCachedMessageOrNil(hornet.Hash(messageIDToDelete)) // msg +1
		if cachedMsg == nil {
			continue
		}

		cachedMsg.ConsumeMessage(func(msg *tangle.Message) { // msg -1
			// Delete the reference in the parents
			tangle.DeleteChild(msg.GetParent1MessageID(), msg.GetMessageID())
			tangle.DeleteChild(msg.GetParent2MessageID(), msg.GetMessageID())

			tangle.DeleteChildren(msg.GetMessageID())
			tangle.DeleteMessage(msg.GetMessageID())
		})
	}

	return len(messageIDsToDelete)
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
	for solidEntryPointMessageID, index := range newSolidEntryPoints {
		tangle.SolidEntryPointsAdd(hornet.Hash(solidEntryPointMessageID), index)
	}
	tangle.StoreSolidEntryPoints()
	tangle.WriteUnlockSolidEntryPoints()

	// we have to set the new solid entry point index.
	// this way we can cleanly prune even if the pruning was aborted last time
	snapshotInfo.EntryPointIndex = targetIndex
	tangle.SetSnapshotInfo(snapshotInfo)

	// unconfirmed msgs have to be pruned for PruningIndex as well, since this could be LSI at startup of the node
	pruneUnconfirmedMessages(snapshotInfo.PruningIndex)

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
		txCountDeleted, msgCountChecked := pruneUnconfirmedMessages(milestoneIndex)

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
			func(cachedMsgMeta *tangle.CachedMetadata) (bool, error) { // msg +1
				defer cachedMsgMeta.Release(true) // msg -1
				// everything that was referenced by that milestone can be pruned (even messages of older milestones)
				return true, nil
			},
			// consumer
			func(cachedMsgMeta *tangle.CachedMetadata) error { // msg +1
				defer cachedMsgMeta.Release(true) // msg -1
				msgsToCheckMap[string(cachedMsgMeta.GetMetadata().GetMessageID())] = struct{}{}
				return nil
			},
			// called on missing parents
			func(parentMessageID hornet.Hash) error { return nil },
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
		txCountDeleted += pruneMessages(msgsToCheckMap)

		pruneMilestone(milestoneIndex)

		snapshotInfo.PruningIndex = milestoneIndex
		tangle.SetSnapshotInfo(snapshotInfo)

		log.Infof("Pruning milestone (%d) took %v. Pruned %d/%d messages. ", milestoneIndex, time.Since(ts), txCountDeleted, msgCountChecked)

		tanglePlugin.Events.PruningMilestoneIndexChanged.Trigger(milestoneIndex)
	}

	database.RunGarbageCollection()

	return nil
}
