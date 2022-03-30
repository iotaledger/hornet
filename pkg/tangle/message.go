package tangle

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/milestonemanager"
	"github.com/gohornet/hornet/pkg/model/storage"
)

// AddMessageToStorage adds a new message to the cache/persistence layer,
// including all additional information like metadata, children,
// unreferenced messages and milestone entries.
// message +1
func AddMessageToStorage(dbStorage *storage.Storage, milestoneManager *milestonemanager.MilestoneManager, message *storage.Message, latestMilestoneIndex milestone.Index, requested bool, forceRelease bool) (cachedMsg *storage.CachedMessage, alreadyAdded bool) {

	cachedMsg, isNew := dbStorage.StoreMessageIfAbsent(message) // message +1
	if !isNew {
		if requested && cachedMsg.Message().IsMilestone() && !dbStorage.ContainsMilestone(milestone.Index(cachedMsg.Message().Milestone().Index)) {
			// if the message was requested, was already known, but contains an unknown milestone payload, we need to re-verfiy the milestone payload.
			// (maybe caused by formerly invalid milestones e.g. because of missing COO public keys in the node config).
			if ms := milestoneManager.VerifyMilestone(message); ms != nil {
				milestoneManager.StoreMilestone(cachedMsg.Retain(), ms, requested) // message pass +1
			}
		}
		return cachedMsg, true
	}

	for _, parent := range message.Parents() {
		dbStorage.StoreChild(parent, cachedMsg.Message().MessageID()).Release(forceRelease) // child +-0
	}

	// Store only non-requested messages, since all requested messages are referenced by a milestone anyway
	// This is only used to delete unreferenced messages from the database at pruning
	if !requested {
		dbStorage.StoreUnreferencedMessage(latestMilestoneIndex, cachedMsg.Message().MessageID()).Release(true) // unreferencedTx +-0
	}

	if ms := milestoneManager.VerifyMilestone(message); ms != nil {
		milestoneManager.StoreMilestone(cachedMsg.Retain(), ms, requested) // message pass +1
	}

	return cachedMsg, false
}
