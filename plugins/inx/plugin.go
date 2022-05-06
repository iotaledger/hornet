package inx

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/restapi"
	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	iotago "github.com/iotaledger/iota.go/v3"
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
	attacher *tangle.MessageAttacher

	messageProcessedTimeout = 1 * time.Second
)

type dependencies struct {
	dig.In
	AppConfig               *configuration.Configuration `name:"appConfig"`
	SyncManager             *syncmanager.SyncManager
	UTXOManager             *utxo.Manager
	Tangle                  *tangle.Tangle
	TipScoreCalculator      *tangle.TipScoreCalculator
	Storage                 *storage.Storage
	KeyManager              *keymanager.KeyManager
	TipSelector             *tipselect.TipSelector `optional:"true"`
	MilestonePublicKeyCount int                    `name:"milestonePublicKeyCount"`
	ProtocolParameters      *iotago.ProtocolParameters
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

	attacherOpts := []tangle.MessageAttacherOption{
		tangle.WithTimeout(messageProcessedTimeout),
		tangle.WithPoW(deps.PoWHandler, deps.AppConfig.Int(CfgINXPoWWorkerCount)),
		tangle.WithPoWMetrics(deps.INXMetrics),
	}
	if deps.TipSelector != nil {
		attacherOpts = append(attacherOpts, tangle.WithTipSel(deps.TipSelector.SelectNonLazyTips))
	}

	attacher = deps.Tangle.MessageAttacher(attacherOpts...)

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
	}, shutdown.PriorityIndexer); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}
