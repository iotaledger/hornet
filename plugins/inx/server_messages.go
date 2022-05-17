package inx

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/iotaledger/hive.go/contextutils"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/workerpool"
	inx "github.com/iotaledger/inx/go"
)

func INXBlockIDsFromBlockIDs(messageIDs hornet.MessageIDs) []*inx.BlockId {
	result := make([]*inx.BlockId, len(messageIDs))
	for i := range messageIDs {
		result[i] = inx.NewBlockId(messageIDs[i].ToArray())
	}
	return result
}

func BlockIDsFromINXBlockIDs(messageIDs []*inx.BlockId) hornet.MessageIDs {
	result := make([]hornet.MessageID, len(messageIDs))
	for i := range messageIDs {
		result[i] = hornet.MessageIDFromArray(messageIDs[i].Unwrap())
	}
	return result
}

func INXNewBlockMetadata(messageID hornet.MessageID, metadata *storage.MessageMetadata) (*inx.BlockMetadata, error) {
	m := &inx.BlockMetadata{
		BlockId: inx.NewBlockId(messageID.ToArray()),
		Parents: INXBlockIDsFromBlockIDs(metadata.Parents()),
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

		tipScore, err := deps.TipScoreCalculator.TipScore(Plugin.Daemon().ContextStopped(), messageID, cmi)
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

func (s *INXServer) ReadBlock(_ context.Context, messageID *inx.BlockId) (*inx.RawBlock, error) {
	cachedMsg := deps.Storage.CachedMessageOrNil(hornet.MessageIDFromArray(messageID.Unwrap())) // message +1
	if cachedMsg == nil {
		return nil, status.Errorf(codes.NotFound, "message %s not found", hornet.MessageIDFromArray(messageID.Unwrap()).ToHex())
	}
	defer cachedMsg.Release(true) // message -1
	return inx.WrapBlock(cachedMsg.Message().Message())
}

func (s *INXServer) ReadBlockMetadata(_ context.Context, messageID *inx.BlockId) (*inx.BlockMetadata, error) {
	cachedMsgMeta := deps.Storage.CachedMessageMetadataOrNil(hornet.MessageIDFromArray(messageID.Unwrap())) // meta +1
	if cachedMsgMeta == nil {
		return nil, status.Errorf(codes.NotFound, "message metadata %s not found", hornet.MessageIDFromArray(messageID.Unwrap()).ToHex())
	}
	defer cachedMsgMeta.Release(true) // meta -1
	return INXNewBlockMetadata(cachedMsgMeta.Metadata().MessageID(), cachedMsgMeta.Metadata())
}

func (s *INXServer) ListenToBlocks(filter *inx.BlockFilter, srv inx.INX_ListenToBlocksServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		cachedMsg := task.Param(0).(*storage.CachedMessage)
		defer cachedMsg.Release(true) // message -1

		payload := inx.NewBlockWithBytes(cachedMsg.Message().MessageID().ToArray(), cachedMsg.Message().Data())
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("Send error: %v", err)
			cancel()
		}
		task.Return(nil)
	})
	closure := events.NewClosure(func(cachedMsg *storage.CachedMessage, latestMilestoneIndex milestone.Index, confirmedMilestoneIndex milestone.Index) {
		//TODO: apply filter?
		wp.Submit(cachedMsg)
	})
	wp.Start()
	deps.Tangle.Events.ReceivedNewMessage.Attach(closure)
	<-ctx.Done()
	deps.Tangle.Events.ReceivedNewMessage.Detach(closure)
	wp.Stop()
	return ctx.Err()
}

func (s *INXServer) ListenToSolidBlocks(filter *inx.BlockFilter, srv inx.INX_ListenToSolidBlocksServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		msgMeta := task.Param(0).(*storage.CachedMetadata)
		defer msgMeta.Release(true) // meta -1

		payload, err := INXNewBlockMetadata(msgMeta.Metadata().MessageID(), msgMeta.Metadata())
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
	closure := events.NewClosure(func(msgMeta *storage.CachedMetadata) {
		//TODO: apply filter?
		wp.Submit(msgMeta)
	})
	wp.Start()
	deps.Tangle.Events.MessageSolid.Attach(closure)
	<-ctx.Done()
	deps.Tangle.Events.MessageSolid.Detach(closure)
	wp.Stop()
	return ctx.Err()
}

func (s *INXServer) ListenToReferencedBlocks(filter *inx.BlockFilter, srv inx.INX_ListenToReferencedBlocksServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		msgMeta := task.Param(0).(*storage.CachedMetadata)
		defer msgMeta.Release(true) // meta -1

		payload, err := INXNewBlockMetadata(msgMeta.Metadata().MessageID(), msgMeta.Metadata())
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
	closure := events.NewClosure(func(msgMeta *storage.CachedMetadata, index milestone.Index, confTime uint32) {
		//TODO: apply filter?
		wp.Submit(msgMeta)
	})
	wp.Start()
	deps.Tangle.Events.MessageReferenced.Attach(closure)
	<-ctx.Done()
	deps.Tangle.Events.MessageReferenced.Detach(closure)
	wp.Stop()
	return ctx.Err()
}

func (s *INXServer) SubmitBlock(context context.Context, message *inx.RawBlock) (*inx.BlockId, error) {
	msg, err := message.UnwrapBlock(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return nil, err
	}

	mergedCtx, mergedCtxCancel := contextutils.MergeContexts(context, Plugin.Daemon().ContextStopped())
	defer mergedCtxCancel()

	messageID, err := attacher.AttachMessage(mergedCtx, msg)
	if err != nil {
		return nil, err
	}
	return inx.NewBlockId(messageID.ToArray()), nil
}
