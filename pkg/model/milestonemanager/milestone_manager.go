package milestonemanager

import (
	"time"

	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/iotaledger/hive.go/events"
	iotago "github.com/iotaledger/iota.go/v3"
)

type packageEvents struct {
	ReceivedValidMilestone *events.Event
}

// MilestoneManager is used to retrieve, verify and store milestones.
type MilestoneManager struct {
	// used to access the node storage.
	storage *storage.Storage
	// used to determine the sync status of the node.
	syncManager *syncmanager.SyncManager
	// provides public and private keys for ranges of milestone indexes.
	keyManager *keymanager.KeyManager
	// amount of public keys in a milestone.
	milestonePublicKeyCount int

	// events
	Events *packageEvents
}

func New(
	dbStorage *storage.Storage,
	syncManager *syncmanager.SyncManager,
	keyManager *keymanager.KeyManager,
	milestonePublicKeyCount int) *MilestoneManager {

	t := &MilestoneManager{
		storage:                 dbStorage,
		syncManager:             syncManager,
		keyManager:              keyManager,
		milestonePublicKeyCount: milestonePublicKeyCount,

		Events: &packageEvents{
			ReceivedValidMilestone: events.NewEvent(storage.MilestoneWithRequestedCaller),
		},
	}
	return t
}

// KeyManager returns the used key manager.
func (m *MilestoneManager) KeyManager() *keymanager.KeyManager {
	return m.keyManager
}

// FindClosestNextMilestoneOrNil searches for the next known cached milestone in the persistence layer.
// milestone +1
func (m *MilestoneManager) FindClosestNextMilestoneOrNil(index milestone.Index) *storage.CachedMilestone {
	lmi := m.syncManager.LatestMilestoneIndex()
	if lmi == 0 {
		// no milestone received yet, check the next 100 milestones as a workaround
		lmi = m.syncManager.ConfirmedMilestoneIndex() + 100
	}

	if index == 4294967295 {
		// prevent overflow (2**32 -1)
		return nil
	}

	for {
		index++

		if index > lmi {
			return nil
		}

		cachedMilestone := m.storage.CachedMilestoneOrNil(index) // milestone +1
		if cachedMilestone != nil {
			return cachedMilestone
		}
	}
}

// VerifyMilestone checks if the message contains a valid milestone payload.
// Returns a milestone payload if the signature is valid.
func (m *MilestoneManager) VerifyMilestone(message *storage.Message) *iotago.Milestone {
	ms := message.Milestone()
	if ms == nil {
		return nil
	}

	for idx, parent := range message.Message().Parents {
		if parent != ms.Parents[idx] {
			// parents in message and payload have to be equal
			return nil
		}
	}

	if err := ms.VerifySignatures(m.milestonePublicKeyCount, m.keyManager.PublicKeysSetForMilestoneIndex(milestone.Index(ms.Index))); err != nil {
		return nil
	}

	return ms
}

// StoreMilestone stores the milestone in the storage layer and triggers the ReceivedValidMilestone event.
func (m *MilestoneManager) StoreMilestone(cachedMsg *storage.CachedMessage, ms *iotago.Milestone, requested bool) {
	defer cachedMsg.Release(true) // message -1

	cachedMilestone, newlyAdded := m.storage.StoreMilestoneIfAbsent(milestone.Index(ms.Index), cachedMsg.Message().MessageID(), time.Unix(int64(ms.Timestamp), 0)) // milestone +1
	if !newlyAdded {
		return
	}

	// Force release to store milestones without caching
	defer cachedMilestone.Release(true) // milestone -1

	m.Events.ReceivedValidMilestone.Trigger(cachedMilestone, requested) // milestone pass +1
}
