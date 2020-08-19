package pow

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/config"
	powpackage "github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/shutdown"
)

const (
	powsrvInitCooldown = 30 * time.Second
)

var (
	PLUGIN      = node.NewPlugin("PoW", node.Enabled, configure, run)
	log         *logger.Logger
	handler     *powpackage.Handler
	handlerOnce sync.Once
)

// Handler gets the pow handler instance.
func Handler() *powpackage.Handler {
	handlerOnce.Do(func() {
		// init the pow handler with all possible settings
		powsrvAPIKey, _ := config.LoadHashFromEnvironment("POWSRV_API_KEY", 12)
		handler = powpackage.New(log, powsrvAPIKey, powsrvInitCooldown)

	})
	return handler
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	// init pow handler
	Handler()
}

func run(_ *node.Plugin) {

	// close the PoW handler on shutdown
	daemon.BackgroundWorker("PoW Handler", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting PoW Handler ... done")
		<-shutdownSignal
		log.Info("Stopping PoW Handler ...")
		Handler().Close()
		log.Info("Stopping PoW Handler ... done")
	}, shutdown.PriorityPoWHandler)
}
