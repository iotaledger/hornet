package inx

import (
	"context"

	"github.com/pkg/errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/hornet/pkg/common"
	"github.com/iotaledger/hornet/pkg/dag"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/tangle"
	inx "github.com/iotaledger/inx/go"
	iotago "github.com/iotaledger/iota.go/v3"
)

// milestone +1
func cachedMilestoneFromRequestOrNil(req *inx.MilestoneRequest) *storage.CachedMilestone {
	msIndex := milestone.Index(req.GetMilestoneIndex())
	if msIndex == 0 {
		return deps.Storage.CachedMilestoneOrNil(req.GetMilestoneId().Unwrap())
	}
	return deps.Storage.CachedMilestoneByIndexOrNil(msIndex)
}

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
		return nil, status.Errorf(codes.NotFound, "milestone index %d not found", msIndex)
	}
	defer cachedMilestone.Release(true) // milestone -1

	return milestoneForCachedMilestone(cachedMilestone.Retain()) // milestone + 1
}

func (s *INXServer) ReadMilestone(_ context.Context, req *inx.MilestoneRequest) (*inx.Milestone, error) {
	cachedMilestone := cachedMilestoneFromRequestOrNil(req) // milestone +1
	if cachedMilestone == nil {
		return nil, status.Error(codes.NotFound, "milestone not found")
	}
	defer cachedMilestone.Release(true)                          // milestone -1
	return milestoneForCachedMilestone(cachedMilestone.Retain()) // milestone +1
}

func (s *INXServer) ListenToLatestMilestones(_ *inx.NoParams, srv inx.INX_ListenToLatestMilestonesServer) error {
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
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))
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

func (s *INXServer) ListenToConfirmedMilestones(req *inx.MilestoneRangeRequest, srv inx.INX_ListenToConfirmedMilestonesServer) error {

	sendPreviousMilestones := func(startIndex milestone.Index, endIndex milestone.Index) (milestone.Index, error) {
		cmi := deps.SyncManager.ConfirmedMilestoneIndex()
		if endIndex == 0 || cmi < endIndex {
			endIndex = cmi
		}

		if startIndex > 0 && startIndex <= cmi {
			// Stream all available milestones
			pruningIndex := deps.Storage.SnapshotInfo().PruningIndex
			if startIndex <= pruningIndex {
				return 0, status.Errorf(codes.InvalidArgument, "given startMilestoneIndex %d is older than the current pruningIndex %d", startIndex, pruningIndex)
			}

			for currentIndex := startIndex; currentIndex <= endIndex; currentIndex++ {
				payload, err := milestoneForIndex(currentIndex)
				if err != nil {
					return 0, err
				}
				if err := srv.Send(payload); err != nil {
					return 0, err
				}
			}
		}
		return endIndex, nil
	}

	requestStartIndex := milestone.Index(req.GetStartMilestoneIndex())
	requestEndIndex := milestone.Index(req.GetEndMilestoneIndex())

	lastSentIndex, err := sendPreviousMilestones(requestStartIndex, requestEndIndex)
	if err != nil {
		return err
	}

	if requestEndIndex > 0 && lastSentIndex >= requestEndIndex {
		// We are done sending, so close the stream
		return nil
	}

	var innerErr error
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		cachedMilestone := task.Param(0).(*storage.CachedMilestone)
		defer cachedMilestone.Release(true) // milestone -1

		if requestStartIndex > 0 && cachedMilestone.Milestone().Index() < requestStartIndex {
			// Skip this because it is before the index we requested
			task.Return(nil)
			return
		}

		payload, err := milestoneForCachedMilestone(cachedMilestone.Retain()) // milestone +1
		if err != nil {
			Plugin.LogInfof("error creating milestone: %v", err)
			cancel()
			innerErr = err
			task.Return(nil)
			return
		}
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("send error: %v", err)
			cancel()
			innerErr = err
			task.Return(nil)
			return
		}

		if requestEndIndex > 0 && cachedMilestone.Milestone().Index() >= requestEndIndex {
			// We are done sending the milestones
			innerErr = nil
			cancel()
		}

		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))
	closure := events.NewClosure(func(milestone *storage.CachedMilestone) {
		wp.Submit(milestone)
	})
	wp.Start()
	deps.Tangle.Events.ConfirmedMilestoneChanged.Attach(closure)
	<-ctx.Done()
	deps.Tangle.Events.ConfirmedMilestoneChanged.Detach(closure)
	wp.Stop()
	return innerErr
}

func (s *INXServer) ComputeWhiteFlag(ctx context.Context, req *inx.WhiteFlagRequest) (*inx.WhiteFlagResponse, error) {

	requestedIndex := milestone.Index(req.GetMilestoneIndex())
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

func (s *INXServer) ReadMilestoneCone(req *inx.MilestoneRequest, srv inx.INX_ReadMilestoneConeServer) error {
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

		meta, err := INXNewBlockMetadata(metadata.BlockID(), metadata)
		if err != nil {
			return err
		}

		data := cachedBlock.Block().Data()
		payload := &inx.BlockWithMetadata{
			Metadata: meta,
			Block: &inx.RawBlock{
				Data: make([]byte, len(data)),
			},
		}
		copy(payload.Block.Data[:], data[:])
		return srv.Send(payload)
	})
}

func (s *INXServer) ReadMilestoneConeMetadata(req *inx.MilestoneRequest, srv inx.INX_ReadMilestoneConeMetadataServer) error {
	cachedMilestone := cachedMilestoneFromRequestOrNil(req) // milestone +1
	if cachedMilestone == nil {
		return status.Error(codes.NotFound, "milestone not found")
	}
	defer cachedMilestone.Release(true) // milestone -1

	return milestoneCone(cachedMilestone.Milestone().Index(), cachedMilestone.Milestone().Parents(), func(metadata *storage.BlockMetadata) error {
		payload, err := INXNewBlockMetadata(metadata.BlockID(), metadata)
		if err != nil {
			return err
		}
		return srv.Send(payload)
	})
}

func milestoneCone(index milestone.Index, parents iotago.BlockIDs, consumer func(metadata *storage.BlockMetadata) error) error {

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
