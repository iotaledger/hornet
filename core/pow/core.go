package pow

import (
	"time"

	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/utils"
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

const (
	powsrvInitCooldown = 30 * time.Second
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
		powsrvAPIKey, err := utils.LoadStringFromEnvironment("POWSRV_API_KEY")
		if err == nil && len(powsrvAPIKey) > 12 {
			powsrvAPIKey = powsrvAPIKey[:12]
		}
		return pow.New(CorePlugin.Logger(), deps.MinPoWScore, deps.NodeConfig.Duration(CfgPoWRefreshTipsInterval), powsrvAPIKey, powsrvInitCooldown)
	}); err != nil {
		CorePlugin.Panic(err)
	}
}

func run() {

	// close the PoW handler on shutdown
	if err := CorePlugin.Daemon().BackgroundWorker("PoW Handler", func(shutdownSignal <-chan struct{}) {
		CorePlugin.LogInfo("Starting PoW Handler ... done")
		<-shutdownSignal
		CorePlugin.LogInfo("Stopping PoW Handler ...")
		deps.Handler.Close()
		CorePlugin.LogInfo("Stopping PoW Handler ... done")
	}, shutdown.PriorityPoWHandler); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}
}
