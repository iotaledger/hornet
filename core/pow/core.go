package pow

import (
	"time"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"

	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/pkg/node"
	powpackage "github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/utils"
)

const (
	// defines whether the node does PoW (e.g. if messages are received via API)
	CfgNodeEnableProofOfWork = "node.enableProofOfWork"
)

func init() {
	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
			Name:     "PoW",
			DepsFunc: func(cDeps dependencies) { deps = cDeps },
			Params: &node.PluginParams{
				Params: map[string]*flag.FlagSet{
					"nodeConfig": func() *flag.FlagSet {
						fs := flag.NewFlagSet("", flag.ContinueOnError)
						fs.Bool(CfgNodeEnableProofOfWork, false, "defines whether the node does PoW (e.g. if messages are received via API)")
						return fs
					}(),
				},
				Masked: nil,
			},
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	CorePlugin *node.CorePlugin
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
		return powpackage.New(log, deps.NodeConfig.Float64(protocfg.CfgProtocolMinPoWScore), powsrvAPIKey, powsrvInitCooldown)
	}); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(CorePlugin.Name)
}

func run() {

	// close the PoW handler on shutdown
	CorePlugin.Daemon().BackgroundWorker("PoW Handler", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting PoW Handler ... done")
		<-shutdownSignal
		log.Info("Stopping PoW Handler ...")
		deps.Handler.Close()
		log.Info("Stopping PoW Handler ... done")
	}, shutdown.PriorityPoWHandler)
}
