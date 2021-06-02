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
	p2ppkg "github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/p2p/autopeering"
	"github.com/gohornet/hornet/pkg/shutdown"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.Enabled,
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

	deps dependencies
)

var (
	log   *logger.Logger
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
	NodeConfig     *configuration.Configuration `name:"nodeConfig"`
	Manager        *p2ppkg.Manager              `optional:"true"`
	NodePrivateKey crypto.PrivKey
}

func configure() {
	selection.SetParameters(selection.Parameters{
		InboundNeighborSize:  deps.NodeConfig.Int(CfgNetAutopeeringInboundPeers),
		OutboundNeighborSize: deps.NodeConfig.Int(CfgNetAutopeeringOutboundPeers),
		SaltLifetime:         deps.NodeConfig.Duration(CfgNetAutopeeringSaltLifetime),
	})
	log = logger.NewLogger(Plugin.Name)
	if err := autopeering.RegisterAutopeeringProtocolInMultiAddresses(); err != nil {
		log.Panicf("unable to register autopeering protocol for multi addresses: %s", err)
	}
	rawPrvKey, err := deps.NodePrivateKey.Raw()
	if err != nil {
		log.Panicf("unable to obtain raw private key: %s", err)
	}
	local = newLocal(rawPrvKey[:ed25519.SeedSize])
	configureAutopeering(local)
	configureEvents()
}

func run() {
	_ = Plugin.Node.Daemon().BackgroundWorker(Plugin.Name, func(shutdownSignal <-chan struct{}) {
		attachEvents()
		start(local, shutdownSignal)
		detachEvents()
	}, shutdown.PriorityAutopeering)
}

// gets the peering service key from the config.
func p2pServiceKey() service.Key {
	return service.Key(deps.NodeConfig.String(protocfg.CfgProtocolNetworkIDName))
}

func configureEvents() {

	onDiscoveryPeerDiscovered = events.NewClosure(func(ev *discover.DiscoveredEvent) {
		if peerID := autopeering.ConvertHivePubKeyToPeerIDOrLog(ev.Peer.PublicKey(), log.Warnf); peerID != nil {
			log.Infof("discovered: %s / %s", ev.Peer.Address(), *peerID)
		}
	})

	onDiscoveryPeerDeleted = events.NewClosure(func(ev *discover.DeletedEvent) {
		if peerID := autopeering.ConvertHivePubKeyToPeerIDOrLog(ev.Peer.PublicKey(), log.Warnf); peerID != nil {
			log.Infof("removed offline: %s / %s", ev.Peer.Address(), *peerID)
		}
	})

	onPeerDisconnected = events.NewClosure(func(peerOptErr *p2p.PeerOptError) {
		if peerOptErr.Peer.Relation != p2p.PeerRelationAutopeered {
			return
		}

		if id := autopeering.ConvertPeerIDToHiveIdentityOrLog(peerOptErr.Peer, log.Warnf); id != nil {
			log.Infof("removing: %s", peerOptErr.Peer.ID)
			selectionProtocol.RemoveNeighbor(id.ID())
		}
	})

	onAutopeerBecameKnown = events.NewClosure(func(p *p2p.Peer, oldRel p2p.PeerRelation) {
		if oldRel != p2p.PeerRelationAutopeered {
			return
		}
		if id := autopeering.ConvertPeerIDToHiveIdentityOrLog(p, log.Warnf); id != nil {
			log.Infof("removing %s from autopeering selection protocol", p.ID)
			selectionProtocol.RemoveNeighbor(id.ID())
		}
	})

	onSelectionSaltUpdated = events.NewClosure(func(ev *selection.SaltUpdatedEvent) {
		log.Infof("salt updated; expires=%s", ev.Public.GetExpiration().Format(time.RFC822))
	})

	onSelectionOutgoingPeering = events.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return
		}
		log.Infof("[outgoing peering] adding autopeering peer %s", ev.Peer.ID())

		addrInfo, err := autopeering.HivePeerToAddrInfo(ev.Peer, p2pServiceKey())
		if err != nil {
			log.Warnf("unable to convert outgoing selection autopeering peer to addr info: %s", err)
			return
		}

		handleSelection(ev, addrInfo, func() {
			log.Infof("connecting to %s", addrInfo)
			if err := deps.Manager.ConnectPeer(addrInfo, p2ppkg.PeerRelationAutopeered); err != nil {
				log.Warnf("couldn't add autopeering peer %s", err)
			}
		})
	})

	onSelectionIncomingPeering = events.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return
		}
		log.Infof("[incoming peering] whitelisting %s", ev.Peer.ID())

		addrInfo, err := autopeering.HivePeerToAddrInfo(ev.Peer, p2pServiceKey())
		if err != nil {
			log.Warnf("unable to convert incoming selection autopeering peer to addr info: %s", err)
			return
		}

		handleSelection(ev, addrInfo, func() {
			// TODO: maybe do whitelisting instead?
			//log.Infof("connecting to %s", addrInfo)
			//if err := deps.Manager.ConnectPeer(addrInfo, p2ppkg.PeerRelationAutopeered); err != nil {
			//	log.Warnf("couldn't add autopeering peer %s", err)
			//}
		})
	})

	onSelectionDropped = events.NewClosure(func(ev *selection.DroppedEvent) {
		peerID := autopeering.ConvertHivePubKeyToPeerIDOrLog(ev.Peer.PublicKey(), log.Warnf)
		if peerID == nil {
			return
		}

		log.Infof("[dropped event] disconnecting %s", peerID)
		var peerRelation p2ppkg.PeerRelation
		deps.Manager.Call(*peerID, func(p *p2ppkg.Peer) {
			peerRelation = p.Relation
		})

		if len(peerRelation) == 0 {
			log.Warnf("didn't find autopeered peer %s for disconnecting", peerID)
			return
		}

		if peerRelation != p2ppkg.PeerRelationAutopeered {
			log.Warnf("won't disconnect %s as it its relation is not 'discovered' but '%s'", peerID, peerRelation)
			return
		}

		if err := deps.Manager.DisconnectPeer(*peerID, errors.New("removed via autopeering selection")); err != nil {
			log.Warnf("couldn't disconnect selection dropped autopeer: %s", err)
		}
	})
}

// handles a peer gotten from the autopeering selection according to its existing relation.
// if the peer is not yet part of the peering manager, the given noRelationFunc is called.
func handleSelection(ev *selection.PeeringEvent, addrInfo *libp2p.AddrInfo, noRelationFunc func()) {
	// extract peer relation
	var peerRelation p2ppkg.PeerRelation
	deps.Manager.Call(addrInfo.ID, func(p *p2ppkg.Peer) {
		peerRelation = p.Relation
	})

	switch peerRelation {
	case p2ppkg.PeerRelationKnown:
		clearFromAutopeeringSelector(ev)

	case p2ppkg.PeerRelationUnknown:
		updatePeerRelationToDiscovered(addrInfo)

	case p2ppkg.PeerRelationAutopeered:
		handleAlreadyAutopeered(addrInfo)

	default:
		noRelationFunc()
	}
}

// logs a warning about a from the selector seen peer which is already autopeered.
func handleAlreadyAutopeered(addrInfo *libp2p.AddrInfo) {
	log.Warnf("peer is already autopeered %s", addrInfo.ID)
}

// updates the given peers relation to discovered.
func updatePeerRelationToDiscovered(addrInfo *libp2p.AddrInfo) {
	if err := deps.Manager.ConnectPeer(addrInfo, p2ppkg.PeerRelationAutopeered); err != nil {
		log.Warnf("couldn't update unknown peer to 'discovered' %s", err)
	}
}

// clears an already statically peered from the autopeering selector.
func clearFromAutopeeringSelector(ev *selection.PeeringEvent) {
	log.Infof("peer is statically peered already %s, removing from autopeering selection protocol", ev.Peer.ID())
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
