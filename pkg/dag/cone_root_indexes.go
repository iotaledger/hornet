package dag

import (
	"bytes"
	"math"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
)

// updateOutdatedConeRootIndexes updates the cone root indexes of the given messages.
// the "outdatedMessageIDs" should be ordered from oldest to latest to avoid recursion.
func updateOutdatedConeRootIndexes(s *storage.Storage, outdatedMessageIDs hornet.MessageIDs, cmi milestone.Index) {
	for _, outdatedMessageID := range outdatedMessageIDs {
		cachedMsgMeta := s.GetCachedMessageMetadataOrNil(outdatedMessageID)
		if cachedMsgMeta == nil {
			panic(common.ErrMessageNotFound)
		}
		GetConeRootIndexes(s, cachedMsgMeta, cmi)
	}
}

// GetConeRootIndexes searches the cone root indexes for a given message.
// cachedMsgMeta has to be solid, otherwise youngestConeRootIndex and oldestConeRootIndex will be 0 if a message is missing in the cone.
func GetConeRootIndexes(s *storage.Storage, cachedMsgMeta *storage.CachedMetadata, cmi milestone.Index) (youngestConeRootIndex milestone.Index, oldestConeRootIndex milestone.Index) {
	defer cachedMsgMeta.Release(true) // meta -1

	// if the msg already contains recent (calculation index matches CMI)
	// information about ycri and ocri, return that info
	ycri, ocri, ci := cachedMsgMeta.GetMetadata().GetConeRootIndexes()
	if ci == cmi {
		return ycri, ocri
	}

	youngestConeRootIndex = 0
	oldestConeRootIndex = math.MaxUint32

	updateIndexes := func(ycri milestone.Index, ocri milestone.Index) {
		if youngestConeRootIndex < ycri {
			youngestConeRootIndex = ycri
		}
		if oldestConeRootIndex > ocri {
			oldestConeRootIndex = ocri
		}
	}

	// collect all parents in the cone that are not referenced,
	// are no solid entry points and have no recent calculation index
	var outdatedMessageIDs hornet.MessageIDs

	startMessageID := cachedMsgMeta.GetMetadata().GetMessageID()

	indexesValid := true

	// traverse the parents of this message to calculate the cone root indexes for this message.
	// this walk will also collect all outdated messages in the same cone, to update them afterwards.
	if err := TraverseParentsOfMessage(s, cachedMsgMeta.GetMetadata().GetMessageID(),
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMetadata *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedMetadata.Release(true) // meta -1

			// first check if the msg was referenced => update ycri and ocri with the confirmation index
			if referenced, at := cachedMetadata.GetMetadata().GetReferenced(); referenced {
				updateIndexes(at, at)
				return false, nil
			}

			if bytes.Equal(startMessageID, cachedMetadata.GetMetadata().GetMessageID()) {
				// do not update indexes for the start message
				return true, nil
			}

			// if the msg was not referenced yet, but already contains recent (calculation index matches CMI) information
			// about ycri and ocri, propagate that info
			ycri, ocri, ci := cachedMetadata.GetMetadata().GetConeRootIndexes()
			if ci == cmi {
				updateIndexes(ycri, ocri)
				return false, nil
			}

			return true, nil
		},
		// consumer
		func(cachedMetadata *storage.CachedMetadata) error { // meta +1
			defer cachedMetadata.Release(true) // meta -1

			if bytes.Equal(startMessageID, cachedMetadata.GetMetadata().GetMessageID()) {
				// skip the start message, so it doesn't get added to the outdatedMessageIDs
				return nil
			}

			outdatedMessageIDs = append(outdatedMessageIDs, cachedMetadata.GetMetadata().GetMessageID())
			return nil
		},
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		func(messageID hornet.MessageID) {
			// if the parent is a solid entry point, use the index of the solid entry point as ORTSI
			entryPointIndex, _ := s.SolidEntryPointsIndex(messageID)
			updateIndexes(entryPointIndex, entryPointIndex)
		}, false, nil); err != nil {
		if err == common.ErrMessageNotFound {
			indexesValid = false
		} else {
			panic(err)
		}
	}

	// update the outdated cone root indexes of all messages in the cone in order from oldest msgs to latest.
	// this is an efficient way to update the whole cone, because updating from oldest to latest will not be recursive.
	updateOutdatedConeRootIndexes(s, outdatedMessageIDs, cmi)

	// only set the calculated cone root indexes if all messages in the past cone were found
	// and the oldestConeRootIndex was found.
	if !indexesValid || oldestConeRootIndex == math.MaxUint32 {
		return 0, 0
	}

	// set the new cone root indexes in the metadata of the message
	cachedMsgMeta.GetMetadata().SetConeRootIndexes(youngestConeRootIndex, oldestConeRootIndex, cmi)

	return youngestConeRootIndex, oldestConeRootIndex
}

// UpdateConeRootIndexes updates the cone root indexes of the future cone of all given messages.
// all the messages of the newly referenced cone already have updated cone root indexes.
// we have to walk the future cone, and update the past cone of all messages that reference an old cone.
// as a special property, invocations of the yielded function share the same 'already traversed' set to circumvent
// walking the future cone of the same messages multiple times.
func UpdateConeRootIndexes(s *storage.Storage, messageIDs hornet.MessageIDs, cmi milestone.Index, iteratorOptions ...storage.IteratorOption) {
	traversed := map[string]struct{}{}

	// we update all messages in order from oldest to latest
	for _, messageID := range messageIDs {

		if err := TraverseChildren(s, messageID,
			// traversal stops if no more messages pass the given condition
			func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1

				_, previouslyTraversed := traversed[cachedMsgMeta.GetMetadata().GetMessageID().ToMapKey()]

				// only traverse this message if it was not traversed before and is solid
				return !previouslyTraversed && cachedMsgMeta.GetMetadata().IsSolid(), nil
			},
			// consumer
			func(cachedMsgMeta *storage.CachedMetadata) error { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1
				traversed[cachedMsgMeta.GetMetadata().GetMessageID().ToMapKey()] = struct{}{}

				// updates the cone root indexes of the outdated past cone for this message
				GetConeRootIndexes(s, cachedMsgMeta.Retain(), cmi) // meta pass +1

				return nil
			}, false, nil, iteratorOptions...); err != nil {
			panic(err)
		}
	}
}
