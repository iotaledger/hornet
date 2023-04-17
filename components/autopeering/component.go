package autopeering

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	libp2p "github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	hivedb "github.com/iotaledger/hive.go/kvstore/database"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/p2p"
	"github.com/iotaledger/hornet/v2/pkg/p2p/autopeering"
)

func init() {
	Component = &app.Component{
		Name:             "Autopeering",
		DepsFunc:         func(cDeps dependencies) { deps = cDeps },
		Params:           params,
		InitConfigParams: initConfigParams,
		IsEnabled:        func(_ *dig.Container) bool { return ParamsAutopeering.Enabled },
		Provide:          provide,
		Configure:        configure,
		Run:              run,
	}
}

var (
	Component *app.Component
	deps      dependencies

	localPeerContainer *autopeering.LocalPeerContainer
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

func initConfigParams(c *dig.Container) error {
	type cfgResult struct {
		dig.Out
		AutopeeringRunAsEntryNode bool `name:"autopeeringRunAsEntryNode"`
	}

	if err := c.Provide(func() cfgResult {
		return cfgResult{
			// it only should be true if the plugin is enabled
			AutopeeringRunAsEntryNode: Component.IsEnabled(c) && ParamsAutopeering.RunAsEntryNode,
		}
	}); err != nil {
		Component.LogPanic(err)
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
			Component.Logger(),
			ParamsAutopeering.BindAddress,
			ParamsAutopeering.EntryNodes,
			ParamsAutopeering.EntryNodesPreferIPv6,
			service.Key(deps.TargetNetworkName),
		)
	}); err != nil {
		Component.LogPanic(err)
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
		Component.LogPanicf("unable to register autopeering protocol for multi addresses: %s", err)
	}

	rawPrvKey, err := deps.NodePrivateKey.Raw()
	if err != nil {
		Component.LogPanicf("unable to obtain raw private key: %s", err)
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
		Component.LogPanicf("unable to initialize local peer container: %s", err)
	}

	Component.LogInfof("initialized local autopeering: %s@%s", localPeerContainer.Local().PublicKey(), localPeerContainer.Local().Address())

	if deps.AutopeeringRunAsEntryNode {
		entryNodeMultiAddress, err := autopeering.GetEntryNodeMultiAddress(localPeerContainer.Local())
		if err != nil {
			Component.LogPanicf("unable to parse entry node multiaddress: %s", err)
		}

		Component.LogInfof("\n\nentry node multiaddress: %s\n", entryNodeMultiAddress.String())
	}

	// only enable peer selection when the peering plugin is enabled
	initSelection := deps.PeeringManager != nil

	deps.AutopeeringManager.Init(localPeerContainer, initSelection)

	return nil
}

func run() error {
	if err := Component.App().Daemon().BackgroundWorker(Component.Name, func(ctx context.Context) {
		detach := hookEvents()
		defer detach()
		deps.AutopeeringManager.Run(ctx)
	}, daemon.PriorityAutopeering); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	return nil
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
	Component.LogWarnf("peer is already autopeered %s", addrInfo.ID.ShortString())
}

// clears an already statically peered from the autopeering selector.
func clearFromAutopeeringSelector(ev *selection.PeeringEvent) {
	peerID, err := autopeering.HivePeerToPeerID(ev.Peer)
	if err != nil {
		Component.LogWarnf("unable to convert selection autopeering peer to peerID: %s", err)

		return
	}

	if deps.AutopeeringManager.Selection() != nil {
		Component.LogInfof("peer is statically peered already %s, removing from autopeering selection protocol", peerID.ShortString())
		deps.AutopeeringManager.Selection().RemoveNeighbor(ev.Peer.ID())
	}
}

func hookEvents() (unhook func()) {
	var unhookCallbacks []func()
	if deps.AutopeeringManager.Discovery() != nil {
		unhookCallbacks = append(unhookCallbacks,
			deps.AutopeeringManager.Discovery().Events().PeerDiscovered.Hook(func(ev *discover.PeerDiscoveredEvent) {
				peerID, err := autopeering.HivePeerToPeerID(ev.Peer)
				if err != nil {
					Component.LogWarnf("unable to convert discovered autopeering peer to peerID: %s", err)

					return
				}

				Component.LogInfof("discovered: %s / %s", ev.Peer.Address(), peerID.ShortString())
			}).Unhook,
			deps.AutopeeringManager.Discovery().Events().PeerDeleted.Hook(func(ev *discover.PeerDeletedEvent) {
				peerID, err := autopeering.HivePeerToPeerID(ev.Peer)
				if err != nil {
					Component.LogWarnf("unable to convert deleted autopeering peer to peerID: %s", err)

					return
				}

				Component.LogInfof("removed offline: %s / %s", ev.Peer.Address(), peerID.ShortString())
			}).Unhook,
		)
	}

	if deps.AutopeeringManager.Selection() != nil {
		unhookCallbacks = append(unhookCallbacks,
			// notify the selection when a connection is closed or failed.
			deps.PeeringManager.Events.Connected.Hook(func(p *p2p.Peer, conn network.Conn) {

				if deps.AutopeeringManager.Selection() == nil {
					return
				}

				id := autopeering.ConvertPeerIDToHiveIdentityOrLog(p, Component.LogWarnf)
				if id == nil {
					return
				}

				// we block peers that are connected via manual peering in the autopeering module.
				// this ensures that no additional connections are established via autopeering.
				switch p.Relation {
				case p2p.PeerRelationKnown, p2p.PeerRelationUnknown:
					deps.AutopeeringManager.Selection().BlockNeighbor(id.ID())
				}
			}).Unhook,

			deps.PeeringManager.Events.Disconnected.Hook(func(peer *p2p.Peer, err error) {
				if deps.AutopeeringManager.Selection() == nil {
					return
				}

				id := autopeering.ConvertPeerIDToHiveIdentityOrLog(peer, Component.LogWarnf)
				if id == nil {
					return
				}

				// if a peer is disconnected, we need to remove it from the autopeering blacklist
				// so that former known and unknown peers can be autopeered.
				deps.AutopeeringManager.Selection().UnblockNeighbor(id.ID())

				if peer.Relation != p2p.PeerRelationAutopeered {
					return
				}

				Component.LogDebugf("removing: %s", peer.ID.ShortString())
				deps.AutopeeringManager.Selection().RemoveNeighbor(id.ID())
			}).Unhook,

			deps.PeeringManager.Events.RelationUpdated.Hook(func(p *p2p.Peer, oldRel p2p.PeerRelation) {
				if deps.AutopeeringManager.Selection() == nil {
					return
				}

				id := autopeering.ConvertPeerIDToHiveIdentityOrLog(p, Component.LogWarnf)
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

				Component.LogInfof("removing %s from autopeering selection protocol", p.ID.ShortString())
				deps.AutopeeringManager.Selection().RemoveNeighbor(id.ID())
			}).Unhook,

			deps.AutopeeringManager.Selection().Events().SaltUpdated.Hook(func(ev *selection.SaltUpdatedEvent) {
				Component.LogInfof("salt updated; expires=%s", ev.Public.GetExpiration().Format(time.RFC822))
			}).Unhook,

			deps.AutopeeringManager.Selection().Events().OutgoingPeering.Hook(func(ev *selection.PeeringEvent) {
				if !ev.Status {
					return
				}

				addrInfo, err := autopeering.HivePeerToAddrInfo(ev.Peer, deps.AutopeeringManager.P2PServiceKey())
				if err != nil {
					Component.LogWarnf("unable to convert outgoing selection autopeering peer to addr info: %s", err)

					return
				}

				Component.LogInfof("[outgoing peering] adding autopeering peer %s", addrInfo.ID.ShortString())

				handleSelection(ev, addrInfo, func() {
					Component.LogInfof("connecting to %s", addrInfo.ID.ShortString())
					if err := deps.PeeringManager.ConnectPeer(addrInfo, p2p.PeerRelationAutopeered); err != nil {
						Component.LogWarnf("couldn't add autopeering peer %s: %s", addrInfo.ID.ShortString(), err)
					}
				})
			}).Unhook,

			deps.AutopeeringManager.Selection().Events().IncomingPeering.Hook(func(ev *selection.PeeringEvent) {
				if !ev.Status {
					return
				}

				addrInfo, err := autopeering.HivePeerToAddrInfo(ev.Peer, deps.AutopeeringManager.P2PServiceKey())
				if err != nil {
					Component.LogWarnf("unable to convert incoming selection autopeering peer to addr info: %s", err)

					return
				}

				Component.LogInfof("[incoming peering] allow autopeering peer %s", addrInfo.ID.ShortString())

				handleSelection(ev, addrInfo, func() {
					if err := deps.PeeringManager.AllowPeer(addrInfo.ID); err != nil {
						Component.LogWarnf("couldn't allow autopeering peer %s: %s", addrInfo.ID.ShortString(), err)
					}
				})
			}).Unhook,

			deps.AutopeeringManager.Selection().Events().Dropped.Hook(func(ev *selection.DroppedEvent) {
				peerID, err := autopeering.HivePeerToPeerID(ev.Peer)
				if err != nil {
					Component.LogWarnf("unable to convert dropped autopeering peer to peerID: %s", err)

					return
				}

				Component.LogInfof("[dropped event] disconnecting %s / %s", ev.Peer.Address(), peerID.ShortString())

				if err := deps.PeeringManager.DisallowPeer(peerID); err != nil {
					Component.LogWarnf("couldn't disallow autopeering peer %s: %s", peerID.ShortString(), err)
				}

				var peerRelation p2p.PeerRelation
				deps.PeeringManager.Call(peerID, func(p *p2p.Peer) {
					peerRelation = p.Relation
				})

				if len(peerRelation) == 0 {
					Component.LogWarnf("didn't find autopeered peer %s for disconnecting", peerID.ShortString())

					return
				}

				if peerRelation != p2p.PeerRelationAutopeered {
					Component.LogWarnf("won't disconnect %s as its relation is not '%s' but '%s'", peerID.ShortString(), p2p.PeerRelationAutopeered, peerRelation)

					return
				}

				if err := deps.PeeringManager.DisconnectPeer(peerID, errors.New("removed via autopeering selection")); err != nil {
					Component.LogWarnf("couldn't disconnect selection dropped autopeer %s: %s", peerID.ShortString(), err)
				}
			}).Unhook,
		)
	}

	return lo.Batch(unhookCallbacks...)
}
