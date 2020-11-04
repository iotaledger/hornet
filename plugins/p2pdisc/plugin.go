package p2pdisc

import (
	"time"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"
	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/dig"
)

func init() {
	Plugin = &node.Plugin{
		Name:      "P2PDiscovery",
		DepsFunc:  func(cDeps dependencies) { deps = cDeps },
		Provide:   provide,
		Configure: configure,
		Run:       run,
		Status:    node.Disabled,
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
		rendezvousPoint := deps.NodeConfig.String(config.CfgP2PDiscRendezvousPoint)
		discoveryIntervalSec := deps.NodeConfig.Duration(config.CfgP2PDiscAdvertiseIntervalSec) * time.Second
		routingTableRefreshPeriodSec := deps.NodeConfig.Duration(config.CfgP2PDiscRoutingTableRefreshPeriodSec) * time.Second
		maxDiscoveredPeerCount := deps.NodeConfig.Int(config.CfgP2PDiscMaxDiscoveredPeerConns)

		return p2p.NewDiscoveryService(deps.Host, deps.Manager,
			p2p.WithDiscoveryServiceAdvertiseInterval(discoveryIntervalSec),
			p2p.WithDiscoveryServiceRendezvousPoint(rendezvousPoint),
			p2p.WithDiscoveryServiceMaxDiscoveredPeers(maxDiscoveredPeerCount),
			p2p.WithDiscoveryServiceLogger(logger.NewLogger("P2P-Discovery")),
			p2p.WithDiscoveryServiceRoutingRefreshPeriod(routingTableRefreshPeriodSec),
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
		rendezvousPoint := deps.NodeConfig.String(config.CfgP2PDiscRendezvousPoint)
		discoveryIntervalSec := deps.NodeConfig.Duration(config.CfgP2PDiscAdvertiseIntervalSec) * time.Second
		log.Infof("started peer discovery task with %d secs interval using '%s' as rendezvous point", discoveryIntervalSec, rendezvousPoint)
		deps.DiscoveryService.Start(shutdownSignal)
	}, shutdown.PriorityPeerDiscovery)
}
