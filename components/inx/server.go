package inx

import (
	"context"
	"net"
	"time"

	grpcprometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/workerpool"
	"github.com/iotaledger/hornet/v2/pkg/common"
	inx "github.com/iotaledger/inx/go"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	workerCount = 1
)

func newServer() *Server {
	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(grpcprometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpcprometheus.UnaryServerInterceptor),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    20 * time.Second,
			Timeout: 5 * time.Second,
		}),
		grpc.MaxConcurrentStreams(10),
	)

	s := &Server{grpcServer: grpcServer}
	inx.RegisterINXServer(grpcServer, s)

	return s
}

type Server struct {
	inx.UnimplementedINXServer
	grpcServer *grpc.Server
}

func (s *Server) ConfigurePrometheus() {
	grpcprometheus.Register(s.grpcServer)
}

func (s *Server) Start() {
	go func() {
		listener, err := net.Listen("tcp", ParamsINX.BindAddress)
		if err != nil {
			Component.LogFatalfAndExit("failed to listen: %v", err)
		}
		defer listener.Close()

		if err := s.grpcServer.Serve(listener); err != nil {
			Component.LogFatalfAndExit("failed to serve: %v", err)
		}
	}()
}

func (s *Server) Stop() {
	s.grpcServer.Stop()
}

func currentNodeStatus() (*inx.NodeStatus, error) {
	snapshotInfo := deps.Storage.SnapshotInfo()
	if snapshotInfo == nil {
		return nil, common.ErrSnapshotInfoNotFound
	}

	pruningIndex := snapshotInfo.PruningIndex()

	index, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, err
	}

	syncState := deps.SyncManager.SyncState()

	var lmi *inx.Milestone
	if syncState.LatestMilestoneIndex > pruningIndex {
		lmi, err = milestoneForStoredMilestone(syncState.LatestMilestoneIndex)
		if err != nil {
			return nil, err
		}
	} else {
		lmi = &inx.Milestone{
			MilestoneInfo: &inx.MilestoneInfo{
				MilestoneIndex: syncState.LatestMilestoneIndex,
			},
			Milestone: nil,
		}
	}

	var cmi *inx.Milestone
	if syncState.ConfirmedMilestoneIndex > pruningIndex {
		cmi, err = milestoneForStoredMilestone(syncState.ConfirmedMilestoneIndex)
		if err != nil {
			return nil, err
		}
	} else {
		cmi = &inx.Milestone{
			MilestoneInfo: &inx.MilestoneInfo{
				MilestoneIndex: syncState.ConfirmedMilestoneIndex,
			},
			Milestone: nil,
		}
	}

	protocolParams, err := rawProtocolParametersForIndex(syncState.ConfirmedMilestoneIndex)
	if err != nil {
		return nil, err
	}

	return &inx.NodeStatus{
		IsHealthy:                 deps.Tangle.IsNodeHealthy(syncState),
		IsSynced:                  syncState.NodeSynced,
		IsAlmostSynced:            syncState.NodeAlmostSynced,
		LatestMilestone:           lmi,
		ConfirmedMilestone:        cmi,
		CurrentProtocolParameters: protocolParams,
		TanglePruningIndex:        pruningIndex,
		MilestonesPruningIndex:    pruningIndex,
		LedgerPruningIndex:        pruningIndex,
		LedgerIndex:               index,
	}, nil
}

func (s *Server) ReadNodeStatus(context.Context, *inx.NoParams) (*inx.NodeStatus, error) {
	return currentNodeStatus()
}

func (s *Server) ListenToNodeStatus(req *inx.NodeStatusRequest, srv inx.INX_ListenToNodeStatusServer) error {
	ctx, cancel := context.WithCancel(Component.Daemon().ContextStopped())

	lastSent := time.Time{}
	sendStatus := func(status *inx.NodeStatus) {
		if err := srv.Send(status); err != nil {
			Component.LogErrorf("send error: %v", err)
			cancel()

			return
		}
		lastSent = time.Now()
	}

	var lastUpdateTimer *time.Timer
	coolDownDuration := time.Duration(req.GetCooldownInMilliseconds()) * time.Millisecond
	wp := workerpool.New("ListenToNodeStatus", workerCount)

	onIndexChange := func(_ iotago.MilestoneIndex) {
		status, err := currentNodeStatus()
		if err != nil {
			Component.LogErrorf("error creating inx.NodeStatus: %s", err.Error())

			return
		}

		if lastUpdateTimer != nil {
			lastUpdateTimer.Stop()
			lastUpdateTimer = nil
		}

		// Use cooldown if the node is syncing
		if coolDownDuration > 0 && !status.IsAlmostSynced {
			timeSinceLastSent := time.Since(lastSent)
			if timeSinceLastSent < coolDownDuration {
				lastUpdateTimer = time.AfterFunc(coolDownDuration-timeSinceLastSent, func() {
					sendStatus(status)
				})

				return
			}
		}

		sendStatus(status)
	}

	wp.Start()
	unhook := lo.Batch(
		deps.Tangle.Events.LatestMilestoneIndexChanged.Hook(onIndexChange, event.WithWorkerPool(wp)).Unhook,
		deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Hook(onIndexChange, event.WithWorkerPool(wp)).Unhook,
		deps.PruningManager.Events.PruningMilestoneIndexChanged.Hook(onIndexChange, event.WithWorkerPool(wp)).Unhook,
	)

	<-ctx.Done()
	unhook()

	// We need to wait until all tasks are done, otherwise we might call
	// "SendMsg" and "CloseSend" in parallel on the grpc stream, which is
	// not safe according to the grpc docs.
	wp.Shutdown()
	wp.ShutdownComplete.Wait()

	return ctx.Err()
}

func (s *Server) ReadNodeConfiguration(context.Context, *inx.NoParams) (*inx.NodeConfiguration, error) {
	keyRanges := deps.KeyManager.KeyRanges()
	inxKeyRanges := make([]*inx.MilestoneKeyRange, len(keyRanges))
	for i, r := range keyRanges {
		inxKeyRanges[i] = &inx.MilestoneKeyRange{
			PublicKey:  r.PublicKey[:],
			StartIndex: r.StartIndex,
			EndIndex:   r.EndIndex,
		}
	}

	return &inx.NodeConfiguration{
		MilestonePublicKeyCount: uint32(deps.MilestonePublicKeyCount),
		MilestoneKeyRanges:      inxKeyRanges,
		BaseToken: &inx.BaseToken{
			Name:            deps.BaseToken.Name,
			TickerSymbol:    deps.BaseToken.TickerSymbol,
			Unit:            deps.BaseToken.Unit,
			Subunit:         deps.BaseToken.Subunit,
			Decimals:        deps.BaseToken.Decimals,
			UseMetricPrefix: deps.BaseToken.UseMetricPrefix,
		},
		SupportedProtocolVersions: deps.ProtocolManager.SupportedVersions(),
	}, nil
}

func (s *Server) ReadProtocolParameters(_ context.Context, req *inx.MilestoneRequest) (*inx.RawProtocolParameters, error) {

	msIndex := req.GetMilestoneIndex()

	// If a milestoneId was passed, use that instead
	if req.GetMilestoneId() != nil {
		cachedMilestone := deps.Storage.CachedMilestoneOrNil(req.GetMilestoneId().Unwrap()) // milestone +1
		if cachedMilestone == nil {
			return nil, status.Error(codes.NotFound, "milestone not found")
		}
		defer cachedMilestone.Release(true)
		msIndex = cachedMilestone.Milestone().Index()
	}

	// If requested no index, use the confirmed milestone index
	if msIndex == 0 {
		msIndex = deps.SyncManager.ConfirmedMilestoneIndex()
	}

	return rawProtocolParametersForIndex(msIndex)
}

type streamRange struct {
	start    iotago.MilestoneIndex
	end      iotago.MilestoneIndex
	lastSent iotago.MilestoneIndex
}

// tells whether the stream range has a range requested.
func (stream *streamRange) rangeRequested() bool {
	return stream.start > 0
}

// tells whether the stream is bounded, aka has an end index.
func (stream *streamRange) isBounded() bool {
	return stream.end > 0
}
