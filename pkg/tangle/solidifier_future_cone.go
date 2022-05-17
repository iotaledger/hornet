package tangle

import (
	"bytes"
	"context"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/iotaledger/hive.go/syncutils"
)

type MarkMessageAsSolidFunc func(*storage.CachedMetadata)

// FutureConeSolidifier traverses the future cone of messages and updates their solidity.
// It holds a reference to a traverser and a memcache, so that these can be reused for "gossip solidifcation".
type FutureConeSolidifier struct {
	syncutils.Mutex

	dbStorage                 *storage.Storage
	markMessageAsSolidFunc    MarkMessageAsSolidFunc
	metadataMemcache          *storage.MetadataMemcache
	memcachedTraverserStorage *dag.MemcachedTraverserStorage
}

// NewFutureConeSolidifier creates a new FutureConeSolidifier instance.
func NewFutureConeSolidifier(dbStorage *storage.Storage, markMessageAsSolidFunc MarkMessageAsSolidFunc) *FutureConeSolidifier {

	metadataMemcache := storage.NewMetadataMemcache(dbStorage.CachedBlockMetadata)
	memcachedTraverserStorage := dag.NewMemcachedTraverserStorage(dbStorage, metadataMemcache)

	return &FutureConeSolidifier{
		dbStorage:                 dbStorage,
		markMessageAsSolidFunc:    markMessageAsSolidFunc,
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

// SolidifyMessageAndFutureCone updates the solidity of the message and its future cone (messages approving the given message).
// We keep on walking the future cone, if a message became newly solid during the walk.
func (s *FutureConeSolidifier) SolidifyMessageAndFutureCone(ctx context.Context, cachedBlockMeta *storage.CachedMetadata) error {
	s.Lock()
	defer s.Unlock()

	defer cachedBlockMeta.Release(true) // meta -1

	return solidifyFutureCone(ctx, s.memcachedTraverserStorage, s.markMessageAsSolidFunc, hornet.BlockIDs{cachedBlockMeta.Metadata().BlockID()})
}

// SolidifyFutureConesWithMetadataMemcache updates the solidity of the given messages and their future cones (messages approving the given messages).
// This function doesn't use the same memcache nor traverser like the FutureConeSolidifier, but it holds the lock, so no other solidifications are done in parallel.
func (s *FutureConeSolidifier) SolidifyFutureConesWithMetadataMemcache(ctx context.Context, memcachedTraverserStorage dag.TraverserStorage, blockIDs hornet.BlockIDs) error {
	s.Lock()
	defer s.Unlock()

	return solidifyFutureCone(ctx, memcachedTraverserStorage, s.markMessageAsSolidFunc, blockIDs)
}

// solidifyFutureCone updates the solidity of the future cone (messages approving the given messages).
// We keep on walking the future cone, if a message became newly solid during the walk.
func solidifyFutureCone(
	ctx context.Context,
	traverserStorage dag.TraverserStorage,
	markMessageAsSolidFunc MarkMessageAsSolidFunc,
	blockIDs hornet.BlockIDs) error {

	childrenTraverser := dag.NewChildrenTraverser(traverserStorage)

	for _, blockID := range blockIDs {

		startMessageID := blockID

		if err := childrenTraverser.Traverse(
			ctx,
			blockID,
			// traversal stops if no more messages pass the given condition
			func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
				defer cachedBlockMeta.Release(true) // meta -1

				if cachedBlockMeta.Metadata().IsSolid() && !bytes.Equal(startMessageID, cachedBlockMeta.Metadata().BlockID()) {
					// do not walk the future cone if the current message is already solid, except it was the startTx
					return false, nil
				}

				// check if current message is solid by checking the solidity of its parents
				for _, parentMessageID := range cachedBlockMeta.Metadata().Parents() {
					contains, err := traverserStorage.SolidEntryPointsContain(parentMessageID)
					if err != nil {
						return false, err
					}
					if contains {
						// Ignore solid entry points (snapshot milestone included)
						continue
					}

					cachedBlockMetaParent, err := traverserStorage.CachedBlockMetadata(parentMessageID) // meta +1
					if err != nil {
						return false, err
					}
					if cachedBlockMetaParent == nil {
						// parent is missing => message is not solid
						// do not walk the future cone if the current message is not solid
						return false, nil
					}

					if !cachedBlockMetaParent.Metadata().IsSolid() {
						// parent is not solid => message is not solid
						// do not walk the future cone if the current message is not solid
						cachedBlockMetaParent.Release(true) // meta -1
						return false, nil
					}
					cachedBlockMetaParent.Release(true) // meta -1
				}

				// mark current message as solid
				markMessageAsSolidFunc(cachedBlockMeta.Retain()) // meta pass +1

				// walk the future cone since the message got newly solid
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
