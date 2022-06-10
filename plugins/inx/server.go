package inx

import (
	"context"
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

	var pendingProtoParas []*inx.PendingProtocolParameters
	for _, ele := range deps.ProtocolManager.Pending() {
		pendingProtoParas = append(pendingProtoParas, &inx.PendingProtocolParameters{
			TargetMilestoneIndex: ele.TargetMilestoneIndex,
			Version:              uint32(ele.ProtocolVersion),
			Params:               ele.Params,
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
		PendingProtocolParameters: pendingProtoParas,
	}, nil
}
