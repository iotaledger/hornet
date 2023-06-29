package tangle

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/runtime/valuenotifier"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	ErrParentsNotGiven = errors.New("no parents given")
	ErrParentsNotSolid = errors.New("parents not solid")
)

// CheckSolidityAndComputeWhiteFlagMutations waits until all given parents are solid, an then calculates the white flag mutations
// with given milestone index, timestamp and previousMilestoneID.
// Attention: this call puts missing parents of the cone as undiscardable requests into the request queue.
// Therefore the caller needs to be trustful (e.g. coordinator plugin).
func (t *Tangle) CheckSolidityAndComputeWhiteFlagMutations(ctx context.Context, index iotago.MilestoneIndex, timestamp uint32, parents iotago.BlockIDs, previousMilestoneID iotago.MilestoneID) (*whiteflag.WhiteFlagMutations, error) {

	snapshotInfo := t.storage.SnapshotInfo()
	if snapshotInfo == nil {
		return nil, errors.Wrap(common.ErrCritical, common.ErrSnapshotInfoNotFound.Error())
	}

	// check if the requested milestone index would be the next one
	if index != t.syncManager.ConfirmedMilestoneIndex()+1 {
		return nil, common.ErrNodeNotSynced
	}

	if len(parents) < 1 {
		return nil, ErrParentsNotGiven
	}

	// register all parents for block solid events
	// this has to be done, even if the parents may be solid already, to prevent race conditions
	blockSolidEventListeners := make(map[iotago.BlockID]*valuenotifier.Listener, len(parents))
	for _, parent := range parents {
		blockSolidEventListeners[parent] = t.BlockSolidListener(parent)
	}

	// check all parents for solidity
	for _, parent := range parents {
		cachedBlockMeta := t.storage.CachedBlockMetadataOrNil(parent)
		if cachedBlockMeta == nil {
			contains, err := t.storage.SolidEntryPointsContain(parent)
			if err != nil {
				return nil, err
			}
			if contains {
				// notify the listener manually, because the parent is already solid.
				t.blockSolidNotifier.Notify(parent)
			}

			continue
		}

		cachedBlockMeta.ConsumeMetadata(func(metadata *storage.BlockMetadata) { // meta -1
			if !metadata.IsSolid() {
				return
			}

			// notify the listener manually, because the parent is already solid.
			t.blockSolidNotifier.Notify(parent)
		})
	}

	blocksMemcache := storage.NewBlocksMemcache(t.storage.CachedBlock)
	metadataMemcache := storage.NewMetadataMemcache(t.storage.CachedBlockMetadata)
	memcachedTraverserStorage := dag.NewMemcachedTraverserStorage(t.storage, metadataMemcache)

	defer func() {
		// deregister the events to free the memory
		for _, listener := range blockSolidEventListeners {
			listener.Deregister()
		}

		// all releases are forced since the cone is referenced and not needed anymore
		memcachedTraverserStorage.Cleanup(true)

		// release all blocks at the end
		blocksMemcache.Cleanup(true)

		// Release all block metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	// check if all requested parents are solid
	solid, aborted := t.SolidQueueCheck(ctx,
		memcachedTraverserStorage,
		index,
		parents)
	if aborted {
		return nil, common.ErrOperationAborted
	}

	if !solid {
		// wait for at most "ComputeWhiteFlagTimeout" for the parents to become solid
		ctx, cancel := context.WithTimeout(ctx, t.whiteFlagParentsSolidTimeout)
		defer cancel()

		for _, blockSolidEventListener := range blockSolidEventListeners {
			// wait until the block is solid
			if err := blockSolidEventListener.Wait(ctx); err != nil {
				return nil, ErrParentsNotSolid
			}
		}
	}

	parentsTraverser := dag.NewParentsTraverser(memcachedTraverserStorage)

	// we need to write lock the ledger in order to ensure consistency,
	// otherwise a new milestone could be applied during computation of the merkle root.
	utxoManager := t.storage.UTXOManager()
	utxoManager.WriteLockLedger()
	defer utxoManager.WriteUnlockLedger()

	// check again if the requested milestone index would be the next one
	if index != t.syncManager.ConfirmedMilestoneIndex()+1 {
		return nil, common.ErrNodeNotSynced
	}

	// at this point all parents are solid
	// compute merkle tree root
	return whiteflag.ComputeWhiteFlagMutations(
		ctx,
		utxoManager,
		parentsTraverser,
		blocksMemcache.CachedBlock,
		index,
		timestamp,
		parents,
		previousMilestoneID,
		snapshotInfo.GenesisMilestoneIndex(),
		whiteflag.DefaultWhiteFlagTraversalCondition,
	)
}
