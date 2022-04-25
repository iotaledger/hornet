package inx

import (
	"context"

	"github.com/pkg/errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	inx "github.com/iotaledger/inx/go"
)

func milestoneForCachedMilestone(ms *storage.CachedMilestone) (*inx.Milestone, error) {
	defer ms.Release(true) // milestone -1

	return &inx.Milestone{
		MilestoneInfo: inx.NewMilestoneInfo(
			ms.Milestone().MilestoneID(),
			uint32(ms.Milestone().Index()),
			ms.Milestone().TimestampUnix()),
		Milestone: &inx.RawMilestone{
			Data: ms.Milestone().Data(),
		},
	}, nil
}

func milestoneForIndex(msIndex milestone.Index) (*inx.Milestone, error) {
	cachedMilestone := deps.Storage.CachedMilestoneByIndexOrNil(msIndex) // milestone +1
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
		defer cachedMilestone.Release(true) // milestone -1

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
	requestedPreviousMilestoneID := req.GetPreviousMilestoneId().Unwrap()

	mutations, err := deps.Tangle.CheckSolidityAndComputeWhiteFlagMutations(ctx, requestedIndex, requestedTimestamp, requestedParents, requestedPreviousMilestoneID)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrNodeNotSynced):
			return nil, status.Errorf(codes.Unavailable, "failed to compute white flag mutations: %s", err.Error())
		case errors.Is(err, tangle.ErrParentsNotGiven):
			return nil, status.Errorf(codes.InvalidArgument, "failed to compute white flag mutations: %s", err.Error())
		case errors.Is(err, tangle.ErrParentsNotSolid):
			return nil, status.Errorf(codes.Unavailable, "failed to compute white flag mutations: %s", err.Error())
		case errors.Is(err, common.ErrOperationAborted):
			return nil, status.Errorf(codes.Unavailable, "failed to compute white flag mutations: %s", err.Error())
		default:
			return nil, status.Errorf(codes.Internal, "failed to compute white flag mutations: %s", err)
		}
	}

	return &inx.WhiteFlagResponse{
		MilestoneConfirmedMerkleRoot: mutations.ConfirmedMerkleRoot[:],
		MilestoneAppliedMerkleRoot:   mutations.AppliedMerkleRoot[:],
	}, nil
}
