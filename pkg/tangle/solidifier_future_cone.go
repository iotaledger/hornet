package tangle

import (
	"bytes"
	"context"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/iotaledger/hive.go/syncutils"
)

type MarkBlockAsSolidFunc func(*storage.CachedMetadata)

// FutureConeSolidifier traverses the future cone of blocks and updates their solidity.
// It holds a reference to a traverser and a memcache, so that these can be reused for "gossip solidifcation".
type FutureConeSolidifier struct {
	syncutils.Mutex

	dbStorage                 *storage.Storage
	markBlockAsSolidFunc      MarkBlockAsSolidFunc
	metadataMemcache          *storage.MetadataMemcache
	memcachedTraverserStorage *dag.MemcachedTraverserStorage
}

// NewFutureConeSolidifier creates a new FutureConeSolidifier instance.
func NewFutureConeSolidifier(dbStorage *storage.Storage, markBlockAsSolidFunc MarkBlockAsSolidFunc) *FutureConeSolidifier {

	metadataMemcache := storage.NewMetadataMemcache(dbStorage.CachedBlockMetadata)
	memcachedTraverserStorage := dag.NewMemcachedTraverserStorage(dbStorage, metadataMemcache)

	return &FutureConeSolidifier{
		dbStorage:                 dbStorage,
		markBlockAsSolidFunc:      markBlockAsSolidFunc,
		metadataMemcache:          metadataMemcache,
		memcachedTraverserStorage: memcachedTraverserStorage,
	}
}

// Cleanup releases all the currently cached objects that have been traversed.
// This SHOULD be called periodically to free the caches (e.g. with every change of the latest known milestone index).
func (s *FutureConeSolidifier) Cleanup(forceRelease bool) {
	s.Lock()
	defer s.Unlock()

	s.memcachedTraverserStorage.Cleanup(true)
	s.metadataMemcache.Cleanup(true)
}

// SolidifyBlockAndFutureCone updates the solidity of the block and its future cone (blocks approving the given block).
// We keep on walking the future cone, if a block became newly solid during the walk.
func (s *FutureConeSolidifier) SolidifyBlockAndFutureCone(ctx context.Context, cachedBlockMeta *storage.CachedMetadata) error {
	s.Lock()
	defer s.Unlock()

	defer cachedBlockMeta.Release(true) // meta -1

	return solidifyFutureCone(ctx, s.memcachedTraverserStorage, s.markBlockAsSolidFunc, hornet.BlockIDs{cachedBlockMeta.Metadata().BlockID()})
}

// SolidifyFutureConesWithMetadataMemcache updates the solidity of the given blocks and their future cones (blocks approving the given blocks).
// This function doesn't use the same memcache nor traverser like the FutureConeSolidifier, but it holds the lock, so no other solidifications are done in parallel.
func (s *FutureConeSolidifier) SolidifyFutureConesWithMetadataMemcache(ctx context.Context, memcachedTraverserStorage dag.TraverserStorage, blockIDs hornet.BlockIDs) error {
	s.Lock()
	defer s.Unlock()

	return solidifyFutureCone(ctx, memcachedTraverserStorage, s.markBlockAsSolidFunc, blockIDs)
}

// solidifyFutureCone updates the solidity of the future cone (blocks approving the given blocks).
// We keep on walking the future cone, if a block became newly solid during the walk.
func solidifyFutureCone(
	ctx context.Context,
	traverserStorage dag.TraverserStorage,
	markBlockAsSolidFunc MarkBlockAsSolidFunc,
	blockIDs hornet.BlockIDs) error {

	childrenTraverser := dag.NewChildrenTraverser(traverserStorage)

	for _, blockID := range blockIDs {

		startBlockID := blockID

		if err := childrenTraverser.Traverse(
			ctx,
			blockID,
			// traversal stops if no more blocks pass the given condition
			func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
				defer cachedBlockMeta.Release(true) // meta -1

				if cachedBlockMeta.Metadata().IsSolid() && !bytes.Equal(startBlockID, cachedBlockMeta.Metadata().BlockID()) {
					// do not walk the future cone if the current block is already solid, except it was the startTx
					return false, nil
				}

				// check if current block is solid by checking the solidity of its parents
				for _, parentBlockID := range cachedBlockMeta.Metadata().Parents() {
					contains, err := traverserStorage.SolidEntryPointsContain(parentBlockID)
					if err != nil {
						return false, err
					}
					if contains {
						// Ignore solid entry points (snapshot milestone included)
						continue
					}

					cachedBlockMetaParent, err := traverserStorage.CachedBlockMetadata(parentBlockID) // meta +1
					if err != nil {
						return false, err
					}
					if cachedBlockMetaParent == nil {
						// parent is missing => block is not solid
						// do not walk the future cone if the current block is not solid
						return false, nil
					}

					if !cachedBlockMetaParent.Metadata().IsSolid() {
						// parent is not solid => block is not solid
						// do not walk the future cone if the current block is not solid
						cachedBlockMetaParent.Release(true) // meta -1
						return false, nil
					}
					cachedBlockMetaParent.Release(true) // meta -1
				}

				// mark current block as solid
				markBlockAsSolidFunc(cachedBlockMeta.Retain()) // meta pass +1

				// walk the future cone since the block got newly solid
				return true, nil
			},
			// consumer
			// no need to consume here
			nil,
			true); err != nil {
			return err
		}
	}
	return nil
}
