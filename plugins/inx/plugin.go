package inx

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/pow"
	restapipkg "github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/restapi"
	"github.com/iotaledger/hive.go/configuration"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusDisabled,
		Pluggable: node.Pluggable{
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
	Plugin   *node.Plugin
	deps     dependencies
	attacher *tangle.MessageAttacher

	messageProcessedTimeout = 1 * time.Second
)

type dependencies struct {
	dig.In
	NodeConfig                *configuration.Configuration `name:"nodeConfig"`
	SyncManager               *syncmanager.SyncManager
	UTXOManager               *utxo.Manager
	Tangle                    *tangle.Tangle
	TipScoreCalculator        *tangle.TipScoreCalculator
	Storage                   *storage.Storage
	NetworkIDName             string               `name:"networkIdName"`
	Bech32HRP                 iotago.NetworkPrefix `name:"bech32HRP"`
	ShutdownHandler           *shutdown.ShutdownHandler
	TipSelector               *tipselect.TipSelector `optional:"true"`
	MinPoWScore               float64                `name:"minPoWScore"`
	DeserializationParameters *iotago.DeSerializationParameters
	PoWHandler                *pow.Handler
	INXServer                 *INXServer
	Echo                      *echo.Echo                 `optional:"true"`
	RestPluginManager         *restapi.RestPluginManager `optional:"true"`
	ExternalMetricsProxy      *restapipkg.DynamicProxy   `name:"externalMetricsProxy" optional:"true"`
}

func provide(c *dig.Container) {
	if err := c.Provide(func() *INXServer {
		return newINXServer()
	}); err != nil {
		Plugin.LogPanic(err)
	}
}

func configure() {

	attacherOpts := []tangle.MessageAttacherOption{
		tangle.WithTimeout(messageProcessedTimeout),
		tangle.WithDeserializationParameters(deps.DeserializationParameters),
		tangle.WithMinPoWScore(deps.MinPoWScore),
	}
	if deps.TipSelector != nil {
		attacherOpts = append(attacherOpts, tangle.WithTipSel(deps.TipSelector.SelectNonLazyTips))
	}
	if deps.NodeConfig.Bool(restapi.CfgRestAPIPoWEnabled) {
		attacherOpts = append(attacherOpts, tangle.WithPoW(deps.PoWHandler, deps.NodeConfig.Int(restapi.CfgRestAPIPoWWorkerCount)))
	}

	attacher = deps.Tangle.MessageAttacher(attacherOpts...)
}

func run() {
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
}
