package p2pdisc

import (
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.Disabled,
		Pluggable: node.Pluggable{
			Name:      "P2PDiscovery",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *node.Plugin
	log    *logger.Logger
	deps   dependencies
)

type dependencies struct {
	dig.In
	DiscoveryService *p2p.DiscoveryService
	NodeConfig       *configuration.Configuration `name:"nodeConfig"`
}

func provide(c *dig.Container) {
	type discdeps struct {
		dig.In
		Host       host.Host
		Manager    *p2p.Manager
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}
	if err := c.Provide(func(deps discdeps) *p2p.DiscoveryService {
		return p2p.NewDiscoveryService(deps.Host, deps.Manager,
			p2p.WithDiscoveryServiceAdvertiseInterval(deps.NodeConfig.Duration(CfgP2PDiscAdvertiseInterval)),
			p2p.WithDiscoveryServiceRendezvousPoint(deps.NodeConfig.String(CfgP2PDiscRendezvousPoint)),
			p2p.WithDiscoveryServiceMaxDiscoveredPeers(deps.NodeConfig.Int(CfgP2PDiscMaxDiscoveredPeerConns)),
			p2p.WithDiscoveryServiceLogger(logger.NewLogger("P2P-Discovery")),
			p2p.WithDiscoveryServiceRoutingRefreshPeriod(deps.NodeConfig.Duration(CfgP2PDiscRoutingTableRefreshPeriod)),
		)
	}); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(Plugin.Name)
}

func run() {
	_ = Plugin.Daemon().BackgroundWorker("P2PDiscovery", func(shutdownSignal <-chan struct{}) {
		rendezvousPoint := deps.NodeConfig.String(CfgP2PDiscRendezvousPoint)
		discoveryInterval := deps.NodeConfig.Duration(CfgP2PDiscAdvertiseInterval)
		log.Infof("started peer discovery task with %s interval using '%s' as rendezvous point", discoveryInterval.Truncate(time.Millisecond), rendezvousPoint)
		deps.DiscoveryService.Start(shutdownSignal)
	}, shutdown.PriorityPeerDiscovery)
}
