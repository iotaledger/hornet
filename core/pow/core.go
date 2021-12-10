package pow

import (
	"context"

	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/configuration"
)

func init() {
	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
			Name:     "PoW",
			DepsFunc: func(cDeps dependencies) { deps = cDeps },
			Params:   params,
			Provide:  provide,
			Run:      run,
		},
	}
}

var (
	CorePlugin *node.CorePlugin
	deps       dependencies
)

type dependencies struct {
	dig.In
	Handler *pow.Handler
}

func provide(c *dig.Container) {

	type handlerDeps struct {
		dig.In
		NodeConfig  *configuration.Configuration `name:"nodeConfig"`
		MinPoWScore float64                      `name:"minPoWScore"`
	}

	if err := c.Provide(func(deps handlerDeps) *pow.Handler {
		// init the pow handler with all possible settings
		return pow.New(deps.MinPoWScore, deps.NodeConfig.Duration(CfgPoWRefreshTipsInterval))
	}); err != nil {
		CorePlugin.LogPanic(err)
	}
}

func run() {

	// close the PoW handler on shutdown
	if err := CorePlugin.Daemon().BackgroundWorker("PoW Handler", func(ctx context.Context) {
		CorePlugin.LogInfo("Starting PoW Handler ... done")
		<-ctx.Done()
		CorePlugin.LogInfo("Stopping PoW Handler ...")
		CorePlugin.LogInfo("Stopping PoW Handler ... done")
	}, shutdown.PriorityPoWHandler); err != nil {
		CorePlugin.LogPanicf("failed to start worker: %s", err)
	}
}
