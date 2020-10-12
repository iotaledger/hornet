package p2pdisc

import (
	"context"
	"sync"
	"time"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/shutdown"
	p2pplug "github.com/gohornet/hornet/plugins/p2p"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

var (
	PLUGIN      = node.NewPlugin("P2PDiscovery", node.Disabled, configure, run)
	log         *logger.Logger
	dhtOnce     sync.Once
	kademliaDHT *dht.IpfsDHT
)

// DHT returns the distributed hash table this plugin uses.
func DHT() *dht.IpfsDHT {
	dhtOnce.Do(func() {
		host := p2pplug.Host()
		ctx := context.Background()
		var err error
		kademliaDHT, err = dht.New(ctx, host, dht.RoutingTableRefreshPeriod(1*time.Minute), dht.Mode(dht.ModeServer))
		if err != nil {
			log.Panicf("unable to create Kademlia DHT: %s", err)
		}
	})
	return kademliaDHT
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)
	DHT()
}

func run(_ *node.Plugin) {

	dhtCtx, dhtCancel := context.WithCancel(context.Background())
	if err := kademliaDHT.Bootstrap(dhtCtx); err != nil {
		log.Errorf("unable to bootstrap Kademlia DHT: %s", err)
	}

	daemon.BackgroundWorker("KademliaDHT", func(shutdownSignal <-chan struct{}) {
		log.Info("started Kademlia DHT")
		<-shutdownSignal
		dhtCancel()
	}, shutdown.PriorityKademliaDHT)

	daemon.BackgroundWorker("P2PDiscovery", func(shutdownSignal <-chan struct{}) {

		rendezvousPoint := config.NodeConfig.GetString(config.CfgP2PDiscRendezvousPoint)
		discoveryIntervalSec := config.NodeConfig.GetInt(config.CfgP2PDiscAdvertiseIntervalSec)
		discoveryInterval := time.Duration(discoveryIntervalSec) * time.Second

		log.Infof("started peer discovery task with %d secs interval using '%s' as rendezvous point", discoveryIntervalSec, rendezvousPoint)
		timeutil.Ticker(findPeers, discoveryInterval, shutdownSignal)
	}, shutdown.PriorityPeerDiscovery)
}

// findPeers tries to find peers to connect to by advertising itself on the DHT on a given
// "rendezvous point".
// btw, this isn't the right way to do it according to also the rendezvous example
// in the libp2p repository and they refer to https://github.com/libp2p/specs/pull/56.
// however, it seems they just merged the spec without any actual implementation yet?
func findPeers() {
	/*
		// only try to find peers if we have less than what we want by the config
		maxDiscPeerCount := config.NodeConfig.GetInt(config.CfgP2PDiscMaxDiscoveredPeerConns)
		m := p2pplug.Manager().PeerCountPerConnType()

		delta := maxDiscPeerCount - m[p2ppkg.ConnTypeDiscovered]
		if delta <= 0 {
			// TODO: might be too verbose
			log.Infof("skipping peer discovery since >= %d discovered peers are connected", maxDiscPeerCount)
			return
		}

		findPeersCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
		rendezvousPoint := config.NodeConfig.GetString(config.CfgP2PDiscRendezvousPoint)
		routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)
		discovery.Advertise(findPeersCtx, routingDiscovery, rendezvousPoint)

		log.Infof("searching for other peers on rendezvous point '%s'", rendezvousPoint)
		// TODO: how long does this block etc.? docs don't tell anything
		peerChan, err := routingDiscovery.FindPeers(findPeersCtx, rendezvousPoint)
		if err != nil {
			log.Errorf("unable to find peers: %s", err)
			return
		}

		host := p2pplug.Host()
		var found int
		for peer := range peerChan {
			if delta == 0 {
				break
			}

			// apparently we can even find ourselves
			if peer.ID == host.ID() {
				continue
			}

			if p2pplug.Manager().Peer(peer.ID) != nil {
				continue
			}

			log.Infof("adding discovered peer %s to the peering service", peer.ID)
			p2pplug.Manager().AddPeer(peer, p2ppkg.ConnTypeDiscovered)
			found++
			delta--
		}
	*/
}
