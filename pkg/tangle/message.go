package tangle

import (
	"github.com/iotaledger/hornet/v2/pkg/model/milestonemanager"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	iotago "github.com/iotaledger/iota.go/v3"
)

// AddBlockToStorage adds a new block to the cache/persistence layer,
// including all additional information like metadata, children,
// unreferenced blocks and milestone entries.
// block +1.
func AddBlockToStorage(dbStorage *storage.Storage, milestoneManager *milestonemanager.MilestoneManager, block *storage.Block, latestMilestoneIndex iotago.MilestoneIndex, requested bool, forceRelease bool) (cachedBlock *storage.CachedBlock, alreadyAdded bool) {

	cachedBlock, isNew := dbStorage.StoreBlockIfAbsent(block) // block +1
	if !isNew {
		if requested && cachedBlock.Block().IsMilestone() && !dbStorage.ContainsMilestoneIndex(cachedBlock.Block().Milestone().Index) {
			// if the block was requested, was already known, but contains an unknown milestone payload, we need to re-verfiy the milestone payload.
			// (maybe caused by formerly invalid milestones e.g. because of missing COO public keys in the node config).
			if milestonePayload := milestoneManager.VerifyMilestoneBlock(block.Block()); milestonePayload != nil {
				milestoneManager.StoreMilestone(cachedBlock.Retain(), milestonePayload, requested) // block pass +1
			}
		}

		return cachedBlock, true
	}

	for _, parent := range block.Parents() {
		dbStorage.StoreChild(parent, cachedBlock.Block().BlockID()).Release(forceRelease) // child +-0
	}

	// Store only non-requested blocks, since all requested blocks are referenced by a milestone anyway
	// This is only used to delete unreferenced blocks from the database at pruning
	if !requested {
		dbStorage.StoreUnreferencedBlock(latestMilestoneIndex, cachedBlock.Block().BlockID()).Release(true) // unreferencedBlock +-0
	}

	if milestonePayload := milestoneManager.VerifyMilestoneBlock(block.Block()); milestonePayload != nil {
		milestoneManager.StoreMilestone(cachedBlock.Retain(), milestonePayload, requested) // block pass +1
	}

	return cachedBlock, false
}
