package tangle

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/milestonemanager"
	"github.com/gohornet/hornet/pkg/model/storage"
)

// AddMessageToStorage adds a new message to the cache/persistence layer,
// including all additional information like metadata, children,
// indexation, unreferenced messages and milestone entries.
// msg +1
func AddMessageToStorage(dbStorage *storage.Storage, milestoneManager *milestonemanager.MilestoneManager, message *storage.Message, latestMilestoneIndex milestone.Index, requested bool, forceRelease bool, reapply bool) (cachedMessage *storage.CachedMessage, alreadyAdded bool) {

	cachedMessage, isNew := dbStorage.StoreMessageIfAbsent(message) // msg +1
	if !isNew && !reapply {
		return cachedMessage, true
	}

	for _, parent := range message.Parents() {
		dbStorage.StoreChild(parent, cachedMessage.Message().MessageID()).Release(forceRelease)
	}

	indexationPayload := storage.CheckIfIndexation(cachedMessage.Message())
	if indexationPayload != nil {
		// store indexation if the message contains an indexation payload
		dbStorage.StoreIndexation(indexationPayload.Index, cachedMessage.Message().MessageID()).Release(true)
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
