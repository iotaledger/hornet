package v2

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
)

func computeWhiteFlagMutations(c echo.Context) (*ComputeWhiteFlagMutationsResponse, error) {

	request := &ComputeWhiteFlagMutationsRequest{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid request, error: %s", err)
	}

	// check if the requested milestone index would be the next one
	if request.Index > deps.SyncManager.ConfirmedMilestoneIndex()+1 {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, common.ErrNodeNotSynced.Error())
	}

	if len(request.Parents) < 1 {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "no parents given")
	}

	parents, err := hornet.MessageIDsFromHex(request.Parents)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid parents, error: %s", err)
	}

	lastMilestoneID := iotago.MilestoneID{}
	if len(request.LastMilestoneID) > 0 {
		lastMilestoneIDBytes, err := iotago.DecodeHex(request.LastMilestoneID)
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid lastMilestoneID, error: %s", err)
		}
		if len(lastMilestoneIDBytes) != iotago.MilestoneIDLength {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid lastMilestoneID, length should be %d bytes", iotago.MilestoneIDLength)
		}
		copy(lastMilestoneID[:], lastMilestoneIDBytes)
	}

	// register all parents for message solid events
	// this has to be done, even if the parents may be solid already, to prevent race conditions
	msgSolidEventChans := make([]chan struct{}, len(parents))
	for i, parent := range parents {
		msgSolidEventChans[i] = deps.Tangle.RegisterMessageSolidEvent(parent)
	}

	// check all parents for solidity
	for _, parent := range parents {
		cachedMsgMeta := deps.Storage.CachedMessageMetadataOrNil(parent)
		if cachedMsgMeta == nil {
			contains, err := deps.Storage.SolidEntryPointsContain(parent)
			if err != nil {
				return nil, err
			}
			if contains {
				// deregister the event, because the parent is already solid (this also fires the event)
				deps.Tangle.DeregisterMessageSolidEvent(parent)
			}
			continue
		}

		cachedMsgMeta.ConsumeMetadata(func(metadata *storage.MessageMetadata) { // meta -1
			if !metadata.IsSolid() {
				return
			}

			// deregister the event, because the parent is already solid (this also fires the event)
			deps.Tangle.DeregisterMessageSolidEvent(parent)
		})
	}

	messagesMemcache := storage.NewMessagesMemcache(deps.Storage.CachedMessage)
	metadataMemcache := storage.NewMetadataMemcache(deps.Storage.CachedMessageMetadata)
	memcachedTraverserStorage := dag.NewMemcachedTraverserStorage(deps.Storage, metadataMemcache)

	defer func() {
		// deregister the events to free the memory
		for _, parent := range parents {
			deps.Tangle.DeregisterMessageSolidEvent(parent)
		}

		// all releases are forced since the cone is referenced and not needed anymore
		memcachedTraverserStorage.Cleanup(true)

		// release all messages at the end
		messagesMemcache.Cleanup(true)

		// Release all message metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	// check if all requested parents are solid
	solid, aborted := deps.Tangle.SolidQueueCheck(Plugin.Daemon().ContextStopped(), memcachedTraverserStorage, request.Index, parents)
	if aborted {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, common.ErrOperationAborted.Error())
	}

	if !solid {
		// wait for at most "whiteFlagParentsSolidTimeout" for the parents to become solid
		ctx, cancel := context.WithTimeout(context.Background(), deps.WhiteFlagParentsSolidTimeout)
		defer cancel()

		for _, msgSolidEventChan := range msgSolidEventChans {
			// wait until the message is solid
			if err := utils.WaitForChannelClosed(ctx, msgSolidEventChan); err != nil {
				return nil, errors.WithMessage(echo.ErrServiceUnavailable, "parents not solid")
			}
		}
	}

	parentsTraverser := dag.NewParentsTraverser(memcachedTraverserStorage)

	// at this point all parents are solid
	// compute merkle tree root
	mutations, err := whiteflag.ComputeWhiteFlagMutations(
		Plugin.Daemon().ContextStopped(),
		deps.Storage.UTXOManager(),
		parentsTraverser,
		messagesMemcache.CachedMessage,
		deps.NetworkID,
		request.Index,
		request.Timestamp,
		parents,
		lastMilestoneID,
		whiteflag.DefaultWhiteFlagTraversalCondition,
	)
	if err != nil {
		if errors.Is(err, common.ErrOperationAborted) {
			return nil, errors.WithMessagef(echo.ErrServiceUnavailable, "failed to compute white flag mutations: %s", err)
		}
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "failed to compute white flag mutations: %s", err)
	}

	return &ComputeWhiteFlagMutationsResponse{
		ConfirmedMerkleRoot: iotago.EncodeHex(mutations.ConfirmedMerkleRoot[:]),
		AppliedMerkleRoot:   iotago.EncodeHex(mutations.AppliedMerkleRoot[:]),
	}, nil
}
