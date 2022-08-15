package dag

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/contextutils"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	iotago "github.com/iotaledger/iota.go/v3"
)

// ForEachSolidEntryPoint calculates the solid entry points for the given target index.
func ForEachSolidEntryPoint(
	ctx context.Context,
	dbStorage *storage.Storage,
	targetIndex iotago.MilestoneIndex,
	solidEntryPointCheckThresholdPast iotago.MilestoneIndex,
	solidEntryPointConsumer func(sep *storage.SolidEntryPoint) bool) error {

	solidEntryPoints := make(map[iotago.BlockID]iotago.MilestoneIndex)

	metadataMemcache := storage.NewMetadataMemcache(dbStorage.CachedBlockMetadata)
	memcachedParentsTraverserStorage := NewMemcachedParentsTraverserStorage(dbStorage, metadataMemcache)
	memcachedChildrenTraverserStorage := NewMemcachedChildrenTraverserStorage(dbStorage, metadataMemcache)

	defer func() {
		// all releases are forced since the cone is referenced and not needed anymore
		memcachedParentsTraverserStorage.Cleanup(true)
		memcachedChildrenTraverserStorage.Cleanup(true)

		// Release all block metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	// we share the same traverser for all milestones, so we don't cleanup the cachedBlocks in between.
	parentsTraverser := NewParentsTraverser(memcachedParentsTraverserStorage)

	// isSolidEntryPoint checks whether any direct child of the given block was referenced by a milestone which is above the target milestone.
	isSolidEntryPoint := func(blockID iotago.BlockID, targetIndex iotago.MilestoneIndex) (bool, error) {
		childBlockIDs, err := memcachedChildrenTraverserStorage.ChildrenBlockIDs(blockID)
		if err != nil {
			return false, err
		}

		for _, childBlockID := range childBlockIDs {
			cachedBlockMeta, err := memcachedChildrenTraverserStorage.CachedBlockMetadata(childBlockID) // meta +1
			if err != nil {
				return false, err
			}

			if cachedBlockMeta == nil {
				// Ignore this block since it doesn't exist anymore
				continue
			}

			if referenced, at := cachedBlockMeta.Metadata().ReferencedWithIndex(); referenced && (at > targetIndex) {
				// referenced by a later milestone than targetIndex => solidEntryPoint
				cachedBlockMeta.Release(true) // meta -1

				return true, nil
			}
			cachedBlockMeta.Release(true) // meta -1
		}

		return false, nil
	}

	// Iterate from a reasonable old milestone to the target index to check for solid entry points
	for milestoneIndex := targetIndex - solidEntryPointCheckThresholdPast; milestoneIndex <= targetIndex; milestoneIndex++ {

		if err := contextutils.ReturnErrIfCtxDone(ctx, common.ErrOperationAborted); err != nil {
			// stop solid entry point calculation if node was shutdown
			return err
		}

		// get all parents of that milestone
		milestoneParents, err := dbStorage.MilestoneParentsByIndex(milestoneIndex)
		if err != nil {
			return errors.Wrapf(common.ErrCritical, "milestone (%d) not found", milestoneIndex)
		}

		// traverse the milestone and collect all blocks that were referenced by this milestone or newer
		if err := parentsTraverser.Traverse(
			ctx,
			milestoneParents,
			// traversal stops if no more blocks pass the given condition
			// Caution: condition func is not in DFS order
			func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
				defer cachedBlockMeta.Release(true) // meta -1

				// collect all blocks that were referenced by that milestone or newer
				referenced, at := cachedBlockMeta.Metadata().ReferencedWithIndex()

				return referenced && at >= milestoneIndex, nil
			},
			// consumer
			func(cachedBlockMeta *storage.CachedMetadata) error { // meta +1
				defer cachedBlockMeta.Release(true) // meta -1

				if err := contextutils.ReturnErrIfCtxDone(ctx, common.ErrOperationAborted); err != nil {
					// stop solid entry point calculation if node was shutdown
					return err
				}

				blockID := cachedBlockMeta.Metadata().BlockID()

				isEntryPoint, err := isSolidEntryPoint(blockID, targetIndex)
				if err != nil {
					return err
				}

				if !isEntryPoint {
					return nil
				}

				referenced, at := cachedBlockMeta.Metadata().ReferencedWithIndex()
				if !referenced {
					return errors.Wrapf(common.ErrCritical, "solid entry point (%v) not referenced", blockID.ToHex())
				}

				if _, exists := solidEntryPoints[blockID]; !exists {
					solidEntryPoints[blockID] = at
					if !solidEntryPointConsumer(&storage.SolidEntryPoint{BlockID: blockID, Index: at}) {
						return common.ErrOperationAborted
					}
				}

				return nil
			},
			// called on missing parents
			// return error on missing parents
			nil,
			// called on solid entry points
			// Ignore solid entry points (snapshot milestone included)
			nil,
			// the pruning target index is also a solid entry point => traverse it anyways
			true); err != nil {
			if errors.Is(err, common.ErrOperationAborted) {
				return common.ErrOperationAborted
			}
		}
	}

	return nil
}
