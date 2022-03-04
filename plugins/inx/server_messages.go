package inx

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/inx"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/workerpool"
)

func INXNewMessageMetadata(messageID hornet.MessageID, metadata *storage.MessageMetadata) (*inx.MessageMetadata, error) {
	m := &inx.MessageMetadata{
		MessageId: inx.NewMessageId(messageID),
		Parents:   metadata.Parents().ToSliceOfSlices(),
		Solid:     metadata.IsSolid(),
	}

	referenced, msIndex := metadata.ReferencedWithIndex()
	if referenced {
		m.ReferencedByMilestoneIndex = uint32(msIndex)
		inclusionState := inx.MessageMetadata_NO_TRANSACTION
		conflict := metadata.Conflict()
		if conflict != storage.ConflictNone {
			inclusionState = inx.MessageMetadata_CONFLICTING
			m.ConflictReason = inx.MessageMetadata_ConflictReason(conflict)
		} else if metadata.IsIncludedTxInLedger() {
			inclusionState = inx.MessageMetadata_INCLUDED
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

		shouldPromote := false
		shouldReattach := false

		switch tipScore {
		case tangle.TipScoreNotFound:
			return nil, errors.WithMessage(echo.ErrInternalServerError, "tip score could not be calculated")
		case tangle.TipScoreOCRIThresholdReached, tangle.TipScoreYCRIThresholdReached:
			shouldPromote = true
			shouldReattach = false
		case tangle.TipScoreBelowMaxDepth:
			shouldPromote = false
			shouldReattach = true
		}

		m.ShouldPromote = shouldPromote
		m.ShouldReattach = shouldReattach
	}

	return m, nil
}

func (s *INXServer) ReadMessage(_ context.Context, messageID *inx.MessageId) (*inx.RawMessage, error) {
	cachedMsg := deps.Storage.CachedMessageOrNil(messageID.Unwrap())
	if cachedMsg == nil {
		return nil, status.Errorf(codes.NotFound, "message %s not found", messageID.Unwrap().ToHex())
	}
	defer cachedMsg.Release(true)
	return inx.WrapMessage(cachedMsg.Message().Message())
}

func (s *INXServer) ReadMessageMetadata(_ context.Context, messageID *inx.MessageId) (*inx.MessageMetadata, error) {
	cachedMsgMeta := deps.Storage.CachedMessageMetadataOrNil(messageID.Unwrap())
	if cachedMsgMeta == nil {
		return nil, status.Errorf(codes.NotFound, "message metadata %s not found", messageID.Unwrap().ToHex())
	}
	defer cachedMsgMeta.Release(true)
	return INXNewMessageMetadata(cachedMsgMeta.Metadata().MessageID(), cachedMsgMeta.Metadata())
}

func (s *INXServer) ListenToMessages(filter *inx.MessageFilter, srv inx.INX_ListenToMessagesServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		cachedMsg := task.Param(0).(*storage.CachedMessage)
		defer cachedMsg.Release(true)

		payload := inx.NewMessageWithBytes(cachedMsg.Message().MessageID(), cachedMsg.Message().Data())
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

func (s *INXServer) ListenToSolidMessages(filter *inx.MessageFilter, srv inx.INX_ListenToSolidMessagesServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		msgMeta := task.Param(0).(*storage.CachedMetadata)
		defer msgMeta.Release(true)

		payload, err := INXNewMessageMetadata(msgMeta.Metadata().MessageID(), msgMeta.Metadata())
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

func (s *INXServer) ListenToReferencedMessages(filter *inx.MessageFilter, srv inx.INX_ListenToReferencedMessagesServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	wp := workerpool.New(func(task workerpool.Task) {
		msgMeta := task.Param(0).(*storage.CachedMetadata)
		defer msgMeta.Release(true)

		payload, err := INXNewMessageMetadata(msgMeta.Metadata().MessageID(), msgMeta.Metadata())
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
	closure := events.NewClosure(func(msgMeta *storage.CachedMetadata, index milestone.Index, confTime uint64) {
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

func (s *INXServer) SubmitMessage(context context.Context, message *inx.RawMessage) (*inx.MessageId, error) {
	msg, err := message.UnwrapMessage(serializer.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}

	mergedCtx, mergedCtxCancel := utils.MergeContexts(context, Plugin.Daemon().ContextStopped())
	defer mergedCtxCancel()

	messageID, err := attacher.AttachMessage(mergedCtx, msg)
	if err != nil {
		return nil, err
	}
	return inx.NewMessageId(messageID), nil
}
