package inx

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/iotaledger/hive.go/contextutils"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/workerpool"
	inx "github.com/iotaledger/inx/go"
	iotago "github.com/iotaledger/iota.go/v3"
)

func INXNewBlockMetadata(blockID iotago.BlockID, metadata *storage.BlockMetadata) (*inx.BlockMetadata, error) {
	m := &inx.BlockMetadata{
		BlockId: inx.NewBlockId(blockID),
		Parents: inx.NewBlockIds(metadata.Parents()),
		Solid:   metadata.IsSolid(),
	}

	referenced, msIndex := metadata.ReferencedWithIndex()
	if referenced {
		m.ReferencedByMilestoneIndex = uint32(msIndex)
		inclusionState := inx.BlockMetadata_NO_TRANSACTION
		conflict := metadata.Conflict()
		if conflict != storage.ConflictNone {
			inclusionState = inx.BlockMetadata_CONFLICTING
			m.ConflictReason = inx.BlockMetadata_ConflictReason(conflict)
		} else if metadata.IsIncludedTxInLedger() {
			inclusionState = inx.BlockMetadata_INCLUDED
		}
		m.LedgerInclusionState = inclusionState

		if metadata.IsMilestone() {
			m.MilestoneIndex = uint32(msIndex)
		}
	} else if metadata.IsSolid() {
		// determine info about the quality of the tip if not referenced
		cmi := deps.SyncManager.ConfirmedMilestoneIndex()

		tipScore, err := deps.TipScoreCalculator.TipScore(Plugin.Daemon().ContextStopped(), blockID, cmi)
		if err != nil {
			if errors.Is(err, common.ErrOperationAborted) {
				return nil, errors.WithMessage(echo.ErrServiceUnavailable, err.Error())
			}
			return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
		}

		var shouldPromote bool
		var shouldReattach bool

		switch tipScore {
		case tangle.TipScoreNotFound:
			return nil, errors.WithMessage(echo.ErrInternalServerError, "tip score could not be calculated")
		case tangle.TipScoreOCRIThresholdReached, tangle.TipScoreYCRIThresholdReached:
			shouldPromote = true
			shouldReattach = false
		case tangle.TipScoreBelowMaxDepth:
			shouldPromote = false
			shouldReattach = true
		case tangle.TipScoreHealthy:
			shouldPromote = false
			shouldReattach = false
		}

		m.ShouldPromote = shouldPromote
		m.ShouldReattach = shouldReattach
	}

	return m, nil
}

func (s *INXServer) ReadBlock(_ context.Context, blockID *inx.BlockId) (*inx.RawBlock, error) {
	blkId := blockID.Unwrap()
	cachedBlock := deps.Storage.CachedBlockOrNil(blkId) // block +1
	if cachedBlock == nil {
		return nil, status.Errorf(codes.NotFound, "block %s not found", blkId.ToHex())
	}
	defer cachedBlock.Release(true) // block -1
	return inx.WrapBlock(cachedBlock.Block().Block())
}

func (s *INXServer) ReadBlockMetadata(_ context.Context, blockID *inx.BlockId) (*inx.BlockMetadata, error) {
	blkId := blockID.Unwrap()
	cachedBlockMeta := deps.Storage.CachedBlockMetadataOrNil(blkId) // meta +1
	if cachedBlockMeta == nil {
		isSolidEntryPoint, err := deps.Storage.SolidEntryPointsContain(blkId)
		if err != nil && isSolidEntryPoint {
			return &inx.BlockMetadata{
				BlockId: blockID,
				Solid:   true,
			}, nil
		}
		return nil, status.Errorf(codes.NotFound, "block metadata %s not found", blkId.ToHex())
	}
	defer cachedBlockMeta.Release(true) // meta -1
	return INXNewBlockMetadata(cachedBlockMeta.Metadata().BlockID(), cachedBlockMeta.Metadata())
}

func (s *INXServer) ListenToBlocks(_ *inx.NoParams, srv inx.INX_ListenToBlocksServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		cachedBlock := task.Param(0).(*storage.CachedBlock)
		defer cachedBlock.Release(true) // block -1

		payload := inx.NewBlockWithBytes(cachedBlock.Block().BlockID(), cachedBlock.Block().Data())
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("Send error: %v", err)
			cancel()
		}
		task.Return(nil)
	})
	closure := events.NewClosure(func(cachedBlock *storage.CachedBlock, latestMilestoneIndex milestone.Index, confirmedMilestoneIndex milestone.Index) {
		wp.Submit(cachedBlock)
	})
	wp.Start()
	deps.Tangle.Events.ReceivedNewBlock.Attach(closure)
	<-ctx.Done()
	deps.Tangle.Events.ReceivedNewBlock.Detach(closure)
	wp.Stop()
	return ctx.Err()
}

func (s *INXServer) ListenToSolidBlocks(_ *inx.NoParams, srv inx.INX_ListenToSolidBlocksServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		blockMeta := task.Param(0).(*storage.CachedMetadata)
		defer blockMeta.Release(true) // meta -1

		payload, err := INXNewBlockMetadata(blockMeta.Metadata().BlockID(), blockMeta.Metadata())
		if err != nil {
			Plugin.LogInfof("Send error: %v", err)
			cancel()
			return
		}
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("Send error: %v", err)
			cancel()
		}
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))
	closure := events.NewClosure(func(blockMeta *storage.CachedMetadata) {
		wp.Submit(blockMeta)
	})
	wp.Start()
	deps.Tangle.Events.BlockSolid.Attach(closure)
	<-ctx.Done()
	deps.Tangle.Events.BlockSolid.Detach(closure)
	wp.Stop()
	return ctx.Err()
}

func (s *INXServer) ListenToReferencedBlocks(_ *inx.NoParams, srv inx.INX_ListenToReferencedBlocksServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		blockMeta := task.Param(0).(*storage.CachedMetadata)
		defer blockMeta.Release(true) // meta -1

		payload, err := INXNewBlockMetadata(blockMeta.Metadata().BlockID(), blockMeta.Metadata())
		if err != nil {
			Plugin.LogInfof("Send error: %v", err)
			cancel()
			return
		}
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("Send error: %v", err)
			cancel()
		}
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))
	closure := events.NewClosure(func(blockMeta *storage.CachedMetadata, index milestone.Index, confTime uint32) {
		wp.Submit(blockMeta)
	})
	wp.Start()
	deps.Tangle.Events.BlockReferenced.Attach(closure)
	<-ctx.Done()
	deps.Tangle.Events.BlockReferenced.Detach(closure)
	wp.Stop()
	return ctx.Err()
}

func (s *INXServer) SubmitBlock(context context.Context, rawBlock *inx.RawBlock) (*inx.BlockId, error) {
	block, err := rawBlock.UnwrapBlock(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return nil, err
	}

	mergedCtx, mergedCtxCancel := contextutils.MergeContexts(context, Plugin.Daemon().ContextStopped())
	defer mergedCtxCancel()

	blockID, err := attacher.AttachBlock(mergedCtx, block)
	if err != nil {
		return nil, err
	}
	return inx.NewBlockId(blockID), nil
}
