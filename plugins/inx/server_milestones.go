package inx

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/workerpool"
	inx "github.com/iotaledger/inx/go"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	ComputeWhiteFlagTimeout = 2 * time.Second
)

func milestoneForCachedMilestone(ms *storage.CachedMilestone) (*inx.Milestone, error) {
	defer ms.Release(true) // milestone -1

	milestone := ms.Milestone()

	milestoneMsg := deps.Storage.CachedMessageOrNil(milestone.MessageID) // message + 1
	if milestoneMsg == nil {
		return nil, status.Errorf(codes.NotFound, "milestone message for %d not found", milestone.Index)
	}
	defer milestoneMsg.Release(true) // message -1

	milestonePayload := milestoneMsg.Message().Milestone()
	if milestone == nil {
		return nil, status.Errorf(codes.Internal, "milestone message for %d does not contain a milestone", milestone.Index)
	}
	milestoneID, err := milestonePayload.ID()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error computing milestone ID: %s", err)
	}

	bytes, err := milestonePayload.Serialize(serializer.DeSeriModeNoValidation, iotago.ZeroRentParas)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error serializing milestone payload: %s", err)
	}

	return &inx.Milestone{
		MilestoneInfo: inx.NewMilestoneInfo(*milestoneID, uint32(milestone.Index), uint32(milestone.Timestamp.Unix())),
		Milestone: &inx.RawMilestone{
			Data: bytes,
		},
	}, nil
}

func milestoneForIndex(msIndex milestone.Index) (*inx.Milestone, error) {
	cachedMilestone := deps.Storage.CachedMilestoneOrNil(msIndex) // milestone +1
	if cachedMilestone == nil {
		return nil, status.Errorf(codes.NotFound, "milestone %d not found", msIndex)
	}
	defer cachedMilestone.Release(true) // milestone -1

	return milestoneForCachedMilestone(cachedMilestone.Retain()) // milestone + 1
}

func (s *INXServer) ReadMilestone(_ context.Context, req *inx.MilestoneRequest) (*inx.Milestone, error) {
	return milestoneForIndex(milestone.Index(req.GetMilestoneIndex()))
}

func (s *INXServer) ListenToLatestMilestone(_ *inx.NoParams, srv inx.INX_ListenToLatestMilestoneServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		cachedMilestone := task.Param(0).(*storage.CachedMilestone)
		defer cachedMilestone.Release(true)                                   // milestone -1
		payload, err := milestoneForCachedMilestone(cachedMilestone.Retain()) // milestone +1
		if err != nil {
			Plugin.LogInfof("error creating milestone: %v", err)
			cancel()
			return
		}
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("send error: %v", err)
			cancel()
		}
		task.Return(nil)
	})
	closure := events.NewClosure(func(milestone *storage.CachedMilestone) {
		wp.Submit(milestone)
	})
	wp.Start()
	deps.Tangle.Events.LatestMilestoneChanged.Attach(closure)
	<-ctx.Done()
	deps.Tangle.Events.LatestMilestoneChanged.Detach(closure)
	wp.Stop()
	return ctx.Err()
}

func (s *INXServer) ListenToConfirmedMilestone(_ *inx.NoParams, srv inx.INX_ListenToConfirmedMilestoneServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		cachedMilestone := task.Param(0).(*storage.CachedMilestone)
		defer cachedMilestone.Release(true)                                   // milestone -1
		payload, err := milestoneForCachedMilestone(cachedMilestone.Retain()) // milestone +1
		if err != nil {
			Plugin.LogInfof("error creating milestone: %v", err)
			cancel()
			return
		}
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("send error: %v", err)
			cancel()
		}
		task.Return(nil)
	})
	closure := events.NewClosure(func(milestone *storage.CachedMilestone) {
		wp.Submit(milestone)
	})
	wp.Start()
	deps.Tangle.Events.ConfirmedMilestoneChanged.Attach(closure)
	<-ctx.Done()
	deps.Tangle.Events.ConfirmedMilestoneChanged.Detach(closure)
	wp.Stop()
	return ctx.Err()
}

func (s *INXServer) ComputeWhiteFlag(ctx context.Context, req *inx.WhiteFlagRequest) (*inx.WhiteFlagResponse, error) {

	requestedIndex := milestone.Index(req.GetMilestoneIndex())
	requestedTimestamp := req.GetMilestoneTimestamp()
	requestedParents := MessageIDsFromINXMessageIDs(req.GetParents())
	requestedLastMilestoneID := req.GetLastMilestoneId().Unwrap()

	// check if the requested milestone index would be the next one
	if requestedIndex > deps.SyncManager.ConfirmedMilestoneIndex()+1 {
		return nil, status.Errorf(codes.Unavailable, common.ErrNodeNotSynced.Error())
	}

	if len(requestedParents) < 1 {
		return nil, status.Errorf(codes.InvalidArgument, "no parents given")
	}

	// register all parents for message solid events
	// this has to be done, even if the parents may be solid already, to prevent race conditions
	msgSolidEventChans := make([]chan struct{}, len(requestedParents))
	for i, parent := range requestedParents {
		msgSolidEventChans[i] = deps.Tangle.RegisterMessageSolidEvent(parent)
	}

	// check all parents for solidity
	for _, parent := range requestedParents {
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
		for _, parent := range requestedParents {
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
	solid, aborted := deps.Tangle.SolidQueueCheck(Plugin.Daemon().ContextStopped(),
		memcachedTraverserStorage,
		requestedIndex,
		requestedParents)
	if aborted {
		return nil, status.Errorf(codes.Unavailable, common.ErrOperationAborted.Error())
	}

	if !solid {
		// wait for at most "ComputeWhiteFlagTimeout" for the parents to become solid
		ctx, cancel := context.WithTimeout(context.Background(), ComputeWhiteFlagTimeout)
		defer cancel()

		for _, msgSolidEventChan := range msgSolidEventChans {
			// wait until the message is solid
			if err := utils.WaitForChannelClosed(ctx, msgSolidEventChan); err != nil {
				return nil, status.Errorf(codes.Unavailable, "parents not solid")
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
		requestedIndex,
		requestedTimestamp,
		requestedParents,
		requestedLastMilestoneID,
		whiteflag.DefaultWhiteFlagTraversalCondition,
	)
	if err != nil {
		if errors.Is(err, common.ErrOperationAborted) {
			return nil, status.Errorf(codes.Unavailable, "failed to compute white flag mutations: %s", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to compute white flag mutations: %s", err)
	}

	return &inx.WhiteFlagResponse{
		MilestoneConfirmedMerkleRoot: mutations.ConfirmedMerkleRoot[:],
		MilestoneAppliedMerkleRoot:   mutations.AppliedMerkleRoot[:],
	}, nil
}
