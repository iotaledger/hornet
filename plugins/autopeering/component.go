package autopeering

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	libp2p "github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/hive.go/core/autopeering/discover"
	"github.com/iotaledger/hive.go/core/autopeering/peer/service"
	"github.com/iotaledger/hive.go/core/autopeering/selection"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	hivedb "github.com/iotaledger/hive.go/core/database"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/generics/event"
	databaseCore "github.com/iotaledger/hornet/v2/core/database"
	"github.com/iotaledger/hornet/v2/core/gossip"
	"github.com/iotaledger/hornet/v2/core/pow"
	"github.com/iotaledger/hornet/v2/core/protocfg"
	"github.com/iotaledger/hornet/v2/core/pruning"
	"github.com/iotaledger/hornet/v2/core/snapshot"
	"github.com/iotaledger/hornet/v2/core/tangle"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/p2p"
	"github.com/iotaledger/hornet/v2/pkg/p2p/autopeering"
	"github.com/iotaledger/hornet/v2/plugins/coreapi"
	dashboard_metrics "github.com/iotaledger/hornet/v2/plugins/dashboard-metrics"
	"github.com/iotaledger/hornet/v2/plugins/debug"
	"github.com/iotaledger/hornet/v2/plugins/inx"
	"github.com/iotaledger/hornet/v2/plugins/prometheus"
	"github.com/iotaledger/hornet/v2/plugins/receipt"
	"github.com/iotaledger/hornet/v2/plugins/urts"
	"github.com/iotaledger/hornet/v2/plugins/warpsync"
)

func init() {
	Plugin = &app.Plugin{
		Component: &app.Component{
			Name:       "Autopeering",
			DepsFunc:   func(cDeps dependencies) { deps = cDeps },
			Params:     params,
			PreProvide: preProvide,
			Provide:    provide,
			Configure:  configure,
			Run:        run,
		},
		IsEnabled: func() bool {
			return ParamsAutopeering.Enabled
		},
	}
}

var (
	Plugin *app.Plugin
	deps   dependencies

	localPeerContainer *autopeering.LocalPeerContainer

	onDiscoveryPeerDiscovered  *event.Closure[*discover.PeerDiscoveredEvent]
	onDiscoveryPeerDeleted     *event.Closure[*discover.PeerDeletedEvent]
	onSelectionSaltUpdated     *event.Closure[*selection.SaltUpdatedEvent]
	onSelectionOutgoingPeering *event.Closure[*selection.PeeringEvent]
	onSelectionIncomingPeering *event.Closure[*selection.PeeringEvent]
	onSelectionDropped         *event.Closure[*selection.DroppedEvent]
	onPeerConnected            *events.Closure
	onPeerDisconnected         *events.Closure
	onPeeringRelationUpdated   *events.Closure
)

type dependencies struct {
	dig.In
	NodePrivateKey            crypto.PrivKey `name:"nodePrivateKey"`
	P2PDatabasePath           string         `name:"p2pDatabasePath"`
	P2PBindMultiAddresses     []string       `name:"p2pBindMultiAddresses"`
	DatabaseEngine            hivedb.Engine  `name:"databaseEngine"`
	AutopeeringRunAsEntryNode bool           `name:"autopeeringRunAsEntryNode"`
	PeeringManager            *p2p.Manager   `optional:"true"`
	AutopeeringManager        *autopeering.Manager
}

func preProvide(c *dig.Container, _ *app.App, initConfig *app.InitConfig) error {

	runAsEntryNode := Plugin.IsEnabled() && ParamsAutopeering.RunAsEntryNode
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
		initConfig.ForceDisableComponent(databaseCore.CoreComponent.Identifier())
		initConfig.ForceDisableComponent(pow.CoreComponent.Identifier())
		initConfig.ForceDisableComponent(gossip.CoreComponent.Identifier())
		initConfig.ForceDisableComponent(tangle.CoreComponent.Identifier())
		initConfig.ForceDisableComponent(protocfg.CoreComponent.Identifier())
		initConfig.ForceDisableComponent(snapshot.CoreComponent.Identifier())
		initConfig.ForceDisableComponent(pruning.CoreComponent.Identifier())
		initConfig.ForceDisableComponent(coreapi.Plugin.Identifier())
		initConfig.ForceDisableComponent(warpsync.Plugin.Identifier())
		initConfig.ForceDisableComponent(urts.Plugin.Identifier())
		initConfig.ForceDisableComponent(receipt.Plugin.Identifier())
		initConfig.ForceDisableComponent(prometheus.Plugin.Identifier())
		initConfig.ForceDisableComponent(inx.Plugin.Identifier())
		initConfig.ForceDisableComponent(dashboard_metrics.Plugin.Identifier())
		initConfig.ForceDisableComponent(debug.Plugin.Identifier())
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
		Plugin.LogPanic(err)
	}

	return nil
}

func provide(c *dig.Container) error {

	type autopeeringDeps struct {
		dig.In
		TargetNetworkName string `name:"targetNetworkName"`
	}

	if err := c.Provide(func(deps autopeeringDeps) *autopeering.Manager {
		return autopeering.NewManager(
			Plugin.Logger(),
			ParamsAutopeering.BindAddress,
			ParamsAutopeering.EntryNodes,
			ParamsAutopeering.EntryNodesPreferIPv6,
			service.Key(deps.TargetNetworkName),
		)
	}); err != nil {
		Plugin.LogPanic(err)
	}

	return nil
}

func configure() error {
	selection.SetParameters(selection.Parameters{
		InboundNeighborSize:  ParamsAutopeering.InboundPeers,
		OutboundNeighborSize: ParamsAutopeering.OutboundPeers,
		SaltLifetime:         ParamsAutopeering.SaltLifetime,
	})

	if err := autopeering.RegisterAutopeeringProtocolInMultiAddresses(); err != nil {
		Plugin.LogPanicf("unable to register autopeering protocol for multi addresses: %s", err)
	}

	rawPrvKey, err := deps.NodePrivateKey.Raw()
	if err != nil {
		Plugin.LogPanicf("unable to obtain raw private key: %s", err)
	}

	localPeerContainer, err = autopeering.NewLocalPeerContainer(
		deps.AutopeeringManager.P2PServiceKey(),
		rawPrvKey[:ed25519.SeedSize],
		deps.P2PDatabasePath,
		deps.DatabaseEngine,
		deps.P2PBindMultiAddresses,
		ParamsAutopeering.BindAddress,
		deps.AutopeeringRunAsEntryNode,
	)
	if err != nil {
		Plugin.LogPanicf("unable to initialize local peer container: %s", err)
	}

	Plugin.LogInfof("initialized local autopeering: %s@%s", localPeerContainer.Local().PublicKey(), localPeerContainer.Local().Address())

	if deps.AutopeeringRunAsEntryNode {
		entryNodeMultiAddress, err := autopeering.GetEntryNodeMultiAddress(localPeerContainer.Local())
		if err != nil {
			Plugin.LogPanicf("unable to parse entry node multiaddress: %s", err)
		}

		Plugin.LogInfof("\n\nentry node multiaddress: %s\n", entryNodeMultiAddress.String())
	}

	// only enable peer selection when the peering plugin is enabled
	initSelection := deps.PeeringManager != nil

	deps.AutopeeringManager.Init(localPeerContainer, initSelection)
	configureEvents()

	return nil
}

func run() error {
	if err := Plugin.App().Daemon().BackgroundWorker(Plugin.Name, func(ctx context.Context) {
		attachEvents()
		deps.AutopeeringManager.Run(ctx)
		detachEvents()
	}, daemon.PriorityAutopeering); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}

func configureEvents() {

	onDiscoveryPeerDiscovered = event.NewClosure(func(ev *discover.PeerDiscoveredEvent) {
		peerID, err := autopeering.HivePeerToPeerID(ev.Peer)
		if err != nil {
			Plugin.LogWarnf("unable to convert discovered autopeering peer to peerID: %s", err)

			return
		}

		Plugin.LogInfof("discovered: %s / %s", ev.Peer.Address(), peerID.ShortString())
	})

	onDiscoveryPeerDeleted = event.NewClosure(func(ev *discover.PeerDeletedEvent) {
		peerID, err := autopeering.HivePeerToPeerID(ev.Peer)
		if err != nil {
			Plugin.LogWarnf("unable to convert deleted autopeering peer to peerID: %s", err)

			return
		}

		Plugin.LogInfof("removed offline: %s / %s", ev.Peer.Address(), peerID.ShortString())
	})

	onPeerConnected = events.NewClosure(func(p *p2p.Peer, conn network.Conn) {

		if deps.AutopeeringManager.Selection() == nil {
			return
		}

		id := autopeering.ConvertPeerIDToHiveIdentityOrLog(p, Plugin.LogWarnf)
		if id == nil {
			return
		}

		// we block peers that are connected via manual peering in the autopeering module.
		// this ensures that no additional connections are established via autopeering.
		switch p.Relation {
		case p2p.PeerRelationKnown, p2p.PeerRelationUnknown:
			deps.AutopeeringManager.Selection().BlockNeighbor(id.ID())
		}
	})

	onPeerDisconnected = events.NewClosure(func(peerOptErr *p2p.PeerOptError) {

		if deps.AutopeeringManager.Selection() == nil {
			return
		}

		id := autopeering.ConvertPeerIDToHiveIdentityOrLog(peerOptErr.Peer, Plugin.LogWarnf)
		if id == nil {
			return
		}

		// if a peer is disconnected, we need to remove it from the autopeering blacklist
		// so that former known and unknown peers can be autopeered.
		deps.AutopeeringManager.Selection().UnblockNeighbor(id.ID())

		if peerOptErr.Peer.Relation != p2p.PeerRelationAutopeered {
			return
		}

		Plugin.LogDebugf("removing: %s", peerOptErr.Peer.ID.ShortString())
		deps.AutopeeringManager.Selection().RemoveNeighbor(id.ID())
	})

	onPeeringRelationUpdated = events.NewClosure(func(p *p2p.Peer, oldRel p2p.PeerRelation) {

		if deps.AutopeeringManager.Selection() == nil {
			return
		}

		id := autopeering.ConvertPeerIDToHiveIdentityOrLog(p, Plugin.LogWarnf)
		if id == nil {
			return
		}

		// we block peers that are connected via manual peering in the autopeering module.
		// this ensures that no additional connections are established via autopeering.
		// if a peer gets updated to autopeered, we need to unblock it.
		switch p.Relation {
		case p2p.PeerRelationKnown, p2p.PeerRelationUnknown:
			deps.AutopeeringManager.Selection().BlockNeighbor(id.ID())
		case p2p.PeerRelationAutopeered:
			deps.AutopeeringManager.Selection().UnblockNeighbor(id.ID())
		}

		if oldRel != p2p.PeerRelationAutopeered {
			return
		}

		Plugin.LogInfof("removing %s from autopeering selection protocol", p.ID.ShortString())
		deps.AutopeeringManager.Selection().RemoveNeighbor(id.ID())
	})

	onSelectionSaltUpdated = event.NewClosure(func(ev *selection.SaltUpdatedEvent) {
		Plugin.LogInfof("salt updated; expires=%s", ev.Public.GetExpiration().Format(time.RFC822))
	})

	onSelectionOutgoingPeering = event.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return
		}

		addrInfo, err := autopeering.HivePeerToAddrInfo(ev.Peer, deps.AutopeeringManager.P2PServiceKey())
		if err != nil {
			Plugin.LogWarnf("unable to convert outgoing selection autopeering peer to addr info: %s", err)

			return
		}

		Plugin.LogInfof("[outgoing peering] adding autopeering peer %s", addrInfo.ID.ShortString())

		handleSelection(ev, addrInfo, func() {
			Plugin.LogInfof("connecting to %s", addrInfo.ID.ShortString())
			if err := deps.PeeringManager.ConnectPeer(addrInfo, p2p.PeerRelationAutopeered); err != nil {
				Plugin.LogWarnf("couldn't add autopeering peer %s: %s", addrInfo.ID.ShortString(), err)
			}
		})
	})

	onSelectionIncomingPeering = event.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return
		}

		addrInfo, err := autopeering.HivePeerToAddrInfo(ev.Peer, deps.AutopeeringManager.P2PServiceKey())
		if err != nil {
			Plugin.LogWarnf("unable to convert incoming selection autopeering peer to addr info: %s", err)

			return
		}

		Plugin.LogInfof("[incoming peering] allow autopeering peer %s", addrInfo.ID.ShortString())

		handleSelection(ev, addrInfo, func() {
			if err := deps.PeeringManager.AllowPeer(addrInfo.ID); err != nil {
				Plugin.LogWarnf("couldn't allow autopeering peer %s: %s", addrInfo.ID.ShortString(), err)
			}
		})
	})

	onSelectionDropped = event.NewClosure(func(ev *selection.DroppedEvent) {
		peerID, err := autopeering.HivePeerToPeerID(ev.Peer)
		if err != nil {
			Plugin.LogWarnf("unable to convert dropped autopeering peer to peerID: %s", err)

			return
		}

		Plugin.LogInfof("[dropped event] disconnecting %s / %s", ev.Peer.Address(), peerID.ShortString())

		if err := deps.PeeringManager.DisallowPeer(peerID); err != nil {
			Plugin.LogWarnf("couldn't disallow autopeering peer %s: %s", peerID.ShortString(), err)
		}

		var peerRelation p2p.PeerRelation
		deps.PeeringManager.Call(peerID, func(p *p2p.Peer) {
			peerRelation = p.Relation
		})

		if len(peerRelation) == 0 {
			Plugin.LogWarnf("didn't find autopeered peer %s for disconnecting", peerID.ShortString())

			return
		}

		if peerRelation != p2p.PeerRelationAutopeered {
			Plugin.LogWarnf("won't disconnect %s as its relation is not '%s' but '%s'", peerID.ShortString(), p2p.PeerRelationAutopeered, peerRelation)

			return
		}

		if err := deps.PeeringManager.DisconnectPeer(peerID, errors.New("removed via autopeering selection")); err != nil {
			Plugin.LogWarnf("couldn't disconnect selection dropped autopeer %s: %s", peerID.ShortString(), err)
		}
	})
}

// handles a peer gotten from the autopeering selection according to its existing relation.
// if the peer is not yet part of the peering manager, the given noRelationFunc is called.
func handleSelection(ev *selection.PeeringEvent, addrInfo *libp2p.AddrInfo, noRelationFunc func()) {
	// extract peer relation
	var peerRelation p2p.PeerRelation
	deps.PeeringManager.Call(addrInfo.ID, func(p *p2p.Peer) {
		peerRelation = p.Relation
	})

	switch peerRelation {
	case p2p.PeerRelationKnown:
		clearFromAutopeeringSelector(ev)

	case p2p.PeerRelationUnknown:
		clearFromAutopeeringSelector(ev)

	case p2p.PeerRelationAutopeered:
		handleAlreadyAutopeered(addrInfo)

	default:
		noRelationFunc()
	}
}

// logs a warning about a from the selector seen peer which is already autopeered.
func handleAlreadyAutopeered(addrInfo *libp2p.AddrInfo) {
	Plugin.LogWarnf("peer is already autopeered %s", addrInfo.ID.ShortString())
}

// clears an already statically peered from the autopeering selector.
func clearFromAutopeeringSelector(ev *selection.PeeringEvent) {
	peerID, err := autopeering.HivePeerToPeerID(ev.Peer)
	if err != nil {
		Plugin.LogWarnf("unable to convert selection autopeering peer to peerID: %s", err)

		return
	}

	if deps.AutopeeringManager.Selection() != nil {
		Plugin.LogInfof("peer is statically peered already %s, removing from autopeering selection protocol", peerID.ShortString())
		deps.AutopeeringManager.Selection().RemoveNeighbor(ev.Peer.ID())
	}
}

func attachEvents() {

	if deps.AutopeeringManager.Discovery() != nil {
		deps.AutopeeringManager.Discovery().Events().PeerDiscovered.Hook(onDiscoveryPeerDiscovered)
		deps.AutopeeringManager.Discovery().Events().PeerDeleted.Hook(onDiscoveryPeerDeleted)
	}

	if deps.AutopeeringManager.Selection() != nil {
		// notify the selection when a connection is closed or failed.
		deps.PeeringManager.Events.Connected.Hook(onPeerConnected)
		deps.PeeringManager.Events.Disconnected.Hook(onPeerDisconnected)
		deps.PeeringManager.Events.RelationUpdated.Hook(onPeeringRelationUpdated)
		deps.AutopeeringManager.Selection().Events().SaltUpdated.Hook(onSelectionSaltUpdated)
		deps.AutopeeringManager.Selection().Events().OutgoingPeering.Hook(onSelectionOutgoingPeering)
		deps.AutopeeringManager.Selection().Events().IncomingPeering.Hook(onSelectionIncomingPeering)
		deps.AutopeeringManager.Selection().Events().Dropped.Hook(onSelectionDropped)
	}
}

func detachEvents() {

	if deps.AutopeeringManager.Discovery() != nil {
		deps.AutopeeringManager.Discovery().Events().PeerDiscovered.Detach(onDiscoveryPeerDiscovered)
		deps.AutopeeringManager.Discovery().Events().PeerDeleted.Detach(onDiscoveryPeerDeleted)
	}

	if deps.AutopeeringManager.Selection() != nil {
		deps.PeeringManager.Events.Connected.Detach(onPeerConnected)
		deps.PeeringManager.Events.Disconnected.Detach(onPeerDisconnected)
		deps.PeeringManager.Events.RelationUpdated.Detach(onPeeringRelationUpdated)
		deps.AutopeeringManager.Selection().Events().SaltUpdated.Detach(onSelectionSaltUpdated)
		deps.AutopeeringManager.Selection().Events().OutgoingPeering.Detach(onSelectionOutgoingPeering)
		deps.AutopeeringManager.Selection().Events().IncomingPeering.Detach(onSelectionIncomingPeering)
		deps.AutopeeringManager.Selection().Events().Dropped.Detach(onSelectionDropped)
	}
}
