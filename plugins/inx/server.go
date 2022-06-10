package inx

import (
	"context"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"net"

	grpcprometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"

	inx "github.com/iotaledger/inx/go"
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

func (s *INXServer) ReadNodeStatus(context.Context, *inx.NoParams) (*inx.NodeStatus, error) {
	pruningIndex := deps.Storage.SnapshotInfo().PruningIndex

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
		//TODO: we should have the milestone here when we store it in the snapshot
		lmi = &inx.Milestone{
			MilestoneInfo: &inx.MilestoneInfo{
				MilestoneIndex: uint32(latestMilestoneIndex),
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
		//TODO: we should have the milestone here when we store it in the snapshot
		cmi = &inx.Milestone{
			MilestoneInfo: &inx.MilestoneInfo{
				MilestoneIndex: uint32(confirmedMilestoneIndex),
			},
			Milestone: nil,
		}
	}

	return &inx.NodeStatus{
		IsHealthy:              deps.Tangle.IsNodeHealthy(),
		LatestMilestone:        lmi,
		ConfirmedMilestone:     cmi,
		TanglePruningIndex:     uint32(pruningIndex),
		MilestonesPruningIndex: uint32(pruningIndex),
		LedgerPruningIndex:     uint32(pruningIndex),
		LedgerIndex:            uint32(index),
	}, nil
}

func (s *INXServer) ReadNodeConfiguration(context.Context, *inx.NoParams) (*inx.NodeConfiguration, error) {
	var keyRanges []*inx.MilestoneKeyRange
	for _, r := range deps.KeyManager.KeyRanges() {
		keyRanges = append(keyRanges, &inx.MilestoneKeyRange{
			PublicKey:  r.PublicKey[:],
			StartIndex: uint32(r.StartIndex),
			EndIndex:   uint32(r.EndIndex),
		})
	}
	return &inx.NodeConfiguration{
		ProtocolParameters:      inx.NewProtocolParameters(deps.ProtocolManager.Current()),
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

type streamRange struct {
	start    milestone.Index
	end      milestone.Index
	lastSent milestone.Index
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
//	- sendFunc gets executed for the given index.
// 	- if data wasn't sent between streamRange.lastSent and the given index, then the given catchUpFunc is executed
//	 with the range from streamRange.lastSent + 1 up to index - 1.
//	- it is the caller's job to call task.Return(...).
//	- streamRange.lastSent is auto. updated
func handleRangedSend(task *workerpool.Task, index milestone.Index, streamRange *streamRange,
	catchUpFunc func(start milestone.Index, end milestone.Index) error,
	sendFunc func(task *workerpool.Task, index milestone.Index) error,
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
