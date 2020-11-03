package p2pdisc

import (
	"sync"
	"time"

	p2pcore "github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/logger"
)

var (
	Plugin               *node.Plugin
	log                  *logger.Logger
	discoveryServiceOnce sync.Once
	discoveryService     *p2p.DiscoveryService
)

// DiscoveryService returns the DiscoveryService instance.
func DiscoveryService() *p2p.DiscoveryService {
	discoveryServiceOnce.Do(func() {
		rendezvousPoint := config.NodeConfig.String(config.CfgP2PDiscRendezvousPoint)
		discoveryIntervalSec := config.NodeConfig.Duration(config.CfgP2PDiscAdvertiseIntervalSec) * time.Second
		routingTableRefreshPeriodSec := config.NodeConfig.Duration(config.CfgP2PDiscRoutingTableRefreshPeriodSec) * time.Second
		maxDiscoveredPeerCount := config.NodeConfig.Int(config.CfgP2PDiscMaxDiscoveredPeerConns)

		discoveryService = p2p.NewDiscoveryService(p2pcore.Host(), p2pcore.Manager(),
			p2p.WithDiscoveryServiceAdvertiseInterval(discoveryIntervalSec),
			p2p.WithDiscoveryServiceRendezvousPoint(rendezvousPoint),
			p2p.WithDiscoveryServiceMaxDiscoveredPeers(maxDiscoveredPeerCount),
			p2p.WithDiscoveryServiceLogger(logger.NewLogger("P2P-Discovery")),
			p2p.WithDiscoveryServiceRoutingRefreshPeriod(routingTableRefreshPeriodSec),
		)
	})
	return discoveryService
}

func init() {
	Plugin = node.NewPlugin("P2PDiscovery", node.Disabled, configure, run)
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)
	DiscoveryService()
}

func run(_ *node.Plugin) {
	_ = Plugin.Daemon().BackgroundWorker("P2PDiscovery", func(shutdownSignal <-chan struct{}) {
		rendezvousPoint := config.NodeConfig.String(config.CfgP2PDiscRendezvousPoint)
		discoveryIntervalSec := config.NodeConfig.Duration(config.CfgP2PDiscAdvertiseIntervalSec) * time.Second
		log.Infof("started peer discovery task with %d secs interval using '%s' as rendezvous point", discoveryIntervalSec, rendezvousPoint)
		DiscoveryService().Start(shutdownSignal)
	}, shutdown.PriorityPeerDiscovery)
}
