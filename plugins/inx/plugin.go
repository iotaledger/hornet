package inx

import (
	"context"
	"go.uber.org/dig"
	"time"

	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/restapi"
	"github.com/iotaledger/hive.go/configuration"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusEnabled,
		Pluggable: node.Pluggable{
			Name:      "INX",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Configure: configure,
			Run:       run,
		},
	}
}

const (
	//TODO: add config param
	INXPort = 9029
)

var (
	Plugin   *node.Plugin
	deps     dependencies
	attacher *tangle.MessageAttacher
	server   *INXServer

	messageProcessedTimeout = 1 * time.Second
)

type dependencies struct {
	dig.In
	NodeConfig                *configuration.Configuration `name:"nodeConfig"`
	SyncManager               *syncmanager.SyncManager
	UTXOManager               *utxo.Manager
	Tangle                    *tangle.Tangle
	Storage                   *storage.Storage
	Bech32HRP                 iotago.NetworkPrefix `name:"bech32HRP"`
	ShutdownHandler           *shutdown.ShutdownHandler
	TipSelector               *tipselect.TipSelector `optional:"true"`
	MinPoWScore               float64                `name:"minPoWScore"`
	DeserializationParameters *iotago.DeSerializationParameters
	PoWHandler                *pow.Handler
}

func configure() {
	attacher = deps.Tangle.MessageAttacher(deps.TipSelector, deps.MinPoWScore, messageProcessedTimeout, deps.DeserializationParameters)

	//TODO: add separate config params
	if deps.NodeConfig.Bool(restapi.CfgRestAPIPoWEnabled) {
		attacher = attacher.WithPoW(deps.PoWHandler, deps.NodeConfig.Int(restapi.CfgRestAPIPoWWorkerCount))
	}

	server = newINXServer()
}

func run() {
	if err := Plugin.Daemon().BackgroundWorker("INX", func(ctx context.Context) {
		Plugin.LogInfo("Starting INX ... done")
		server.Start()
		<-ctx.Done()
		server.Stop()
		Plugin.LogInfo("Stopping INX ... done")
	}, shutdown.PriorityIndexer); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
}
