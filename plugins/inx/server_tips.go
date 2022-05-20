package inx

import (
	"context"
	"time"

	iotago "github.com/iotaledger/iota.go/v3"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotaledger/hive.go/timeutil"

	inx "github.com/iotaledger/inx/go"
)

func (s *INXServer) RequestTips(ctx context.Context, req *inx.TipsRequest) (*inx.TipsResponse, error) {
	if deps.TipSelector == nil {
		return nil, status.Error(codes.Unavailable, "no tipselector available")
	}

	var err error
	var tips iotago.BlockIDs
	if req.AllowSemiLazy {
		_, tips, err = deps.TipSelector.SelectSpammerTips()
	} else {
		tips, err = deps.TipSelector.SelectNonLazyTips()
	}

	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "error selecting tips: %s", err.Error())
	}
	return &inx.TipsResponse{
		Tips: inx.NewBlockIds(tips),
	}, nil
}

func (s *INXServer) ListenToTipsMetrics(req *inx.TipsMetricRequest, srv inx.INX_ListenToTipsMetricsServer) error {
	if req.GetIntervalInMilliseconds() == 0 {
		return status.Error(codes.InvalidArgument, "interval must be > 0")
	}
	if deps.TipSelector == nil {
		return status.Error(codes.Unavailable, "no tipselector available")
	}
	var innerErr error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticker := timeutil.NewTicker(func() {
		nonLazy, semiLazy := deps.TipSelector.TipCount()
		metrics := &inx.TipsMetric{
			NonLazyPoolSize:  uint32(nonLazy),
			SemiLazyPoolSize: uint32(semiLazy),
		}
		if err := srv.Send(metrics); err != nil {
			Plugin.LogInfof("send error: %v", err)
			innerErr = err
			cancel()
		}
	}, time.Duration(req.GetIntervalInMilliseconds())*time.Millisecond, ctx)
	ticker.WaitForShutdown()
	return innerErr
}
