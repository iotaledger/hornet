package pow

import (
	"time"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/config"
	powpackage "github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/utils"
)

const (
	powsrvInitCooldown = 30 * time.Second
)

var (
	CoreModule *node.CoreModule
	log        *logger.Logger
	deps       dependencies
)

type dependencies struct {
	dig.In
	Handler *powpackage.Handler
}

func init() {
	CoreModule = node.NewCoreModule("PoW", configure, run)
	CoreModule.Events.Init.Attach(events.NewClosure(func(c *dig.Container) {
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
			return powpackage.New(log, deps.NodeConfig.Int(config.CfgCoordinatorMWM), powsrvAPIKey, powsrvInitCooldown)
		}); err != nil {
			panic(err)
		}
	}))
}

func configure(c *dig.Container) {
	log = logger.NewLogger(CoreModule.Name)

	if err := c.Invoke(func(cDeps dependencies) {
		deps = cDeps
	}); err != nil {
		panic(err)
	}
}

func run(_ *dig.Container) {

	// close the PoW handler on shutdown
	CoreModule.Daemon().BackgroundWorker("PoW Handler", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting PoW Handler ... done")
		<-shutdownSignal
		log.Info("Stopping PoW Handler ...")
		deps.Handler.Close()
		log.Info("Stopping PoW Handler ... done")
	}, shutdown.PriorityPoWHandler)
}
