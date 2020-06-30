package autopeering

import (
	"net"
	"strconv"
	"time"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/iputils"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/autopeering/services"
	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/peering"
)

var (
	PLUGIN = node.NewPlugin("Autopeering", node.Enabled, configure, run)

	log   *logger.Logger
	local *Local

	// Closures
	onDiscoveryPeerDiscovered  *events.Closure
	onDiscoveryPeerDeleted     *events.Closure
	onSelectionSaltUpdated     *events.Closure
	onManagerPeerDisconnected  *events.Closure
	onSelectionOutgoingPeering *events.Closure
	onSelectionIncomingPeering *events.Closure
	onSelectionDropped         *events.Closure
)

func configure(p *node.Plugin) {
	selection.SetParameters(selection.Parameters{
		InboundNeighborSize:        config.NodeConfig.GetInt(config.CfgNetAutopeeringInboundPeers),
		OutboundNeighborSize:       config.NodeConfig.GetInt(config.CfgNetAutopeeringOutboundPeers),
		SaltLifetime:               time.Duration(config.NodeConfig.GetInt(config.CfgNetAutopeeringSaltLifetime)) * time.Minute,
		OutboundUpdateInterval:     30 * time.Second,
		FullOutboundUpdateInterval: 30 * time.Second,
	})
	services.GossipServiceKey()
	log = logger.NewLogger(p.Name)
	local = NewLocal()
	configureAP(local)
	configureEvents()
}

func run(p *node.Plugin) {
	daemon.BackgroundWorker(p.Name, func(shutdownSignal <-chan struct{}) {
		attachEvents()
		start(local, shutdownSignal)
		detachEvents()
	}, shutdown.PriorityAutopeering)
}

func configureEvents() {

	onDiscoveryPeerDiscovered = events.NewClosure(func(ev *discover.DiscoveredEvent) {
		log.Infof("discovered: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	})

	onDiscoveryPeerDeleted = events.NewClosure(func(ev *discover.DeletedEvent) {
		log.Infof("removed offline: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	})

	onSelectionSaltUpdated = events.NewClosure(func(ev *selection.SaltUpdatedEvent) {
		log.Infof("salt updated; expires=%s", ev.Public.GetExpiration().Format(time.RFC822))
	})

	onManagerPeerDisconnected = events.NewClosure(func(p *peer.Peer) {
		if p.Autopeering == nil {
			return
		}
		gossipService := p.Autopeering.Services().Get(services.GossipServiceKey())
		gossipAddr := net.JoinHostPort(p.Autopeering.IP().String(), strconv.Itoa(gossipService.Port()))
		log.Infof("removing: %s / %s", gossipAddr, p.Autopeering.ID())
		Selection.RemoveNeighbor(p.Autopeering.ID())
	})
	onSelectionOutgoingPeering = events.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		gossipService := ev.Peer.Services().Get(services.GossipServiceKey())
		gossipAddr := net.JoinHostPort(ev.Peer.IP().String(), strconv.Itoa(gossipService.Port()))
		log.Infof("[outgoing peering] adding autopeering peer %s / %s", gossipAddr, ev.Peer.ID())

		originAddr, _ := iputils.ParseOriginAddress(gossipAddr)

		// check if the peer is already statically peered
		if peering.Manager().IsStaticallyPeered([]string{originAddr.Addr}, originAddr.Port) {
			log.Infof("peer is statically peered already %s", originAddr.String())
			log.Infof("removing: %s / %s", gossipAddr, ev.Peer.ID())
			Selection.RemoveNeighbor(ev.Peer.ID())
			return
		}

		if err := peering.Manager().Add(gossipAddr, false, "", ev.Peer); err != nil {
			log.Warnf("couldn't add autopeering peer %s", err)
		}
	})

	onSelectionIncomingPeering = events.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		gossipService := ev.Peer.Services().Get(services.GossipServiceKey())
		gossipAddr := net.JoinHostPort(ev.Peer.IP().String(), strconv.Itoa(gossipService.Port()))
		log.Infof("[incoming peering] whitelisting %s / %s", gossipAddr, ev.Peer.ID())

		// whitelist the peer
		originAddr, _ := iputils.ParseOriginAddress(gossipAddr)

		// check if the peer is already statically peered
		if peering.Manager().IsStaticallyPeered([]string{originAddr.Addr}, originAddr.Port) {
			log.Infof("peer is statically peered already %s", originAddr.String())
			log.Infof("removing: %s / %s", gossipAddr, ev.Peer.ID())
			Selection.RemoveNeighbor(ev.Peer.ID())
			return
		}
		peering.Manager().Whitelist([]string{originAddr.Addr}, originAddr.Port, ev.Peer)
	})

	onSelectionDropped = events.NewClosure(func(ev *selection.DroppedEvent) {
		log.Infof("[dropped event] trying to remove connection to %s", ev.DroppedID)

		var found *peer.Peer
		peering.Manager().ForAll(func(p *peer.Peer) bool {
			if p.Autopeering == nil || p.Autopeering.ID() != ev.DroppedID {
				return true
			}
			found = p
			return false
		})

		if found == nil {
			log.Warnf("didn't find autopeered peer %s for removal", ev.DroppedID)
			return
		}

		log.Infof("removing autopeered peer %s", found.InitAddress.String())
		if err := peering.Manager().Remove(found.ID); err != nil {
			log.Errorf("couldn't remove autopeered peer %s: %s", found.InitAddress.String(), err)
			return
		}

		log.Infof("disconnected autopeered peer %s", found.InitAddress.String())
	})
}

func attachEvents() {
	Discovery.Events().PeerDiscovered.Attach(onDiscoveryPeerDiscovered)
	Discovery.Events().PeerDeleted.Attach(onDiscoveryPeerDeleted)
	Selection.Events().SaltUpdated.Attach(onSelectionSaltUpdated)

	// only handle outgoing/incoming peering requests when the peering plugin is enabled
	if node.IsSkipped(peering.PLUGIN) {
		return
	}

	// notify the selection when a connection is closed or failed.
	peering.Manager().Events.PeerDisconnected.Attach(onManagerPeerDisconnected)
	Selection.Events().OutgoingPeering.Attach(onSelectionOutgoingPeering)
	Selection.Events().IncomingPeering.Attach(onSelectionIncomingPeering)
	Selection.Events().Dropped.Attach(onSelectionDropped)
}

func detachEvents() {
	Discovery.Events().PeerDiscovered.Detach(onDiscoveryPeerDiscovered)
	Discovery.Events().PeerDeleted.Detach(onDiscoveryPeerDeleted)
	Selection.Events().SaltUpdated.Detach(onSelectionSaltUpdated)

	// outgoing/incoming peering requests are only handle when the peering plugin is enabled
	if node.IsSkipped(peering.PLUGIN) {
		return
	}

	peering.Manager().Events.PeerDisconnected.Detach(onManagerPeerDisconnected)
	Selection.Events().OutgoingPeering.Detach(onSelectionOutgoingPeering)
	Selection.Events().IncomingPeering.Detach(onSelectionIncomingPeering)
	Selection.Events().Dropped.Detach(onSelectionDropped)
}
