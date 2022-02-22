package inx

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"

	"github.com/gohornet/hornet/pkg/inx"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/serializer/v2"
)

func newINXServer() *INXServer {
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	s := &INXServer{grpcServer: grpcServer}
	inx.RegisterINXServer(grpcServer, s)
	return s
}

type INXServer struct {
	inx.UnimplementedINXServer
	grpcServer *grpc.Server
}

func (s *INXServer) Start() {
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", INXPort))
		if err != nil {
			Plugin.LogFatalf("failed to listen: %v", err)
		}
		defer lis.Close()

		if err := s.grpcServer.Serve(lis); err != nil {
			Plugin.LogFatalf("failed to serve: %v", err)
		}
	}()
}

func (s *INXServer) Stop() {
	s.grpcServer.Stop()
}

func (s *INXServer) ListenToMessages(filter *inx.MessageFilter, srv inx.INX_ListenToMessagesServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	onMessageReceived := events.NewClosure(func(cachedMsg *storage.CachedMessage, latestMilestoneIndex milestone.Index, confirmedMilestoneIndex milestone.Index) {
		defer cachedMsg.Release(true)

		//TODO: use workerpool?
		//TODO: apply filter?
		payload := INXMessageWithBytes(cachedMsg.Message().MessageID(), cachedMsg.Message().Data())
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("Send error: %v", err)
			cancel()
		}
	})
	deps.Tangle.Events.ReceivedNewMessage.Attach(onMessageReceived)
	<-ctx.Done()
	deps.Tangle.Events.ReceivedNewMessage.Detach(onMessageReceived)
	return ctx.Err()
}

func (s *INXServer) SubmitMessage(context context.Context, req *inx.SubmitMessageRequest) (*inx.SubmitMessageResponse, error) {
	msg, err := req.GetMessage().UnwrapMessage(serializer.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}

	mergedCtx, mergedCtxCancel := utils.MergeContexts(context, Plugin.Daemon().ContextStopped())
	defer mergedCtxCancel()

	messageID, err := attacher.AttachMessage(mergedCtx, msg)
	if err != nil {
		return nil, err
	}

	r := &inx.SubmitMessageResponse{
		MessageId: make([]byte, len(messageID)),
	}
	copy(r.MessageId, messageID[:])
	return r, nil
}

func (s *INXServer) ReadLockLedger(context.Context, *inx.NoParams) (*inx.NoParams, error) {
	deps.UTXOManager.ReadLockLedger()
	return &inx.NoParams{}, nil
}

func (s *INXServer) ReadUnlockLedger(context.Context, *inx.NoParams) (*inx.NoParams, error) {
	deps.UTXOManager.ReadUnlockLedger()
	return &inx.NoParams{}, nil
}

func (s *INXServer) LedgerStatus(context.Context, *inx.NoParams) (*inx.LedgerStatusResponse, error) {
	index, err := deps.UTXOManager.ReadLedgerIndex()
	if err != nil {
		return nil, err
	}
	return &inx.LedgerStatusResponse{
		LedgerIndex: uint32(index),
	}, nil
}

func (s *INXServer) ReadUnspentOutputs(_ *inx.NoParams, srv inx.INX_ReadUnspentOutputsServer) error {
	var innerErr error
	err := deps.UTXOManager.ForEachUnspentOutput(func(output *utxo.Output) bool {
		payload, err := INXOutputWithOutput(output)
		if err != nil {
			innerErr = err
			return false
		}
		if err := srv.Send(payload); err != nil {
			innerErr = err
			return false
		}
		return true
	})
	if innerErr != nil {
		return innerErr
	}
	return err
}

func (s *INXServer) ListenToLedgerUpdates(_ *inx.NoParams, srv inx.INX_ListenToLedgerUpdatesServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	onLedgerUpdated := events.NewClosure(func(index milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents) {
		payload, err := INXLedgerUpdated(index, newOutputs, newSpents)
		if err != nil {
			Plugin.LogInfof("Send error: %v", err)
			cancel()
		}
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("Send error: %v", err)
			cancel()
		}
	})
	deps.Tangle.Events.LedgerUpdated.Attach(onLedgerUpdated)
	<-ctx.Done()
	deps.Tangle.Events.LedgerUpdated.Detach(onLedgerUpdated)
	return ctx.Err()

}
