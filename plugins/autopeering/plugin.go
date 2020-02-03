package autopeering

import (
	"time"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/packages/autopeering/services"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/gossip"
)

func init() {
	// 2 inbound/outbound
	selection.SetParameters(selection.Parameters{
		InboundNeighborSize:        2,
		OutboundNeighborSize:       2,
		SaltLifetime:               30 * time.Minute,
		OutboundUpdateInterval:     30 * time.Second,
		FullOutboundUpdateInterval: 30 * time.Second,
	})
}

const name = "Autopeering" // name of the plugin

var PLUGIN = node.NewPlugin(name, node.Enabled, configure, run)

func configure(*node.Plugin) {
	services.GossipServiceKey()
	log = logger.NewLogger(name)
	configureEvents()
	configureAP()
}

func run(*node.Plugin) {
	daemon.BackgroundWorker(name, start, shutdown.ShutdownPriorityAutopeering)
}

func configureEvents() {
	// notify the selection when a connection is closed or failed.
	gossip.Events.NeighborConnectionClosed.Attach(events.NewClosure(func(neighbor *gossip.Neighbor) {
		// check whether autopeered neighbor
		if neighbor.Autopeering == nil {
			return
		}
		gossipAddr := neighbor.Autopeering.Services().Get(services.GossipServiceKey()).String()
		log.Infof("removing: %s / %s", gossipAddr, neighbor.Autopeering.ID())
		Selection.RemoveNeighbor(neighbor.Autopeering.ID())
	}))

	discover.Events.PeerDiscovered.Attach(events.NewClosure(func(ev *discover.DiscoveredEvent) {
		log.Infof("discovered: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))
	discover.Events.PeerDeleted.Attach(events.NewClosure(func(ev *discover.DeletedEvent) {
		log.Infof("removed offline: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))

	selection.Events.SaltUpdated.Attach(events.NewClosure(func(ev *selection.SaltUpdatedEvent) {
		log.Infof("salt updated; expires=%s", ev.Public.GetExpiration().Format(time.RFC822))
	}))
	selection.Events.OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if ev.Status {
			log.Infof("peering chosen: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}))
	selection.Events.IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if ev.Status {
			log.Infof("peering accepted: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}))
	selection.Events.Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		log.Infof("peering dropped: %s", ev.DroppedID.String())
	}))
}
