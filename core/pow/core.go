package pow

import (
	"sync"
	"time"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/logger"

	"github.com/gohornet/hornet/pkg/config"
	powpackage "github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/utils"
)

const (
	powsrvInitCooldown = 30 * time.Second
)

var (
	CoreModule  *node.CoreModule
	log         *logger.Logger
	handler     *powpackage.Handler
	handlerOnce sync.Once
)

// Handler gets the pow handler instance.
func Handler() *powpackage.Handler {
	handlerOnce.Do(func() {
		// init the pow handler with all possible settings
		powsrvAPIKey, err := utils.LoadStringFromEnvironment("POWSRV_API_KEY")
		if err != nil && len(powsrvAPIKey) > 12 {
			powsrvAPIKey = powsrvAPIKey[:12]
		}
		handler = powpackage.New(log, config.NodeConfig.Int(config.CfgCoordinatorMWM), powsrvAPIKey, powsrvInitCooldown)

	})
	return handler
}

func init() {
	CoreModule = node.NewCoreModule("PoW", configure, run)
}

func configure(coreModule *node.CoreModule) {
	log = logger.NewLogger(coreModule.Name)

	// init pow handler
	Handler()
}

func run(_ *node.CoreModule) {

	// close the PoW handler on shutdown
	CoreModule.Daemon().BackgroundWorker("PoW Handler", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting PoW Handler ... done")
		<-shutdownSignal
		log.Info("Stopping PoW Handler ...")
		Handler().Close()
		log.Info("Stopping PoW Handler ... done")
	}, shutdown.PriorityPoWHandler)
}
