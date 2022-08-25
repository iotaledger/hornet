package milestonemanager

import (
	"math"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/keymanager"
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
			ReceivedValidMilestone: events.NewEvent(storage.MilestoneWithBlockIDAndRequestedCaller),
		},
	}

	return t
}

// KeyManager returns the used key manager.
func (m *MilestoneManager) KeyManager() *keymanager.KeyManager {
	return m.keyManager
}

// FindClosestNextMilestoneIndex searches for the next known milestone in the persistence layer.
func (m *MilestoneManager) FindClosestNextMilestoneIndex(index iotago.MilestoneIndex) (iotago.MilestoneIndex, error) {
	lmi := m.syncManager.LatestMilestoneIndex()
	if lmi == 0 {
		// no milestone received yet, check the next 100 milestones as a workaround
		lmi = m.syncManager.ConfirmedMilestoneIndex() + 100
	}

	if index == math.MaxUint32 {
		// prevent overflow (2**32 -1)
		return 0, storage.ErrMilestoneNotFound
	}

	for {
		index++

		if index > lmi {
			return 0, storage.ErrMilestoneNotFound
		}

		if m.storage.ContainsMilestoneIndex(index) {
			return index, nil
		}
	}
}

// VerifyMilestoneBlock checks if the block contains a valid milestone payload.
// Returns a milestone payload if the signature is valid.
func (m *MilestoneManager) VerifyMilestoneBlock(block *iotago.Block) *iotago.Milestone {

	milestonePayload, ok := block.Payload.(*iotago.Milestone)
	if !ok {
		// not a milestone payload
		return nil
	}

	for idx, parent := range block.Parents {
		if parent != milestonePayload.Parents[idx] {
			// parents in block and payload have to be equal
			return nil
		}
	}

	if err := milestonePayload.VerifySignatures(m.milestonePublicKeyCount, m.keyManager.PublicKeysSetForMilestoneIndex(milestonePayload.Index)); err != nil {
		return nil
	}

	return milestonePayload
}

// VerifyMilestonePayload checks if milestone payload is valid.
// Attention: It does not check if the milestone payload parents match the block parents.
// Returns a milestone payload if the signature is valid.
func (m *MilestoneManager) VerifyMilestonePayload(payload iotago.Payload) *iotago.Milestone {

	milestonePayload, ok := payload.(*iotago.Milestone)
	if !ok {
		// not a milestone payload
		return nil
	}

	if err := milestonePayload.VerifySignatures(m.milestonePublicKeyCount, m.keyManager.PublicKeysSetForMilestoneIndex(milestonePayload.Index)); err != nil {
		return nil
	}

	return milestonePayload
}

// StoreMilestone stores the milestone in the storage layer and triggers the ReceivedValidMilestone event.
func (m *MilestoneManager) StoreMilestone(cachedBlock *storage.CachedBlock, milestonePayload *iotago.Milestone, requested bool) {
	defer cachedBlock.Release(true) // block -1

	// Mark every valid milestone block as milestone in the database (needed for whiteflag to find last milestone)
	cachedBlock.Metadata().SetMilestone(true)

	cachedMilestone, newlyAdded := m.storage.StoreMilestoneIfAbsent(milestonePayload) // milestone +1

	// Force release to store milestones without caching
	defer cachedMilestone.Release(true) // milestone -1

	if !newlyAdded {
		return
	}

	m.Events.ReceivedValidMilestone.Trigger(cachedBlock.Metadata().BlockID(), cachedMilestone, requested) // milestone pass +1
}
