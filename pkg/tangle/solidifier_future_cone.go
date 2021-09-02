package tangle

import (
	"bytes"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/iotaledger/hive.go/syncutils"
)

// FutureConeSolidifier traverses the future cone of messages and updates their solidity.
// It holds a reference to a traverser and a memcache, so that these can be reused for "gossip solidifcation".
type FutureConeSolidifier struct {
	syncutils.Mutex

	storage                *storage.Storage
	markMessageAsSolidFunc func(*storage.CachedMetadata)

	metadataMemcache  *storage.MetadataMemcache
	childrenTraverser *dag.ChildrenTraverser
}

// NewFutureConeSolidifier creates a new FutureConeSolidifier instance.
func NewFutureConeSolidifier(s *storage.Storage, markMessageAsSolidFunc func(*storage.CachedMetadata)) *FutureConeSolidifier {

	metadataMemcache := storage.NewMetadataMemcache(s)

	return &FutureConeSolidifier{
		storage:                s,
		markMessageAsSolidFunc: markMessageAsSolidFunc,
		metadataMemcache:       metadataMemcache,
		childrenTraverser:      dag.NewChildrenTraverser(s, metadataMemcache),
	}
}

// Cleanup releases all the currently cached objects that have been traversed.
// This SHOULD be called periodically to free the caches (e.g. with every change of the latest known milestone index).
func (s *FutureConeSolidifier) Cleanup(forceRelease bool) {
	s.Lock()
	defer s.Unlock()

	s.metadataMemcache.Cleanup(forceRelease)
}

// SolidifyMessageAndFutureCone updates the solidity of the message and its future cone (messages approving the given message).
// We keep on walking the future cone, if a message became newly solid during the walk.
func (s *FutureConeSolidifier) SolidifyMessageAndFutureCone(cachedMsgMeta *storage.CachedMetadata, abortSignal chan struct{}) error {
	s.Lock()
	defer s.Unlock()

	defer cachedMsgMeta.Release(true)

	return s.solidifyFutureCone(s.childrenTraverser, s.metadataMemcache, hornet.MessageIDs{cachedMsgMeta.Metadata().MessageID()}, abortSignal)
}

// SolidifyFutureConesWithMetadataMemcache updates the solidity of the given messages and their future cones (messages approving the given messages).
// This function doesn't use the same memcache nor traverser like the FutureConeSolidifier, but it holds the lock, so no other solidifications are done in parallel.
func (s *FutureConeSolidifier) SolidifyFutureConesWithMetadataMemcache(messageIDs hornet.MessageIDs, metadataMemcache *storage.MetadataMemcache, abortSignal chan struct{}) error {
	s.Lock()
	defer s.Unlock()

	// we do not cleanup the traverser to not cleanup the MetadataMemcache
	t := dag.NewChildrenTraverser(s.storage, metadataMemcache)

	return s.solidifyFutureCone(t, metadataMemcache, messageIDs, abortSignal)
}

// solidifyFutureCone updates the solidity of the future cone (messages approving the given messages).
// We keep on walking the future cone, if a message became newly solid during the walk.
// metadataMemcache has to be cleaned up outside.
func (s *FutureConeSolidifier) solidifyFutureCone(traverser *dag.ChildrenTraverser, metadataMemcache *storage.MetadataMemcache, messageIDs hornet.MessageIDs, abortSignal chan struct{}) error {

	for _, messageID := range messageIDs {

		startMessageID := messageID

		if err := traverser.Traverse(messageID,
			// traversal stops if no more messages pass the given condition
			func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1

				if cachedMsgMeta.Metadata().IsSolid() && !bytes.Equal(startMessageID, cachedMsgMeta.Metadata().MessageID()) {
					// do not walk the future cone if the current message is already solid, except it was the startTx
					return false, nil
				}

				// check if current message is solid by checking the solidity of its parents
				for _, parentMessageID := range cachedMsgMeta.Metadata().Parents() {
					if s.storage.SolidEntryPointsContain(parentMessageID) {
						// Ignore solid entry points (snapshot milestone included)
						continue
					}

					cachedParentMsgMeta := metadataMemcache.CachedMetadataOrNil(parentMessageID) // meta +1
					if cachedParentMsgMeta == nil {
						// parent is missing => message is not solid
						// do not walk the future cone if the current message is not solid
						return false, nil
					}

					if !cachedParentMsgMeta.Metadata().IsSolid() {
						// parent is not solid => message is not solid
						// do not walk the future cone if the current message is not solid
						return false, nil
					}
				}

				// mark current message as solid
				s.markMessageAsSolidFunc(cachedMsgMeta.Retain())

				// walk the future cone since the message got newly solid
				return true, nil
			},
			// consumer
			// no need to consume here
			nil,
			true,
			abortSignal); err != nil {
			return err
		}
	}
	return nil
}
