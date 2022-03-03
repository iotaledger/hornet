package inx

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gohornet/hornet/pkg/inx"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
)

func (s *INXServer) ReadNodeStatus(context.Context, *inx.NoParams) (*inx.NodeStatus, error) {
	index, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, err
	}
	return &inx.NodeStatus{
		LatestMilestoneIndex:    uint32(deps.SyncManager.LatestMilestoneIndex()),
		ConfirmedMilestoneIndex: uint32(deps.SyncManager.ConfirmedMilestoneIndex()),
		PruningIndex:            uint32(deps.Storage.SnapshotInfo().PruningIndex),
		LedgerIndex:             uint32(index),
	}, nil
}

func (s *INXServer) ReadMilestone(_ context.Context, req *inx.MilestoneRequest) (*inx.Milestone, error) {
	cachedMilestone := deps.Storage.CachedMilestoneOrNil(milestone.Index(req.GetMilestoneIndex()))
	if cachedMilestone == nil {
		return nil, status.Errorf(codes.NotFound, "milestone %d not found", req.GetMilestoneIndex())
	}
	defer cachedMilestone.Release(true)
	return inx.NewMilestone(cachedMilestone.Milestone()), nil
}

func (s *INXServer) ListenToLatestMilestone(_ *inx.NoParams, srv inx.INX_ListenToLatestMilestoneServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		cachedMilestone := task.Param(0).(*storage.CachedMilestone)
		defer cachedMilestone.Release(true)
		payload := inx.NewMilestone(cachedMilestone.Milestone())
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("Send error: %v", err)
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
		defer cachedMilestone.Release(true)
		payload := inx.NewMilestone(cachedMilestone.Milestone())
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("Send error: %v", err)
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
