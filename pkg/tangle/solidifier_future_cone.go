package tangle

import (
	"bytes"
	"context"
	"fmt"

	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hornet/pkg/dag"
	"github.com/iotaledger/hornet/pkg/model/hornet"
	"github.com/iotaledger/hornet/pkg/model/storage"
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

	metadataMemcache := storage.NewMetadataMemcache(dbStorage.CachedMessageMetadata)
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
func (s *FutureConeSolidifier) SolidifyMessageAndFutureCone(ctx context.Context, cachedMsgMeta *storage.CachedMetadata) error {
	s.Lock()
	defer s.Unlock()

	defer cachedMsgMeta.Release(true) // meta -1

	return solidifyFutureCone(ctx, s.memcachedTraverserStorage, s.markMessageAsSolidFunc, hornet.MessageIDs{cachedMsgMeta.Metadata().MessageID()})
}

// SolidifyFutureConesWithMetadataMemcache updates the solidity of the given messages and their future cones (messages approving the given messages).
// This function doesn't use the same memcache nor traverser like the FutureConeSolidifier, but it holds the lock, so no other solidifications are done in parallel.
func (s *FutureConeSolidifier) SolidifyFutureConesWithMetadataMemcache(ctx context.Context, memcachedTraverserStorage dag.TraverserStorage, messageIDs hornet.MessageIDs) error {
	s.Lock()
	defer s.Unlock()

	return solidifyFutureCone(ctx, memcachedTraverserStorage, s.markMessageAsSolidFunc, messageIDs)
}

// SolidifyDirectChildrenWithMetadataMemcache updates the solidity of the direct children of the given messages.
// The given messages itself must already be solid, otherwise an error is returned.
// This function doesn't use the same memcache nor traverser like the FutureConeSolidifier, but it holds the lock, so no other solidifications are done in parallel.
func (s *FutureConeSolidifier) SolidifyDirectChildrenWithMetadataMemcache(ctx context.Context, memcachedTraverserStorage dag.TraverserStorage, messageIDs hornet.MessageIDs) error {
	s.Lock()
	defer s.Unlock()

	return solidifyDirectChildren(ctx, memcachedTraverserStorage, s.markMessageAsSolidFunc, messageIDs)
}

// checkMessageSolid checks if the message is solid by checking the solid state of the direct parents.
func checkMessageSolid(dbStorage dag.TraverserStorage, cachedMsgMeta *storage.CachedMetadata) (isSolid bool, newlySolid bool, err error) {
	defer cachedMsgMeta.Release(true) // meta -1

	if cachedMsgMeta.Metadata().IsSolid() {
		return true, false, nil
	}

	// check if current message is solid by checking the solidity of its parents
	for _, parentMessageID := range cachedMsgMeta.Metadata().Parents() {
		contains, err := dbStorage.SolidEntryPointsContain(parentMessageID)
		if err != nil {
			return false, false, err
		}
		if contains {
			// Ignore solid entry points (snapshot milestone included)
			continue
		}

		cachedMsgMetaParent, err := dbStorage.CachedMessageMetadata(parentMessageID) // meta +1
		if err != nil {
			return false, false, err
		}
		if cachedMsgMetaParent == nil {
			// parent is missing => message is not solid
			return false, false, nil
		}

		if !cachedMsgMetaParent.Metadata().IsSolid() {
			// parent is not solid => message is not solid
			cachedMsgMetaParent.Release(true) // meta -1

			return false, false, nil
		}
		cachedMsgMetaParent.Release(true) // meta -1
	}

	return true, true, nil
}

// solidifyFutureCone updates the solidity of the future cone (messages approving the given messages).
// We keep on walking the future cone, if a message became newly solid during the walk.
func solidifyFutureCone(
	ctx context.Context,
	traverserStorage dag.TraverserStorage,
	markMessageAsSolidFunc MarkMessageAsSolidFunc,
	messageIDs hornet.MessageIDs) error {

	childrenTraverser := dag.NewChildrenTraverser(traverserStorage)

	for _, messageID := range messageIDs {

		startMessageID := messageID

		if err := childrenTraverser.Traverse(
			ctx,
			messageID,
			// traversal stops if no more messages pass the given condition
			func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1

				isSolid, newlySolid, err := checkMessageSolid(traverserStorage, cachedMsgMeta.Retain())
				if err != nil {
					return false, err
				}

				if newlySolid {
					// mark current message as solid
					markMessageAsSolidFunc(cachedMsgMeta.Retain()) // meta pass +1
				}

				// only walk the future cone if the current message got newly solid or it is solid and it was the startTx
				return newlySolid || (isSolid && bytes.Equal(startMessageID, cachedMsgMeta.Metadata().MessageID())), nil
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

// solidifyDirectChildren updates the solidity of the future cone (messages approving the given messages).
// We only solidify the direct children of the given messageIDs.
func solidifyDirectChildren(
	ctx context.Context,
	traverserStorage dag.TraverserStorage,
	markMessageAsSolidFunc MarkMessageAsSolidFunc,
	messageIDs hornet.MessageIDs) error {

	childrenTraverser := dag.NewChildrenTraverser(traverserStorage)

	for _, messageID := range messageIDs {

		startMessageID := messageID

		if err := childrenTraverser.Traverse(
			ctx,
			messageID,
			// traversal stops if no more messages pass the given condition
			func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1

				isSolid, newlySolid, err := checkMessageSolid(traverserStorage, cachedMsgMeta.Retain())
				if err != nil {
					return false, err
				}

				if !isSolid && bytes.Equal(startMessageID, cachedMsgMeta.Metadata().MessageID()) {
					return false, fmt.Errorf("starting message for solidifyDirectChildren was not solid: %s", startMessageID.ToHex())
				}

				if newlySolid {
					// mark current message as solid
					markMessageAsSolidFunc(cachedMsgMeta.Retain()) // meta pass +1
				}

				// only walk the future cone if the current message is solid and it was the startTx
				return isSolid && bytes.Equal(startMessageID, cachedMsgMeta.Metadata().MessageID()), nil
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
