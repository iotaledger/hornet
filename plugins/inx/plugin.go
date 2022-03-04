package inx

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"go.uber.org/dig"

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
	hiveutils "github.com/iotaledger/hive.go/kvstore/utils"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusDisabled,
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
	INXPath = "inx"
)

var (
	Plugin   *node.Plugin
	deps     dependencies
	attacher *tangle.MessageAttacher
	server   *INXServer

	extensions []*Extension

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
	server = newINXServer()
	loadExtensions()
}

func run() {
	if err := Plugin.Daemon().BackgroundWorker("INX", func(ctx context.Context) {
		Plugin.LogInfo("Starting INX ... done")
		server.Start()
		startExtensions()
		<-ctx.Done()
		stopExtensions()
		server.Stop()
		Plugin.LogInfo("Stopping INX ... done")
	}, shutdown.PriorityIndexer); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
}

func loadExtensions() {
	extensions = make([]*Extension, 0)

	dirExists, err := hiveutils.PathExists(INXPath)
	if err != nil {
		return
	}
	if !dirExists {
		return
	}
	files, err := ioutil.ReadDir(INXPath)
	for _, f := range files {
		if f.IsDir() {
			extension, err := NewExtension(filepath.Join(INXPath, f.Name()))
			if err != nil {
				Plugin.LogErrorf("Error loading INX extension: %s", err)
				continue
			}
			extensions = append(extensions, extension)
		}
	}
}

func startExtensions() {
	for _, e := range extensions {
		go func() {
			Plugin.LogInfof("Starting INX extension: %s", e.Name)
			err := e.Start()
			if err != nil {
				Plugin.LogErrorf("INX extension ended with error: %s", err)
			}
		}()
	}
}

func stopExtensions() {
	for _, e := range extensions {
		Plugin.LogInfof("Stopping INX extension: %s", e.Name)
		if err := e.Stop(); err != nil {
			Plugin.LogErrorf("INX extension stop error: %s", err)
			Plugin.LogInfof("Killing INX extension: %s", e.Name)
			e.Kill()
		}
	}
}
