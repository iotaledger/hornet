package inx

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/workerpool"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	inx "github.com/iotaledger/inx/go"
	iotago "github.com/iotaledger/iota.go/v3"
)

// milestone +1.
func cachedMilestoneFromRequestOrNil(req *inx.MilestoneRequest) *storage.CachedMilestone {
	msIndex := req.GetMilestoneIndex()
	if msIndex == 0 {
		return deps.Storage.CachedMilestoneOrNil(req.GetMilestoneId().Unwrap())
	}

	return deps.Storage.CachedMilestoneByIndexOrNil(msIndex)
}

func inxMilestoneForMilestone(ms *storage.Milestone) *inx.Milestone {
	return &inx.Milestone{
		MilestoneInfo: inx.NewMilestoneInfo(
			ms.MilestoneID(),
			ms.Index(),
			ms.TimestampUnix()),
		Milestone: &inx.RawMilestone{
			Data: ms.Data(),
		},
	}
}

func inxMilestoneForCachedMilestone(ms *storage.CachedMilestone) *inx.Milestone {
	defer ms.Release(true) // milestone -1

	return &inx.Milestone{
		MilestoneInfo: inx.NewMilestoneInfo(
			ms.Milestone().MilestoneID(),
			ms.Milestone().Index(),
			ms.Milestone().TimestampUnix()),
		Milestone: &inx.RawMilestone{
			Data: ms.Milestone().Data(),
		},
	}
}

func milestoneForStoredMilestone(msIndex iotago.MilestoneIndex) (*inx.Milestone, error) {
	cachedMilestone := deps.Storage.CachedMilestoneByIndexOrNil(msIndex) // milestone +1
	if cachedMilestone == nil {
		return nil, status.Errorf(codes.NotFound, "milestone index %d not found", msIndex)
	}
	defer cachedMilestone.Release(true) // milestone -1

	return inxMilestoneForCachedMilestone(cachedMilestone.Retain()), nil // milestone + 1
}

func rawProtocolParametersForIndex(msIndex iotago.MilestoneIndex) (*inx.RawProtocolParameters, error) {
	milestoneOpt, err := deps.Storage.ProtocolParametersMilestoneOption(msIndex)
	if err != nil {
		return nil, err
	}

	return &inx.RawProtocolParameters{
		ProtocolVersion: uint32(milestoneOpt.ProtocolVersion),
		Params:          milestoneOpt.Params,
	}, nil
}

func (s *Server) ReadMilestone(_ context.Context, req *inx.MilestoneRequest) (*inx.Milestone, error) {
	cachedMilestone := cachedMilestoneFromRequestOrNil(req) // milestone +1
	if cachedMilestone == nil {
		return nil, status.Error(codes.NotFound, "milestone not found")
	}
	defer cachedMilestone.Release(true) // milestone -1

	return inxMilestoneForCachedMilestone(cachedMilestone.Retain()), nil // milestone +1
}

func (s *Server) ListenToLatestMilestones(_ *inx.NoParams, srv inx.INX_ListenToLatestMilestonesServer) error {
	ctx, cancel := context.WithCancel(Plugin.Daemon().ContextStopped())

	wp := workerpool.New(func(task workerpool.Task) {
		defer task.Return(nil)

		payload, ok := task.Param(0).(*inx.Milestone)
		if !ok {
			Plugin.LogErrorf("send error: expected *inx.Milestone, got %T", task.Param(0))
			cancel()

			return
		}

		if err := srv.Send(payload); err != nil {
			Plugin.LogErrorf("send error: %v", err)
			cancel()
		}

	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	onLatestMilestoneChanged := events.NewClosure(func(cachedMilestone *storage.CachedMilestone) {
		defer cachedMilestone.Release(true) // milestone -1

		payload := inxMilestoneForCachedMilestone(cachedMilestone.Retain())
		wp.Submit(payload)
	})

	wp.Start()
	deps.Tangle.Events.LatestMilestoneChanged.Hook(onLatestMilestoneChanged)
	<-ctx.Done()
	deps.Tangle.Events.LatestMilestoneChanged.Detach(onLatestMilestoneChanged)

	// We need to wait until all tasks are done, otherwise we might call
	// "SendMsg" and "CloseSend" in parallel on the grpc stream, which is
	// not safe according to the grpc docs.
	wp.StopAndWait()

	return ctx.Err()
}

func (s *Server) ListenToConfirmedMilestones(req *inx.MilestoneRangeRequest, srv inx.INX_ListenToConfirmedMilestonesServer) error {

	snapshotInfo := deps.Storage.SnapshotInfo()
	if snapshotInfo == nil {
		return common.ErrSnapshotInfoNotFound
	}

	createMilestonePayloadForIndexAndSend := func(msIndex iotago.MilestoneIndex) error {
		inxMilestone, err := milestoneForStoredMilestone(msIndex)
		if err != nil {
			return err
		}

		rawParams, err := rawProtocolParametersForIndex(msIndex)
		if err != nil {
			return err
		}

		payload := &inx.MilestoneAndProtocolParameters{
			Milestone:                 inxMilestone,
			CurrentProtocolParameters: rawParams,
		}

		if err := srv.Send(payload); err != nil {
			return fmt.Errorf("send error: %w", err)
		}

		return nil
	}

	createMilestoneAndProtocolParametersPayloadForMilestone := func(ms *storage.Milestone) (*inx.MilestoneAndProtocolParameters, error) {
		rawParams, err := rawProtocolParametersForIndex(ms.Index())
		if err != nil {
			return nil, err
		}

		return &inx.MilestoneAndProtocolParameters{
			Milestone:                 inxMilestoneForMilestone(ms),
			CurrentProtocolParameters: rawParams,
		}, nil
	}

	sendMilestonesRange := func(startIndex iotago.MilestoneIndex, endIndex iotago.MilestoneIndex) error {
		for currentIndex := startIndex; currentIndex <= endIndex; currentIndex++ {
			if err := createMilestonePayloadForIndexAndSend(currentIndex); err != nil {
				return err
			}
		}

		return nil
	}

	// if a startIndex is given, we send all available milestones including the start index.
	// if an endIndex is given, we send all available milestones up to and including min(ledgerIndex, endIndex).
	// if no startIndex is given, but an endIndex, we don't send previous milestones.
	sendPreviousMilestones := func(startIndex iotago.MilestoneIndex, endIndex iotago.MilestoneIndex) (iotago.MilestoneIndex, error) {
		if startIndex == 0 {
			// no need to send previous milestones
			return 0, nil
		}

		cmi := deps.SyncManager.ConfirmedMilestoneIndex()

		if startIndex > cmi {
			// no need to send previous milestones
			return 0, nil
		}

		// Stream all available milestones first
		pruningIndex := snapshotInfo.PruningIndex()
		if startIndex <= pruningIndex {
			return 0, status.Errorf(codes.InvalidArgument, "given startMilestoneIndex %d is older than the current pruningIndex %d", startIndex, pruningIndex)
		}

		if endIndex == 0 || endIndex > cmi {
			endIndex = cmi
		}

		if err := sendMilestonesRange(startIndex, endIndex); err != nil {
			return 0, err
		}

		return endIndex, nil
	}

	stream := &streamRange{
		start: req.GetStartMilestoneIndex(),
		end:   req.GetEndMilestoneIndex(),
	}

	var err error
	stream.lastSent, err = sendPreviousMilestones(stream.start, stream.end)
	if err != nil {
		return err
	}

	if stream.isBounded() && stream.lastSent >= stream.end {
		// We are done sending, so close the stream
		return nil
	}

	catchUpFunc := func(start iotago.MilestoneIndex, end iotago.MilestoneIndex) error {
		err := sendMilestonesRange(start, end)
		if err != nil {
			err := fmt.Errorf("sendMilestonesRange error: %w", err)
			Plugin.LogError(err.Error())

			return err
		}

		return nil
	}

	sendFunc := func(task *workerpool.Task, _ iotago.MilestoneIndex) error {
		// no release needed

		payload, ok := task.Param(0).(*inx.MilestoneAndProtocolParameters)
		if !ok {
			err := fmt.Errorf("expected *inx.MilestoneAndProtocolParameters, got %T", task.Param(0))
			Plugin.LogErrorf("send error: %v", err)

			return err
		}

		if err := srv.Send(payload); err != nil {
			err := fmt.Errorf("send error: %w", err)
			Plugin.LogError(err.Error())

			return err
		}

		return nil
	}

	var innerErr error
	ctx, cancel := context.WithCancel(Plugin.Daemon().ContextStopped())

	wp := workerpool.New(func(task workerpool.Task) {
		defer task.Return(nil)

		msIndex, ok := task.Param(1).(iotago.MilestoneIndex)
		if !ok {
			Plugin.LogErrorf("send error: expected iotago.MilestoneIndex, got %T", task.Param(0))
			cancel()

			return
		}

		done, err := handleRangedSend(&task, msIndex, stream, catchUpFunc, sendFunc)
		switch {
		case err != nil:
			innerErr = err
			cancel()

		case done:
			cancel()
		}

	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	onConfirmedMilestoneChanged := events.NewClosure(func(cachedMilestone *storage.CachedMilestone) {
		defer cachedMilestone.Release(true) // milestone -1

		payload, err := createMilestoneAndProtocolParametersPayloadForMilestone(cachedMilestone.Milestone())
		if err != nil {
			Plugin.LogErrorf("serialize error: %v", err)
			cancel()

			return
		}

		wp.Submit(payload, cachedMilestone.Milestone().Index())
	})

	wp.Start()
	deps.Tangle.Events.ConfirmedMilestoneChanged.Hook(onConfirmedMilestoneChanged)
	<-ctx.Done()
	deps.Tangle.Events.ConfirmedMilestoneChanged.Detach(onConfirmedMilestoneChanged)

	// We need to wait until all tasks are done, otherwise we might call
	// "SendMsg" and "CloseSend" in parallel on the grpc stream, which is
	// not safe according to the grpc docs.
	wp.StopAndWait()

	return innerErr
}

func (s *Server) ComputeWhiteFlag(ctx context.Context, req *inx.WhiteFlagRequest) (*inx.WhiteFlagResponse, error) {

	requestedIndex := req.GetMilestoneIndex()
	requestedTimestamp := req.GetMilestoneTimestamp()
	requestedParents := req.UnwrapParents()
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
		MilestoneInclusionMerkleRoot: mutations.InclusionMerkleRoot[:],
		MilestoneAppliedMerkleRoot:   mutations.AppliedMerkleRoot[:],
	}, nil
}

func (s *Server) ReadMilestoneCone(req *inx.MilestoneRequest, srv inx.INX_ReadMilestoneConeServer) error {
	cachedMilestone := cachedMilestoneFromRequestOrNil(req) // milestone +1
	if cachedMilestone == nil {
		return status.Error(codes.NotFound, "milestone not found")
	}
	defer cachedMilestone.Release(true) // milestone -1

	return milestoneCone(cachedMilestone.Milestone().Index(), cachedMilestone.Milestone().Parents(), func(metadata *storage.BlockMetadata) error {
		cachedBlock := deps.Storage.CachedBlockOrNil(metadata.BlockID()) // block + 1
		if cachedBlock == nil {
			return status.Errorf(codes.Internal, "block %s not found", metadata.BlockID().ToHex())
		}
		defer cachedBlock.Release(true)

		meta, err := NewINXBlockMetadata(Plugin.Daemon().ContextStopped(), metadata.BlockID(), metadata)
		if err != nil {
			return err
		}

		payload := &inx.BlockWithMetadata{
			Metadata: meta,
			Block: &inx.RawBlock{
				Data: cachedBlock.Block().Data(),
			},
		}

		return srv.Send(payload)
	})
}

func (s *Server) ReadMilestoneConeMetadata(req *inx.MilestoneRequest, srv inx.INX_ReadMilestoneConeMetadataServer) error {
	cachedMilestone := cachedMilestoneFromRequestOrNil(req) // milestone +1
	if cachedMilestone == nil {
		return status.Error(codes.NotFound, "milestone not found")
	}
	defer cachedMilestone.Release(true) // milestone -1

	return milestoneCone(cachedMilestone.Milestone().Index(), cachedMilestone.Milestone().Parents(), func(metadata *storage.BlockMetadata) error {
		payload, err := NewINXBlockMetadata(Plugin.Daemon().ContextStopped(), metadata.BlockID(), metadata)
		if err != nil {
			return err
		}

		return srv.Send(payload)
	})
}

func milestoneCone(index iotago.MilestoneIndex, parents iotago.BlockIDs, consumer func(metadata *storage.BlockMetadata) error) error {

	if index > deps.SyncManager.ConfirmedMilestoneIndex() {
		return status.Errorf(codes.InvalidArgument, "milestone %d not confirmed yet", index)
	}

	memcachedTraverserStorage := dag.NewMemcachedTraverserStorage(deps.Storage, storage.NewMetadataMemcache(deps.Storage.CachedBlockMetadata))
	defer memcachedTraverserStorage.Cleanup(true)

	if err := dag.TraverseParents(
		Plugin.Daemon().ContextStopped(),
		memcachedTraverserStorage,
		parents,
		// traversal stops if no more blocks pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedBlockMeta.Release(true) // meta -1
			if referenced, at := cachedBlockMeta.Metadata().ReferencedWithIndex(); referenced {
				if at < index {
					return false, nil
				}
			}

			return true, nil
		},
		// consumer
		func(cachedBlockMeta *storage.CachedMetadata) error { // meta +1
			defer cachedBlockMeta.Release(true)

			return consumer(cachedBlockMeta.Metadata())
		},
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false); err != nil {
		if errors.Is(err, common.ErrOperationAborted) {
			return status.Errorf(codes.Unavailable, "traverse parents failed, error: %s", err)
		}

		return status.Errorf(codes.Internal, "traverse parents failed, error: %s", err)
	}

	return nil
}
