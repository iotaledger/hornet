package gossip

import (
	"github.com/gohornet/hornet/packages/autopeering/services"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/events"
)

// sets up the glue code between the autopeering module and Hornet:
// Incoming: Whitelist the peer and don't initiate any connection
// Outgoing: Put the neighbor into the reconnect pool with the autopeering info
func configureAutopeering() {
	apLog := gossipLogger.Named("Autopeering")
	// called whenever the autopeering logic wants to drop a neighborhood peer
	selection.Events.Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		apLog.Infof("[dropped event] trying to remove connection to %s", ev.DroppedID)
		neighborsLock.Lock()
		var selected *Neighbor
		// search for the connected neighbor and close the connection
		for _, neighbor := range connectedNeighbors {
			if neighbor.Autopeering == nil || neighbor.Autopeering.ID() != ev.DroppedID {
				continue
			}
			selected = neighbor
			break
		}
		defer neighborsLock.Unlock()

		if selected == nil {
			apLog.Warnf("didn't find autopeered neighbor %s for removal", ev.DroppedID)
			return
		}

		apLog.Infof("removing autopeered neighbor %s", selected.InitAddress.String())
		if err := RemoveNeighbor(selected.Identity); err != nil {
			apLog.Errorf("couldn't remove autopeered neighbor %s: %s", selected.InitAddress.String(), err)
		} else {
			apLog.Errorf("disconnected autopeered neighbor %s: %s", selected.InitAddress.String(), err)
		}
	}))
	selection.Events.IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		gossipAddr := ev.Peer.Services().Get(services.GossipServiceKey()).String()
		apLog.Infof("[incoming peering] whitelisting %s / %s / %s", ev.Peer.Address(), gossipAddr, ev.Peer.ID())
		// whitelist the given peer
		neighborsLock.Lock()
		defer neighborsLock.Unlock()
		// will be grabbed later by the incoming connection
		allowedIdentities[gossipAddr] = ev.Peer
		hostsBlacklistLock.Lock()
		delete(hostsBlacklist, gossipAddr)
		hostsBlacklistLock.Unlock()
	}))
	selection.Events.OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		gossipAddr := ev.Peer.Services().Get(services.GossipServiceKey()).String()
		apLog.Infof("[outgoing peering] adding autopeering neighbor %s / %s / %s", ev.Peer.Address(), gossipAddr, ev.Peer.ID())
		if err := AddNeighbor(gossipAddr, false, "", ev.Peer); err != nil {
			apLog.Warnf("couldn't add autopeering neighbor %s", err)
		}
	}))
}
