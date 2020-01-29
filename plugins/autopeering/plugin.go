package autopeering

import (
	"time"

	"github.com/gohornet/hornet/packages/parameter"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

func init() {
	// 2 inbound/outbound
	selection.SetParameters(selection.Parameters{
		InboundNeighborSize:        2,
		OutboundNeighborSize:       2,
		SaltLifetime:               30 * time.Minute,
		OutboundUpdateInterval:     1 * time.Minute,
		FullOutboundUpdateInterval: 1 * time.Minute,
	})
}

const name = "Autopeering" // name of the plugin

var PLUGIN = node.NewPlugin(name, node.Enabled, configure, run)
var GossipServiceKey service.Key

func configure(*node.Plugin) {
	GossipServiceKey = service.Key(parameter.NodeConfig.GetString("milestones.coordinator")[:10])
	log = logger.NewLogger(name)
	configureEvents()
	configureAP()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker(name, start, shutdown.ShutdownPriorityAutopeering); err != nil {
		log.Errorf("Failed to start as daemon: %s", err)
	}
}

func configureEvents() {
	// notify the selection when a connection is closed or failed.
	gossip.Events.NeighborConnectionClosed.Attach(events.NewClosure(func(neighbor *gossip.Neighbor) {
		// check whether autopeered neighbor
		if neighbor.Autopeering == nil {
			return
		}
		Selection.RemoveNeighbor(neighbor.Autopeering.ID())
	}))

	discover.Events.PeerDiscovered.Attach(events.NewClosure(func(ev *discover.DiscoveredEvent) {
		log.Infof("Discovered: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))
	discover.Events.PeerDeleted.Attach(events.NewClosure(func(ev *discover.DeletedEvent) {
		log.Infof("Removed offline: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))

	selection.Events.SaltUpdated.Attach(events.NewClosure(func(ev *selection.SaltUpdatedEvent) {
		log.Infof("Salt updated; expires=%s", ev.Public.GetExpiration().Format(time.RFC822))
	}))
	selection.Events.OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if ev.Status {
			log.Infof("Peering chosen: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}))
	selection.Events.IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if ev.Status {
			log.Infof("Peering accepted: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}))
	selection.Events.Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		log.Infof("Peering dropped: %s", ev.DroppedID.String())
	}))
}
