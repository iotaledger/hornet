package autopeering

import (
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	libp2p "github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/crypto/ed25519"

	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/p2p/autopeering"
	"github.com/gohornet/hornet/pkg/shutdown"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusDisabled,
		Pluggable: node.Pluggable{
			Name:      "Autopeering",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   nil,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *node.Plugin
	deps   dependencies

	local *Local

	onDiscoveryPeerDiscovered  *events.Closure
	onDiscoveryPeerDeleted     *events.Closure
	onSelectionSaltUpdated     *events.Closure
	onSelectionOutgoingPeering *events.Closure
	onSelectionIncomingPeering *events.Closure
	onSelectionDropped         *events.Closure
	onPeerDisconnected         *events.Closure
	onAutopeerBecameKnown      *events.Closure
)

type dependencies struct {
	dig.In
	NodeConfig      *configuration.Configuration `name:"nodeConfig"`
	Manager         *p2p.Manager                 `optional:"true"`
	NodePrivateKey  crypto.PrivKey               `name:"nodePrivateKey"`
	P2PDatabasePath string                       `name:"p2pDatabasePath"`
}

func configure() {
	selection.SetParameters(selection.Parameters{
		InboundNeighborSize:  deps.NodeConfig.Int(CfgNetAutopeeringInboundPeers),
		OutboundNeighborSize: deps.NodeConfig.Int(CfgNetAutopeeringOutboundPeers),
		SaltLifetime:         deps.NodeConfig.Duration(CfgNetAutopeeringSaltLifetime),
	})

	if err := autopeering.RegisterAutopeeringProtocolInMultiAddresses(); err != nil {
		Plugin.Panicf("unable to register autopeering protocol for multi addresses: %s", err)
	}

	rawPrvKey, err := deps.NodePrivateKey.Raw()
	if err != nil {
		Plugin.Panicf("unable to obtain raw private key: %s", err)
	}

	local = newLocal(rawPrvKey[:ed25519.SeedSize], deps.P2PDatabasePath)
	configureAutopeering(local)
	configureEvents()
}

func run() {
	if err := Plugin.Node.Daemon().BackgroundWorker(Plugin.Name, func(shutdownSignal <-chan struct{}) {
		attachEvents()
		start(local, shutdownSignal)
		detachEvents()
	}, shutdown.PriorityAutopeering); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}
}

// gets the peering service key from the config.
func p2pServiceKey() service.Key {
	return service.Key(deps.NodeConfig.String(protocfg.CfgProtocolNetworkIDName))
}

func configureEvents() {

	onDiscoveryPeerDiscovered = events.NewClosure(func(ev *discover.DiscoveredEvent) {
		if peerID := autopeering.ConvertHivePubKeyToPeerIDOrLog(ev.Peer.PublicKey(), Plugin.LogWarnf); peerID != nil {
			Plugin.LogInfof("discovered: %s / %s", ev.Peer.Address(), *peerID)
		}
	})

	onDiscoveryPeerDeleted = events.NewClosure(func(ev *discover.DeletedEvent) {
		if peerID := autopeering.ConvertHivePubKeyToPeerIDOrLog(ev.Peer.PublicKey(), Plugin.LogWarnf); peerID != nil {
			Plugin.LogInfof("removed offline: %s / %s", ev.Peer.Address(), *peerID)
		}
	})

	onPeerDisconnected = events.NewClosure(func(peerOptErr *p2p.PeerOptError) {
		if peerOptErr.Peer.Relation != p2p.PeerRelationAutopeered {
			return
		}

		if id := autopeering.ConvertPeerIDToHiveIdentityOrLog(peerOptErr.Peer, Plugin.LogWarnf); id != nil {
			Plugin.LogInfof("removing: %s", peerOptErr.Peer.ID)
			selectionProtocol.RemoveNeighbor(id.ID())
		}
	})

	onAutopeerBecameKnown = events.NewClosure(func(p *p2p.Peer, oldRel p2p.PeerRelation) {
		if oldRel != p2p.PeerRelationAutopeered {
			return
		}
		if id := autopeering.ConvertPeerIDToHiveIdentityOrLog(p, Plugin.LogWarnf); id != nil {
			Plugin.LogInfof("removing %s from autopeering selection protocol", p.ID)
			selectionProtocol.RemoveNeighbor(id.ID())
		}
	})

	onSelectionSaltUpdated = events.NewClosure(func(ev *selection.SaltUpdatedEvent) {
		Plugin.LogInfof("salt updated; expires=%s", ev.Public.GetExpiration().Format(time.RFC822))
	})

	onSelectionOutgoingPeering = events.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return
		}
		Plugin.LogInfof("[outgoing peering] adding autopeering peer %s", ev.Peer.ID())

		addrInfo, err := autopeering.HivePeerToAddrInfo(ev.Peer, p2pServiceKey())
		if err != nil {
			Plugin.LogWarnf("unable to convert outgoing selection autopeering peer to addr info: %s", err)
			return
		}

		handleSelection(ev, addrInfo, func() {
			Plugin.LogInfof("connecting to %s", addrInfo)
			if err := deps.Manager.ConnectPeer(addrInfo, p2p.PeerRelationAutopeered); err != nil {
				Plugin.LogWarnf("couldn't add autopeering peer %s", err)
			}
		})
	})

	onSelectionIncomingPeering = events.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return
		}
		Plugin.LogInfof("[incoming peering] whitelisting %s", ev.Peer.ID())

		addrInfo, err := autopeering.HivePeerToAddrInfo(ev.Peer, p2pServiceKey())
		if err != nil {
			Plugin.LogWarnf("unable to convert incoming selection autopeering peer to addr info: %s", err)
			return
		}

		handleSelection(ev, addrInfo, func() {
			// TODO: maybe do whitelisting instead?
			//Plugin.LogInfof("connecting to %s", addrInfo)
			//if err := deps.Manager.ConnectPeer(addrInfo, p2p.PeerRelationAutopeered); err != nil {
			//	Plugin.LogWarnf("couldn't add autopeering peer %s", err)
			//}
		})
	})

	onSelectionDropped = events.NewClosure(func(ev *selection.DroppedEvent) {
		peerID := autopeering.ConvertHivePubKeyToPeerIDOrLog(ev.Peer.PublicKey(), Plugin.LogWarnf)
		if peerID == nil {
			return
		}

		Plugin.LogInfof("[dropped event] disconnecting %s", peerID)
		var peerRelation p2p.PeerRelation
		deps.Manager.Call(*peerID, func(p *p2p.Peer) {
			peerRelation = p.Relation
		})

		if len(peerRelation) == 0 {
			Plugin.LogWarnf("didn't find autopeered peer %s for disconnecting", peerID)
			return
		}

		if peerRelation != p2p.PeerRelationAutopeered {
			Plugin.LogWarnf("won't disconnect %s as it its relation is not 'discovered' but '%s'", peerID, peerRelation)
			return
		}

		if err := deps.Manager.DisconnectPeer(*peerID, errors.New("removed via autopeering selection")); err != nil {
			Plugin.LogWarnf("couldn't disconnect selection dropped autopeer: %s", err)
		}
	})
}

// handles a peer gotten from the autopeering selection according to its existing relation.
// if the peer is not yet part of the peering manager, the given noRelationFunc is called.
func handleSelection(ev *selection.PeeringEvent, addrInfo *libp2p.AddrInfo, noRelationFunc func()) {
	// extract peer relation
	var peerRelation p2p.PeerRelation
	deps.Manager.Call(addrInfo.ID, func(p *p2p.Peer) {
		peerRelation = p.Relation
	})

	switch peerRelation {
	case p2p.PeerRelationKnown:
		clearFromAutopeeringSelector(ev)

	case p2p.PeerRelationUnknown:
		updatePeerRelationToDiscovered(addrInfo)

	case p2p.PeerRelationAutopeered:
		handleAlreadyAutopeered(addrInfo)

	default:
		noRelationFunc()
	}
}

// logs a warning about a from the selector seen peer which is already autopeered.
func handleAlreadyAutopeered(addrInfo *libp2p.AddrInfo) {
	Plugin.LogWarnf("peer is already autopeered %s", addrInfo.ID)
}

// updates the given peers relation to discovered.
func updatePeerRelationToDiscovered(addrInfo *libp2p.AddrInfo) {
	if err := deps.Manager.ConnectPeer(addrInfo, p2p.PeerRelationAutopeered); err != nil {
		Plugin.LogWarnf("couldn't update unknown peer to 'discovered' %s", err)
	}
}

// clears an already statically peered from the autopeering selector.
func clearFromAutopeeringSelector(ev *selection.PeeringEvent) {
	Plugin.LogInfof("peer is statically peered already %s, removing from autopeering selection protocol", ev.Peer.ID())
	selectionProtocol.RemoveNeighbor(ev.Peer.ID())
}

func attachEvents() {
	discoveryProtocol.Events().PeerDiscovered.Attach(onDiscoveryPeerDiscovered)
	discoveryProtocol.Events().PeerDeleted.Attach(onDiscoveryPeerDeleted)

	if deps.Manager == nil {
		return
	}

	// notify the selection when a connection is closed or failed.
	deps.Manager.Events.Disconnected.Attach(onPeerDisconnected)
	deps.Manager.Events.RelationUpdated.Attach(onAutopeerBecameKnown)
	selectionProtocol.Events().SaltUpdated.Attach(onSelectionSaltUpdated)
	selectionProtocol.Events().OutgoingPeering.Attach(onSelectionOutgoingPeering)
	selectionProtocol.Events().IncomingPeering.Attach(onSelectionIncomingPeering)
	selectionProtocol.Events().Dropped.Attach(onSelectionDropped)
}

func detachEvents() {
	discoveryProtocol.Events().PeerDiscovered.Detach(onDiscoveryPeerDiscovered)
	discoveryProtocol.Events().PeerDeleted.Detach(onDiscoveryPeerDeleted)

	if deps.Manager == nil {
		return
	}

	deps.Manager.Events.Disconnected.Detach(onPeerDisconnected)
	deps.Manager.Events.RelationUpdated.Detach(onAutopeerBecameKnown)
	selectionProtocol.Events().SaltUpdated.Detach(onSelectionSaltUpdated)
	selectionProtocol.Events().OutgoingPeering.Detach(onSelectionOutgoingPeering)
	selectionProtocol.Events().IncomingPeering.Detach(onSelectionIncomingPeering)
	selectionProtocol.Events().Dropped.Detach(onSelectionDropped)
}
