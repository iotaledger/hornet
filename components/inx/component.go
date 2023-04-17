package inx

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hornet/v2/components/protocfg"
	"github.com/iotaledger/hornet/v2/components/restapi"
	"github.com/iotaledger/hornet/v2/pkg/components"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/pow"
	"github.com/iotaledger/hornet/v2/pkg/protocol"
	"github.com/iotaledger/hornet/v2/pkg/pruning"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	"github.com/iotaledger/hornet/v2/pkg/tipselect"
	"github.com/iotaledger/iota.go/v3/keymanager"
)

func init() {
	Component = &app.Component{
		Name:     "INX",
		DepsFunc: func(cDeps dependencies) { deps = cDeps },
		Params:   params,
		IsEnabled: func(c *dig.Container) bool {
			// do not enable in "autopeering entry node" mode
			return components.IsAutopeeringEntryNodeDisabled(c) && ParamsINX.Enabled
		},
		Provide:   provide,
		Configure: configure,
		Run:       run,
	}
}

var (
	Component *app.Component
	deps      dependencies
	attacher  *tangle.BlockAttacher

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
	PruningManager          *pruning.Manager
	BaseToken               *protocfg.BaseToken
	PoWHandler              *pow.Handler
	INXServer               *Server
	INXMetrics              *metrics.INXMetrics
	Echo                    *echo.Echo                `optional:"true"`
	RestRouteManager        *restapi.RestRouteManager `optional:"true"`
}

func provide(c *dig.Container) error {

	if err := c.Provide(func() *metrics.INXMetrics {
		return &metrics.INXMetrics{
			Events: &metrics.INXEvents{
				PoWCompleted: event.New2[int, time.Duration](),
			},
		}
	}); err != nil {
		Component.LogPanic(err)
	}

	if err := c.Provide(func() *Server {
		return newServer()
	}); err != nil {
		Component.LogPanic(err)
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
	if err := Component.Daemon().BackgroundWorker("INX", func(ctx context.Context) {
		Component.LogInfo("Starting INX ... done")
		deps.INXServer.Start()
		<-ctx.Done()
		Component.LogInfo("Stopping INX ...")
		deps.INXServer.Stop()
		Component.LogInfo("Stopping INX ... done")
	}, daemon.PriorityIndexer); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}
