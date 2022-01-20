package tangle

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/milestonemanager"
	"github.com/gohornet/hornet/pkg/model/storage"
)

// AddMessageToStorage adds a new message to the cache/persistence layer,
// including all additional information like metadata, children,
// unreferenced messages and milestone entries.
// msg +1
func AddMessageToStorage(dbStorage *storage.Storage, milestoneManager *milestonemanager.MilestoneManager, message *storage.Message, latestMilestoneIndex milestone.Index, requested bool, forceRelease bool) (cachedMessage *storage.CachedMessage, alreadyAdded bool) {

	cachedMessage, isNew := dbStorage.StoreMessageIfAbsent(message) // msg +1
	if !isNew {
		if requested && cachedMessage.Message().IsMilestone() && !dbStorage.ContainsMilestone(milestone.Index(cachedMessage.Message().Milestone().Index)) {
			// if the message was requested, was already known, but contains an unknown milestone payload, we need to re-verfiy the milestone payload.
			// (maybe caused by formerly invalid milestones e.g. because of missing COO public keys in the node config).
			if ms := milestoneManager.VerifyMilestone(message); ms != nil {
				milestoneManager.StoreMilestone(cachedMessage.Retain(), ms, requested)
			}
		}
		return cachedMessage, true
	}

	for _, parent := range message.Parents() {
		dbStorage.StoreChild(parent, cachedMessage.Message().MessageID()).Release(forceRelease)
	}

	// Store only non-requested messages, since all requested messages are referenced by a milestone anyway
	// This is only used to delete unreferenced messages from the database at pruning
	if !requested {
		dbStorage.StoreUnreferencedMessage(latestMilestoneIndex, cachedMessage.Message().MessageID()).Release(true)
	}

	if ms := milestoneManager.VerifyMilestone(message); ms != nil {
		milestoneManager.StoreMilestone(cachedMessage.Retain(), ms, requested)
	}

	return cachedMessage, false
}
