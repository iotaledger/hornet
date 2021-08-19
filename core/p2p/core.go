// TODO: obviously move all this into its separate pkg
package p2p

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"
)

func init() {
	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
			Name:      "P2P",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	CorePlugin *node.CorePlugin
	deps       dependencies

	peerStoreExists bool
)

type dependencies struct {
	dig.In
	Manager              *p2p.Manager
	Host                 host.Host
	NodeConfig           *configuration.Configuration `name:"nodeConfig"`
	PeeringConfig        *configuration.Configuration `name:"peeringConfig"`
	PeeringConfigManager *p2p.ConfigManager
}

func provide(c *dig.Container) {
	type hostdeps struct {
		dig.In

		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	type p2presult struct {
		dig.Out

		P2PDatabasePath string `name:"p2pDatabasePath"`
		NodePrivateKey  crypto.PrivKey `name:"nodePrivateKey"`
		Host            host.Host
	}

	if err := c.Provide(func(deps hostdeps) (p2presult, error) {

		res := p2presult{}

		p2pDatabasePath := deps.NodeConfig.String(CfgP2PDatabasePath)
		res.P2PDatabasePath = p2pDatabasePath

		pubKeyFilePath := filepath.Join(p2pDatabasePath, p2p.PubKeyFileName)

		peerStorePath := p2pDatabasePath
		peerStoreExists = p2p.PeerStoreExists(peerStorePath)

		peerStore, err := p2p.NewPeerstore(peerStorePath)
		if err != nil {
			CorePlugin.Panic(err)
		}

		identityPrivKey := deps.NodeConfig.String(CfgP2PIdentityPrivKey)

		// load up the previously generated identity or create a new one
		var prvKey crypto.PrivKey
		if !peerStoreExists {
			prvKey, err = p2p.CreateIdentity(pubKeyFilePath, identityPrivKey)
		} else {
			var peerID peer.ID
			peerID, err = p2p.LoadIdentityFromFile(pubKeyFilePath)
			if err == nil {
				prvKey, err = p2p.LoadPrivateKeyFromStore(peerID, peerStore, identityPrivKey)
			}
		}
		if err != nil {
			if errors.Is(err, p2p.ErrPrivKeyInvalid) {
				err = fmt.Errorf("config parameter '%s' contains an invalid private key", CfgP2PIdentityPrivKey)
			}
			CorePlugin.Panicf("unable to load/create peer identity: %s", err)
		}
		res.NodePrivateKey = prvKey

		createdHost, err := libp2p.New(context.Background(),
			libp2p.Identity(prvKey),
			libp2p.ListenAddrStrings(deps.NodeConfig.Strings(CfgP2PBindMultiAddresses)...),
			libp2p.Peerstore(peerStore),
			libp2p.DefaultTransports,
			libp2p.ConnectionManager(connmgr.NewConnManager(
				deps.NodeConfig.Int(CfgP2PConnMngLowWatermark),
				deps.NodeConfig.Int(CfgP2PConnMngHighWatermark),
				time.Minute,
			)),
			libp2p.NATPortMap(),
		)
		if err != nil {
			return res, fmt.Errorf("unable to initialize peer: %w", err)
		}

		res.Host = createdHost

		return res, nil
	}); err != nil {
		CorePlugin.Panic(err)
	}

	type mngdeps struct {
		dig.In

		Host   host.Host
		Config *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps mngdeps) *p2p.Manager {
		return p2p.NewManager(deps.Host,
			p2p.WithManagerLogger(logger.NewLogger("P2P-Manager")),
			p2p.WithManagerReconnectInterval(deps.Config.Duration(CfgP2PReconnectInterval), 1*time.Second),
		)
	}); err != nil {
		CorePlugin.Panic(err)
	}

	type configManagerDeps struct {
		dig.In

		PeeringConfig         *configuration.Configuration `name:"peeringConfig"`
		PeeringConfigFilePath string                       `name:"peeringConfigFilePath"`
	}

	if err := c.Provide(func(deps configManagerDeps) *p2p.ConfigManager {

		p2pConfigManager := p2p.NewConfigManager(func(peers []*p2p.PeerConfig) error {
			if err := deps.PeeringConfig.Set(CfgPeers, peers); err != nil {
				return err
			}

			return deps.PeeringConfig.StoreFile(deps.PeeringConfigFilePath, []string{"p2p"})
		})

		// peers from peering config
		var peers []*p2p.PeerConfig
		if err := deps.PeeringConfig.Unmarshal(CfgPeers, &peers); err != nil {
			CorePlugin.Panicf("invalid peer config: %s", err)
		}

		for i, p := range peers {
			multiAddr, err := multiaddr.NewMultiaddr(p.MultiAddress)
			if err != nil {
				CorePlugin.Panicf("invalid config peer address at pos %d: %s", i, err)
			}

			if err = p2pConfigManager.AddPeer(multiAddr, p.Alias); err != nil {
				CorePlugin.LogWarnf("unable to add peer to config manager %s: %s", p.MultiAddress, err)
			}
		}

		// peers from CLI arguments
		peerIDsStr := deps.PeeringConfig.Strings(CfgP2PPeers)
		peerAliases := deps.PeeringConfig.Strings(CfgP2PPeerAliases)

		applyAliases := true
		if len(peerIDsStr) != len(peerAliases) {
			CorePlugin.LogWarnf("won't apply peer aliases: you must define aliases for all defined static peers (got %d aliases, %d peers).", len(peerAliases), len(peerIDsStr))
			applyAliases = false
		}

		for i, peerIDStr := range peerIDsStr {
			multiAddr, err := multiaddr.NewMultiaddr(peerIDStr)
			if err != nil {
				CorePlugin.Panicf("invalid CLI peer address at pos %d: %s", i, err)
			}

			var alias string
			if applyAliases {
				alias = peerAliases[i]
			}

			if err = p2pConfigManager.AddPeer(multiAddr, alias); err != nil {
				CorePlugin.LogWarnf("unable to add peer to config manager %s: %s", peerIDStr, err)
			}
		}

		p2pConfigManager.StoreOnChange(true)

		return p2pConfigManager
	}); err != nil {
		CorePlugin.Panic(err)
	}
}

func configure() {

	// make sure nobody copies around the peer store since it contains the
	// private key of the node
	CorePlugin.LogInfof("never share your %s folder as it contains your node's private key!", deps.NodeConfig.String(CfgP2PDatabasePath))

	pubKeyFilePath := filepath.Join(deps.NodeConfig.String(CfgP2PDatabasePath), p2p.PubKeyFileName)

	if !peerStoreExists {
		CorePlugin.LogInfof("stored new peer identity under %s", pubKeyFilePath)
	} else {
		CorePlugin.LogInfof("retrieved existing peer identity from %s", pubKeyFilePath)
	}

	CorePlugin.LogInfof("peer configured, ID: %s", deps.Host.ID())
}

func run() {

	// register a daemon to disconnect all peers up on shutdown
	if err := CorePlugin.Daemon().BackgroundWorker("Manager", func(shutdownSignal <-chan struct{}) {
		CorePlugin.LogInfof("listening on: %s", deps.Host.Addrs())
		go deps.Manager.Start(shutdownSignal)
		connectConfigKnownPeers()
		<-shutdownSignal
		if err := deps.Host.Peerstore().Close(); err != nil {
			CorePlugin.LogError("unable to cleanly closing peer store: %s", err)
		}
	}, shutdown.PriorityP2PManager); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}
}

// connects to the peers defined in the config.
func connectConfigKnownPeers() {
	for _, p := range deps.PeeringConfigManager.Peers() {
		multiAddr, err := multiaddr.NewMultiaddr(p.MultiAddress)
		if err != nil {
			CorePlugin.Panicf("invalid peer address: %s", err)
		}

		addrInfo, err := peer.AddrInfoFromP2pAddr(multiAddr)
		if err != nil {
			CorePlugin.Panicf("invalid peer address info: %s", err)
		}

		if err = deps.Manager.ConnectPeer(addrInfo, p2p.PeerRelationKnown, p.Alias); err != nil {
			CorePlugin.LogInfof("can't connect to peer (%s): %s", multiAddr.String(), err)
		}
	}
}
