package inx

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gohornet/hornet/pkg/inx"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/workerpool"
)

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
	return inx.NewMessageMetadata(cachedMsgMeta.Metadata().MessageID(), cachedMsgMeta.Metadata()), nil
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

		payload := inx.NewMessageMetadata(msgMeta.Metadata().MessageID(), msgMeta.Metadata())
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

		payload := inx.NewMessageMetadata(msgMeta.Metadata().MessageID(), msgMeta.Metadata())
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
