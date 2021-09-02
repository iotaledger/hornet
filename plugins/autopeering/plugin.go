package autopeering

import (
	"strings"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	libp2p "github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/crypto/ed25519"

	databaseCore "github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/core/gossip"
	"github.com/gohornet/hornet/core/pow"
	"github.com/gohornet/hornet/core/snapshot"
	"github.com/gohornet/hornet/core/tangle"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/p2p/autopeering"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/coordinator"
	"github.com/gohornet/hornet/plugins/dashboard"
	"github.com/gohornet/hornet/plugins/debug"
	"github.com/gohornet/hornet/plugins/faucet"
	"github.com/gohornet/hornet/plugins/migrator"
	"github.com/gohornet/hornet/plugins/mqtt"
	"github.com/gohornet/hornet/plugins/prometheus"
	"github.com/gohornet/hornet/plugins/receipt"
	restapiv1 "github.com/gohornet/hornet/plugins/restapi/v1"
	"github.com/gohornet/hornet/plugins/spammer"
	"github.com/gohornet/hornet/plugins/urts"
	"github.com/gohornet/hornet/plugins/warpsync"

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
			Name:       "Autopeering",
			DepsFunc:   func(cDeps dependencies) { deps = cDeps },
			Params:     params,
			PreProvide: preProvide,
			Configure:  configure,
			Run:        run,
		},
	}
}

var (
	Plugin *node.Plugin
	deps   dependencies

	localPeerContainer *autopeering.LocalPeerContainer

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
	NodeConfig                *configuration.Configuration `name:"nodeConfig"`
	NodePrivateKey            crypto.PrivKey               `name:"nodePrivateKey"`
	P2PDatabasePath           string                       `name:"p2pDatabasePath"`
	P2PBindMultiAddresses     []string                     `name:"p2pBindMultiAddresses"`
	DatabaseEngine            database.Engine              `name:"databaseEngine"`
	NetworkIDName             string                       `name:"networkIdName"`
	AutopeeringRunAsEntryNode bool                         `name:"autopeeringRunAsEntryNode"`
	Manager                   *p2p.Manager                 `optional:"true"`
}

func preProvide(c *dig.Container, configs map[string]*configuration.Configuration, initConfig *node.InitConfig) {

	pluginEnabled := true

	containsPlugin := func(pluginsList []string, pluginIdentifier string) bool {
		contains := false
		for _, plugin := range pluginsList {
			if strings.ToLower(plugin) == pluginIdentifier {
				contains = true
				break
			}
		}
		return contains
	}

	if disabled := containsPlugin(initConfig.DisabledPlugins, Plugin.Identifier()); disabled {
		// Autopeering is disabled
		pluginEnabled = false
	}

	if enabled := containsPlugin(initConfig.EnabledPlugins, Plugin.Identifier()); !enabled {
		// Autopeering was not enabled
		pluginEnabled = false
	}

	runAsEntryNode := pluginEnabled && configs["nodeConfig"].Bool(CfgNetAutopeeringRunAsEntryNode)
	if runAsEntryNode {
		// the following pluggables stay enabled
		// - profile
		// - protocfg
		// - gracefulshutdown
		// - p2p
		// - profiling
		// - versioncheck
		// - autopeering

		// disable the other plugins if the node runs as an entry node for autopeering
		initConfig.ForceDisablePluggable(databaseCore.CorePlugin.Identifier())
		initConfig.ForceDisablePluggable(pow.CorePlugin.Identifier())
		initConfig.ForceDisablePluggable(gossip.CorePlugin.Identifier())
		initConfig.ForceDisablePluggable(tangle.CorePlugin.Identifier())
		initConfig.ForceDisablePluggable(snapshot.CorePlugin.Identifier())
		initConfig.ForceDisablePluggable(restapiv1.Plugin.Identifier())
		initConfig.ForceDisablePluggable(warpsync.Plugin.Identifier())
		initConfig.ForceDisablePluggable(urts.Plugin.Identifier())
		initConfig.ForceDisablePluggable(dashboard.Plugin.Identifier())
		initConfig.ForceDisablePluggable(spammer.Plugin.Identifier())
		initConfig.ForceDisablePluggable(mqtt.Plugin.Identifier())
		initConfig.ForceDisablePluggable(coordinator.Plugin.Identifier())
		initConfig.ForceDisablePluggable(migrator.Plugin.Identifier())
		initConfig.ForceDisablePluggable(receipt.Plugin.Identifier())
		initConfig.ForceDisablePluggable(prometheus.Plugin.Identifier())
		initConfig.ForceDisablePluggable(debug.Plugin.Identifier())
		initConfig.ForceDisablePluggable(faucet.Plugin.Identifier())
	}

	// the parameter has to be provided in the preProvide stage.
	// this is a special case, since it only should be true if the plugin is enabled
	type cfgResult struct {
		dig.Out
		AutopeeringRunAsEntryNode bool `name:"autopeeringRunAsEntryNode"`
	}

	if err := c.Provide(func() cfgResult {
		return cfgResult{
			AutopeeringRunAsEntryNode: runAsEntryNode,
		}
	}); err != nil {
		Plugin.Panic(err)
	}
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

	localPeerContainer, err = autopeering.NewLocalPeerContainer(p2pServiceKey(),
		rawPrvKey[:ed25519.SeedSize],
		deps.P2PDatabasePath,
		deps.DatabaseEngine,
		deps.P2PBindMultiAddresses,
		deps.NodeConfig.String(CfgNetAutopeeringBindAddr),
		deps.AutopeeringRunAsEntryNode)
	if err != nil {
		Plugin.Panicf("unable to initialize local peer container: %s", err)
	}

	Plugin.LogInfof("initialized local autopeering: %s@%s", localPeerContainer.Local().PublicKey(), localPeerContainer.Local().Address())

	if deps.AutopeeringRunAsEntryNode {
		entryNodeMultiAddress, err := autopeering.GetEntryNodeMultiAddress(localPeerContainer.Local())
		if err != nil {
			Plugin.Panicf("unable to parse entry node multiaddress: %s", err)
		}

		Plugin.LogInfof("\n\nentry node multiaddress: %s\n", entryNodeMultiAddress.String())
	}

	configureAutopeering(localPeerContainer)
	configureEvents()
}

func run() {
	if err := Plugin.Node.Daemon().BackgroundWorker(Plugin.Name, func(shutdownSignal <-chan struct{}) {
		attachEvents()
		start(localPeerContainer, shutdownSignal)
		detachEvents()
	}, shutdown.PriorityAutopeering); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}
}

// gets the peering service key from the config.
func p2pServiceKey() service.Key {
	return service.Key(deps.NetworkIDName)
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
			Plugin.LogWarnf("won't disconnect %s as it its relation is not '%s' but '%s'", peerID, p2p.PeerRelationAutopeered, peerRelation)
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
