package inx

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gohornet/hornet/pkg/inx"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/utils"
	restapiv2 "github.com/gohornet/hornet/plugins/restapi/v2"
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

func (s *INXServer) ListenToReferencedMessages(filter *inx.MessageFilter, srv inx.INX_ListenToReferencedMessagesServer) error {
	ctx, cancel := context.WithCancel(context.Background())
	onReferencedMessage := events.NewClosure(func(msgMeta *storage.CachedMetadata, index milestone.Index, confTime uint64) {
		defer msgMeta.Release(true)

		//TODO: use workerpool?
		//TODO: apply filter?
		payload := INXMessageMetadata(msgMeta.Metadata().MessageID(), msgMeta.Metadata(), index)
		if err := srv.Send(payload); err != nil {
			Plugin.LogInfof("Send error: %v", err)
			cancel()
		}
	})
	deps.Tangle.Events.MessageReferenced.Attach(onReferencedMessage)
	<-ctx.Done()
	deps.Tangle.Events.MessageReferenced.Detach(onReferencedMessage)
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

func (s *INXServer) ReadUnspentOutputs(_ *inx.NoParams, srv inx.INX_ReadUnspentOutputsServer) error {
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

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

func (s *INXServer) ListenToLedgerUpdates(req *inx.LedgerUpdateRequest, srv inx.INX_ListenToLedgerUpdatesServer) error {
	startIndex := milestone.Index(req.GetStartMilestoneIndex())
	if startIndex > 0 {
		// Stream all available milestone diffs first
		pruningIndex := deps.Storage.SnapshotInfo().PruningIndex
		if startIndex <= pruningIndex {
			return status.Errorf(codes.InvalidArgument, "given startMilestoneIndex %d is older than the current pruningIndex %d", startIndex, pruningIndex)
		}

		deps.UTXOManager.ReadLockLedger()
		ledgerIndex, err := deps.UTXOManager.ReadLedgerIndex()
		if err != nil {
			deps.UTXOManager.ReadUnlockLedger()
			return status.Error(codes.Unavailable, "error accessing the UTXO ledger")
		}
		currentIndex := startIndex
		for currentIndex <= ledgerIndex {
			msDiff, err := deps.UTXOManager.MilestoneDiff(currentIndex)
			if err != nil {
				deps.UTXOManager.ReadUnlockLedger()
				return status.Errorf(codes.NotFound, "ledger update for milestoneIndex %d not found", currentIndex)
			}
			payload, err := INXLedgerUpdated(msDiff.Index, msDiff.Outputs, msDiff.Spents)
			if err := srv.Send(payload); err != nil {
				deps.UTXOManager.ReadLockLedger()
				return err
			}
			currentIndex++
		}
		deps.UTXOManager.ReadUnlockLedger()
	}

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

func proxyMiddleware(host string, port uint32) (echo.MiddlewareFunc, error) {
	apiURL, err := url.Parse(fmt.Sprintf("http://%s:%d", host, port))
	if err != nil {
		return nil, err
	}

	config := middleware.DefaultProxyConfig
	config.Balancer = middleware.NewRoundRobinBalancer([]*middleware.ProxyTarget{
		{
			URL: apiURL,
		},
	})
	config.Rewrite = map[string]string{
		"^/api/plugins/*": "/$1",
	}

	return middleware.ProxyWithConfig(config), nil
}

func (s *INXServer) RegisterAPIRoute(_ context.Context, req *inx.APIRouteRequest) (*inx.NoParams, error) {
	if len(req.GetRoute()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "route can not be empty")
	}
	if len(req.GetHost()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "host can not be empty")
	}
	if req.GetPort() == 0 {
		return nil, status.Error(codes.InvalidArgument, "port can not be zero")
	}
	mw, err := proxyMiddleware(req.GetHost(), req.GetPort())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid host and port combination")
	}
	restapiv2.AddPlugin(req.GetRoute()).Use(mw)
	return &inx.NoParams{}, nil
}

func (s *INXServer) UnregisterAPIRoute(_ context.Context, req *inx.APIRouteRequest) (*inx.NoParams, error) {
	if len(req.GetRoute()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "route can not be empty")
	}
	restapiv2.RemovePlugin(req.GetRoute())
	return &inx.NoParams{}, nil
}
