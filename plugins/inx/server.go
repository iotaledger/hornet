package inx

import (
	"context"
	"net"

	grpcprometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotaledger/hive.go/core/workerpool"
	"github.com/iotaledger/hornet/v2/pkg/common"
	inx "github.com/iotaledger/inx/go"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	workerCount     = 1
	workerQueueSize = 10000
)

func newINXServer() *INXServer {
	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(grpcprometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpcprometheus.UnaryServerInterceptor),
	)
	s := &INXServer{grpcServer: grpcServer}
	inx.RegisterINXServer(grpcServer, s)

	return s
}

type INXServer struct {
	inx.UnimplementedINXServer
	grpcServer *grpc.Server
}

func (s *INXServer) ConfigurePrometheus() {
	grpcprometheus.Register(s.grpcServer)
}

func (s *INXServer) Start() {
	go func() {
		lis, err := net.Listen("tcp", ParamsINX.BindAddress)
		if err != nil {
			Plugin.LogFatalfAndExit("failed to listen: %v", err)
		}
		defer lis.Close()

		if err := s.grpcServer.Serve(lis); err != nil {
			Plugin.LogFatalfAndExit("failed to serve: %v", err)
		}
	}()
}

func (s *INXServer) Stop() {
	s.grpcServer.Stop()
}

func (s *INXServer) ReadNodeStatus(context.Context, *inx.NoParams) (*inx.NodeStatus, error) {

	snapshotInfo := deps.Storage.SnapshotInfo()
	if snapshotInfo == nil {
		return nil, common.ErrSnapshotInfoNotFound
	}

	pruningIndex := snapshotInfo.PruningIndex()

	index, err := deps.UTXOManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return nil, err
	}

	latestMilestoneIndex := deps.SyncManager.LatestMilestoneIndex()
	var lmi *inx.Milestone
	if latestMilestoneIndex > pruningIndex {
		lmi, err = milestoneForIndex(latestMilestoneIndex)
		if err != nil {
			return nil, err
		}
	} else {
		lmi = &inx.Milestone{
			MilestoneInfo: &inx.MilestoneInfo{
				MilestoneIndex: latestMilestoneIndex,
			},
			Milestone: nil,
		}
	}

	confirmedMilestoneIndex := deps.SyncManager.ConfirmedMilestoneIndex()
	var cmi *inx.Milestone
	if confirmedMilestoneIndex > pruningIndex {
		cmi, err = milestoneForIndex(confirmedMilestoneIndex)
		if err != nil {
			return nil, err
		}
	} else {
		cmi = &inx.Milestone{
			MilestoneInfo: &inx.MilestoneInfo{
				MilestoneIndex: confirmedMilestoneIndex,
			},
			Milestone: nil,
		}
	}

	return &inx.NodeStatus{
		IsHealthy:              deps.Tangle.IsNodeHealthy(),
		LatestMilestone:        lmi,
		ConfirmedMilestone:     cmi,
		TanglePruningIndex:     pruningIndex,
		MilestonesPruningIndex: pruningIndex,
		LedgerPruningIndex:     pruningIndex,
		LedgerIndex:            index,
	}, nil
}

func (s *INXServer) ReadNodeConfiguration(context.Context, *inx.NoParams) (*inx.NodeConfiguration, error) {
	var keyRanges []*inx.MilestoneKeyRange
	for _, r := range deps.KeyManager.KeyRanges() {
		keyRanges = append(keyRanges, &inx.MilestoneKeyRange{
			PublicKey:  r.PublicKey[:],
			StartIndex: r.StartIndex,
			EndIndex:   r.EndIndex,
		})
	}

	return &inx.NodeConfiguration{
		MilestonePublicKeyCount: uint32(deps.MilestonePublicKeyCount),
		MilestoneKeyRanges:      keyRanges,
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

func (s *INXServer) ReadProtocolParameters(_ context.Context, req *inx.MilestoneRequest) (*inx.RawProtocolParameters, error) {

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

// handles the sending of data within a streamRange.
//   - sendFunc gets executed for the given index.
//   - if data wasn't sent between streamRange.lastSent and the given index, then the given catchUpFunc is executed
//     with the range from streamRange.lastSent + 1 up to index - 1.
//   - it is the caller's job to call task.Return(...).
//   - streamRange.lastSent is auto. updated
func handleRangedSend(task *workerpool.Task, index iotago.MilestoneIndex, streamRange *streamRange,
	catchUpFunc func(start iotago.MilestoneIndex, end iotago.MilestoneIndex) error,
	sendFunc func(task *workerpool.Task, index iotago.MilestoneIndex) error,
) (bool, error) {

	// below requested range
	if streamRange.rangeRequested() && index < streamRange.start {
		return false, nil
	}

	// execute catch up function with missing indices
	if streamRange.rangeRequested() && index-1 > streamRange.lastSent {
		startIndex := streamRange.start
		if startIndex < streamRange.lastSent+1 {
			startIndex = streamRange.lastSent + 1
		}

		endIndex := index - 1
		if streamRange.isBounded() && endIndex > streamRange.end {
			endIndex = streamRange.end
		}

		if err := catchUpFunc(startIndex, endIndex); err != nil {
			return false, err
		}

		streamRange.lastSent = endIndex
	}

	// stream finished
	if streamRange.isBounded() && index > streamRange.end {
		return true, nil
	}

	if err := sendFunc(task, index); err != nil {
		return false, err
	}

	streamRange.lastSent = index

	// stream finished
	if streamRange.isBounded() && index >= streamRange.end {
		return true, nil
	}

	return false, nil
}
