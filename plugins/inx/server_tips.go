package inx

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotaledger/hive.go/core/timeutil"
	inx "github.com/iotaledger/inx/go"
	iotago "github.com/iotaledger/iota.go/v3"
)

func (s *Server) RequestTips(ctx context.Context, req *inx.TipsRequest) (*inx.TipsResponse, error) {
	if deps.TipSelector == nil {
		return nil, status.Error(codes.Unavailable, "no tipselector available")
	}

	var err error
	var tips iotago.BlockIDs
	if req.AllowSemiLazy {
		tips, err = deps.TipSelector.SelectTipsWithSemiLazyAllowed()
	} else {
		tips, err = deps.TipSelector.SelectNonLazyTips()
	}

	if req.GetCount() > 0 && req.GetCount() < uint32(len(tips)) {
		tips = tips[:req.GetCount()]
	}

	if err != nil {
		err = fmt.Errorf("error selecting tips: %w", err)
		Plugin.LogError(err.Error())

		return nil, status.Error(codes.Unavailable, err.Error())
	}

	return &inx.TipsResponse{
		Tips: inx.NewBlockIds(tips),
	}, nil
}

func (s *Server) ListenToTipsMetrics(req *inx.TipsMetricRequest, srv inx.INX_ListenToTipsMetricsServer) error {
	if req.GetIntervalInMilliseconds() == 0 {
		return status.Error(codes.InvalidArgument, "interval must be > 0")
	}

	if deps.TipSelector == nil {
		return status.Error(codes.Unavailable, "no tipselector available")
	}

	var innerErr error
	ctx, cancel := context.WithCancel(Plugin.Daemon().ContextStopped())
	defer cancel()

	ticker := timeutil.NewTicker(func() {
		nonLazy, semiLazy := deps.TipSelector.TipCount()

		metrics := &inx.TipsMetric{
			NonLazyPoolSize:  uint32(nonLazy),
			SemiLazyPoolSize: uint32(semiLazy),
		}

		if err := srv.Send(metrics); err != nil {
			Plugin.LogErrorf("send error: %v", err)
			innerErr = err
			cancel()
		}

	}, time.Duration(req.GetIntervalInMilliseconds())*time.Millisecond, ctx)
	ticker.WaitForShutdown()

	return innerErr
}
