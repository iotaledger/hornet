package dag

import (
	"context"
	"math"

	"github.com/pkg/errors"

	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	iotago "github.com/iotaledger/iota.go/v3"
)

// updateOutdatedConeRootIndexes updates the cone root indexes of the given blocks.
// the "outdatedBlockIDs" should be ordered from oldest to latest to avoid recursion.
func updateOutdatedConeRootIndexes(ctx context.Context, parentsTraverserStorage ParentsTraverserStorage, outdatedBlockIDs iotago.BlockIDs, cmi iotago.MilestoneIndex) error {
	for _, outdatedBlockID := range outdatedBlockIDs {
		cachedBlockMeta, err := parentsTraverserStorage.CachedBlockMetadata(outdatedBlockID)
		if err != nil {
			return err
		}
		if cachedBlockMeta == nil {
			panic(common.ErrBlockNotFound)
		}

		if _, _, err := ConeRootIndexes(ctx, parentsTraverserStorage, cachedBlockMeta, cmi); err != nil {
			return err
		}
	}

	return nil
}

// ConeRootIndexes searches the cone root indexes for a given block.
// cachedBlockMeta has to be solid, otherwise youngestConeRootIndex and oldestConeRootIndex will be 0 if a block is missing in the cone.
func ConeRootIndexes(ctx context.Context, parentsTraverserStorage ParentsTraverserStorage, cachedBlockMeta *storage.CachedMetadata, cmi iotago.MilestoneIndex) (youngestConeRootIndex iotago.MilestoneIndex, oldestConeRootIndex iotago.MilestoneIndex, err error) {
	defer cachedBlockMeta.Release(true) // meta -1

	// if the block already contains recent (calculation index matches CMI)
	// information about ycri and ocri, return that info
	ycri, ocri, ci := cachedBlockMeta.Metadata().ConeRootIndexes()
	if ci == cmi {
		return ycri, ocri, nil
	}

	youngestConeRootIndex = 0
	oldestConeRootIndex = math.MaxUint32

	updateIndexes := func(ycri iotago.MilestoneIndex, ocri iotago.MilestoneIndex) {
		if youngestConeRootIndex < ycri {
			youngestConeRootIndex = ycri
		}
		if oldestConeRootIndex > ocri {
			oldestConeRootIndex = ocri
		}
	}

	// collect all parents in the cone that are not referenced,
	// are no solid entry points and have no recent calculation index
	var outdatedBlockIDs iotago.BlockIDs

	startBlockID := cachedBlockMeta.Metadata().BlockID()

	indexesValid := true

	// traverse the parents of this block to calculate the cone root indexes for this block.
	// this walk will also collect all outdated blocks in the same cone, to update them afterwards.
	if err := TraverseParentsOfBlock(
		ctx,
		parentsTraverserStorage,
		cachedBlockMeta.Metadata().BlockID(),
		// traversal stops if no more blocks pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedBlockMeta.Release(true) // meta -1

			// first check if the block was referenced => update ycri and ocri with the confirmation index
			if referenced, at := cachedBlockMeta.Metadata().ReferencedWithIndex(); referenced {
				updateIndexes(at, at)

				return false, nil
			}

			if startBlockID == cachedBlockMeta.Metadata().BlockID() {
				// do not update indexes for the start block
				return true, nil
			}

			// if the block was not referenced yet, but already contains recent (calculation index matches CMI) information
			// about ycri and ocri, propagate that info
			ycri, ocri, ci := cachedBlockMeta.Metadata().ConeRootIndexes()
			if ci == cmi {
				updateIndexes(ycri, ocri)

				return false, nil
			}

			return true, nil
		},
		// consumer
		func(cachedBlockMeta *storage.CachedMetadata) error { // meta +1
			defer cachedBlockMeta.Release(true) // meta -1

			if startBlockID == cachedBlockMeta.Metadata().BlockID() {
				// skip the start block, so it doesn't get added to the outdatedBlockIDs
				return nil
			}

			outdatedBlockIDs = append(outdatedBlockIDs, cachedBlockMeta.Metadata().BlockID())

			return nil
		},
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		func(blockID iotago.BlockID) error {
			// if the parent is a solid entry point, use the index of the solid entry point as ORTSI
			entryPointIndex, _, err := parentsTraverserStorage.SolidEntryPointsIndex(blockID)
			if err != nil {
				return err
			}
			updateIndexes(entryPointIndex, entryPointIndex)

			return nil
		}, false); err != nil {
		if errors.Is(err, common.ErrBlockNotFound) {
			indexesValid = false
		} else if errors.Is(err, common.ErrOperationAborted) {
			// if the context was canceled, directly stop traversing
			return 0, 0, err
		} else {
			panic(err)
		}
	}

	// update the outdated cone root indexes of all blocks in the cone in order from oldest blocks to latest.
	// this is an efficient way to update the whole cone, because updating from oldest to latest will not be recursive.
	if err := updateOutdatedConeRootIndexes(ctx, parentsTraverserStorage, outdatedBlockIDs, cmi); err != nil {
		return 0, 0, err
	}

	// only set the calculated cone root indexes if all blocks in the past cone were found
	// and the oldestConeRootIndex was found.
	if !indexesValid || oldestConeRootIndex == math.MaxUint32 {
		return 0, 0, nil
	}

	// set the new cone root indexes in the metadata of the block
	cachedBlockMeta.Metadata().SetConeRootIndexes(youngestConeRootIndex, oldestConeRootIndex, cmi)

	return youngestConeRootIndex, oldestConeRootIndex, nil
}

// UpdateConeRootIndexes updates the cone root indexes of the future cone of all given blocks.
// all the blocks of the newly referenced cone already have updated cone root indexes.
// we have to walk the future cone, and update the past cone of all blocks that reference an old cone.
// as a special property, invocations of the yielded function share the same 'already traversed' set to circumvent
// walking the future cone of the same blocks multiple times.
func UpdateConeRootIndexes(ctx context.Context, traverserStorage TraverserStorage, blockIDs iotago.BlockIDs, cmi iotago.MilestoneIndex) error {
	traversed := map[iotago.BlockID]struct{}{}

	t := NewChildrenTraverser(traverserStorage)

	// we update all blocks in order from oldest to latest
	for _, blockID := range blockIDs {

		if err := t.Traverse(
			ctx,
			blockID,
			// traversal stops if no more blocks pass the given condition
			func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
				defer cachedBlockMeta.Release(true) // meta -1

				_, previouslyTraversed := traversed[cachedBlockMeta.Metadata().BlockID()]

				// only traverse this block if it was not traversed before and is solid
				return !previouslyTraversed && cachedBlockMeta.Metadata().IsSolid(), nil
			},
			// consumer
			func(cachedBlockMeta *storage.CachedMetadata) error { // meta +1
				defer cachedBlockMeta.Release(true) // meta -1
				traversed[cachedBlockMeta.Metadata().BlockID()] = struct{}{}

				// updates the cone root indexes of the outdated past cone for this block
				if _, _, err := ConeRootIndexes(ctx, traverserStorage, cachedBlockMeta.Retain(), cmi); err != nil { // meta pass +1
					return err
				}

				return nil
			}, false); err != nil {
			return err
		}
	}

	return nil
}
