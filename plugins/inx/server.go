package inx

import (
	"context"
	"fmt"
	"net"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"

	"github.com/gohornet/hornet/pkg/inx"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	workerCount     = 1
	workerQueueSize = 10000
)

func newINXServer() *INXServer {
	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
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
	grpc_prometheus.Register(s.grpcServer)
}

func (s *INXServer) Start() {
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", deps.NodeConfig.Int(CfgINXPort)))
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
		lmi = &inx.Milestone{
			MilestoneIndex: uint32(latestMilestoneIndex),
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
			MilestoneIndex: uint32(confirmedMilestoneIndex),
		}
	}

	return &inx.NodeStatus{
		IsHealthy:          deps.Tangle.IsNodeHealthy(),
		LatestMilestone:    lmi,
		ConfirmedMilestone: cmi,
		PruningIndex:       uint32(pruningIndex),
		LedgerIndex:        uint32(index),
	}, nil
}

func (s *INXServer) ReadProtocolParameters(context.Context, *inx.NoParams) (*inx.ProtocolParameters, error) {
	return &inx.ProtocolParameters{
		NetworkName:     deps.NetworkIDName,
		ProtocolVersion: iotago.ProtocolVersion,
		Bech32HRP:       string(deps.Bech32HRP),
		MinPoWScore:     float32(deps.MinPoWScore),
		RentStructure: &inx.RentStructure{
			VByteCost:       deps.DeserializationParameters.RentStructure.VByteCost,
			VByteFactorData: uint64(deps.DeserializationParameters.RentStructure.VBFactorData),
			VByteFactorKey:  uint64(deps.DeserializationParameters.RentStructure.VBFactorKey),
		},
	}, nil
}
