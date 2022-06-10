package inx

import (
	"context"
	"github.com/iotaledger/hornet/pkg/protocol"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hornet/core/protocfg"
	"github.com/iotaledger/hornet/pkg/daemon"
	"github.com/iotaledger/hornet/pkg/keymanager"
	"github.com/iotaledger/hornet/pkg/metrics"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	"github.com/iotaledger/hornet/pkg/pow"
	"github.com/iotaledger/hornet/pkg/tangle"
	"github.com/iotaledger/hornet/pkg/tipselect"
	"github.com/iotaledger/hornet/plugins/restapi"
)

func init() {
	Plugin = &app.Plugin{
		Status: app.StatusDisabled,
		Component: &app.Component{
			Name:      "INX",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin   *app.Plugin
	deps     dependencies
	attacher *tangle.BlockAttacher

	blockProcessedTimeout = 1 * time.Second
)

type dependencies struct {
	dig.In
	SyncManager             *syncmanager.SyncManager
	UTXOManager             *utxo.Manager
	Tangle                  *tangle.Tangle
	TipScoreCalculator      *tangle.TipScoreCalculator
	Storage                 *storage.Storage
	KeyManager              *keymanager.KeyManager
	TipSelector             *tipselect.TipSelector `optional:"true"`
	MilestonePublicKeyCount int                    `name:"milestonePublicKeyCount"`
	ProtocolManager         *protocol.Manager
	BaseToken               *protocfg.BaseToken
	PoWHandler              *pow.Handler
	INXServer               *INXServer
	INXMetrics              *metrics.INXMetrics
	Echo                    *echo.Echo                 `optional:"true"`
	RestPluginManager       *restapi.RestPluginManager `optional:"true"`
}

func provide(c *dig.Container) error {

	if err := c.Provide(func() *metrics.INXMetrics {
		return &metrics.INXMetrics{
			Events: &metrics.INXEvents{
				PoWCompleted: events.NewEvent(metrics.PoWCompletedCaller),
			},
		}
	}); err != nil {
		Plugin.LogPanic(err)
	}

	if err := c.Provide(func() *INXServer {
		return newINXServer()
	}); err != nil {
		Plugin.LogPanic(err)
	}

	return nil
}

func configure() error {

	attacherOpts := []tangle.BlockAttacherOption{
		tangle.WithTimeout(blockProcessedTimeout),
		tangle.WithPoW(deps.PoWHandler, ParamsINX.PoW.WorkerCount),
		tangle.WithPoWMetrics(deps.INXMetrics),
	}
	if deps.TipSelector != nil {
		attacherOpts = append(attacherOpts, tangle.WithTipSel(deps.TipSelector.SelectNonLazyTips))
	}

	attacher = deps.Tangle.BlockAttacher(attacherOpts...)

	return nil
}

func run() error {
	if err := Plugin.Daemon().BackgroundWorker("INX", func(ctx context.Context) {
		Plugin.LogInfo("Starting INX ... done")
		deps.INXServer.Start()
		<-ctx.Done()
		Plugin.LogInfo("Stopping INX ...")
		deps.INXServer.Stop()
		Plugin.LogInfo("Stopping INX ... done")
	}, daemon.PriorityIndexer); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}
