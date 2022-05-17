package tangle

import (
	"context"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/events"
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
func (t *Tangle) CheckSolidityAndComputeWhiteFlagMutations(ctx context.Context, index milestone.Index, timestamp uint32, parents hornet.BlockIDs, previousMilestoneID iotago.MilestoneID) (*whiteflag.WhiteFlagMutations, error) {

	// check if the requested milestone index would be the next one
	if index > t.syncManager.ConfirmedMilestoneIndex()+1 {
		return nil, common.ErrNodeNotSynced
	}

	if len(parents) < 1 {
		return nil, ErrParentsNotGiven
	}

	// register all parents for message solid events
	// this has to be done, even if the parents may be solid already, to prevent race conditions
	msgSolidEventChans := make([]chan struct{}, len(parents))
	for i, parent := range parents {
		msgSolidEventChans[i] = t.RegisterBlockSolidEvent(parent)
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
				// deregister the event, because the parent is already solid (this also fires the event)
				t.DeregisterBlockSolidEvent(parent)
			}
			continue
		}

		cachedBlockMeta.ConsumeMetadata(func(metadata *storage.BlockMetadata) { // meta -1
			if !metadata.IsSolid() {
				return
			}

			// deregister the event, because the parent is already solid (this also fires the event)
			t.DeregisterBlockSolidEvent(parent)
		})
	}

	messagesMemcache := storage.NewBlocksMemcache(t.storage.CachedBlock)
	metadataMemcache := storage.NewMetadataMemcache(t.storage.CachedBlockMetadata)
	memcachedTraverserStorage := dag.NewMemcachedTraverserStorage(t.storage, metadataMemcache)

	defer func() {
		// deregister the events to free the memory
		for _, parent := range parents {
			t.DeregisterBlockSolidEvent(parent)
		}

		// all releases are forced since the cone is referenced and not needed anymore
		memcachedTraverserStorage.Cleanup(true)

		// release all messages at the end
		messagesMemcache.Cleanup(true)

		// Release all message metadata at the end
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

		for _, msgSolidEventChan := range msgSolidEventChans {
			// wait until the message is solid
			if err := events.WaitForChannelClosed(ctx, msgSolidEventChan); err != nil {
				return nil, ErrParentsNotSolid
			}
		}
	}

	parentsTraverser := dag.NewParentsTraverser(memcachedTraverserStorage)

	// at this point all parents are solid
	// compute merkle tree root
	return whiteflag.ComputeWhiteFlagMutations(
		ctx,
		t.storage.UTXOManager(),
		parentsTraverser,
		messagesMemcache.CachedBlock,
		index,
		timestamp,
		parents,
		previousMilestoneID,
		whiteflag.DefaultWhiteFlagTraversalCondition,
	)
}
