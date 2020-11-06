package pow

import (
	"time"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/node"
	powpackage "github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/plugins/coordinator"
)

func init() {
	CoreModule = &node.CoreModule{
		Name:      "PoW",
		DepsFunc:  func(cDeps dependencies) { deps = cDeps },
		Provide:   provide,
		Configure: configure,
		Run:       run,
	}
}

var (
	CoreModule *node.CoreModule
	log        *logger.Logger
	deps       dependencies
)

const (
	powsrvInitCooldown = 30 * time.Second
)

type dependencies struct {
	dig.In
	Handler *powpackage.Handler
}

func provide(c *dig.Container) {
	type handlerdeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps handlerdeps) *powpackage.Handler {
		// init the pow handler with all possible settings
		powsrvAPIKey, err := utils.LoadStringFromEnvironment("POWSRV_API_KEY")
		if err != nil && len(powsrvAPIKey) > 12 {
			powsrvAPIKey = powsrvAPIKey[:12]
		}
		return powpackage.New(log, deps.NodeConfig.Float64(coordinator.CfgCoordinatorMinPoWScore), powsrvAPIKey, powsrvInitCooldown)
	}); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(CoreModule.Name)
}

func run() {

	// close the PoW handler on shutdown
	CoreModule.Daemon().BackgroundWorker("PoW Handler", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting PoW Handler ... done")
		<-shutdownSignal
		log.Info("Stopping PoW Handler ...")
		deps.Handler.Close()
		log.Info("Stopping PoW Handler ... done")
	}, shutdown.PriorityPoWHandler)
}
