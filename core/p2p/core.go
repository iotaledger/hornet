// TODO: obviously move all this into its separate pkg
package p2p

import (
	"context"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	p2ppkg "github.com/gohornet/hornet/pkg/p2p"
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
	CorePlugin      *node.CorePlugin
	log             *logger.Logger
	deps            dependencies
	peerStoreExists bool
)

type dependencies struct {
	dig.In
	Manager              *p2ppkg.Manager
	Host                 host.Host
	NodeConfig           *configuration.Configuration `name:"nodeConfig"`
	PeeringConfig        *configuration.Configuration `name:"peeringConfig"`
	PeeringConfigManager *p2ppkg.ConfigManager
}

func provide(c *dig.Container) {
	log = logger.NewLogger(CorePlugin.Name)

	type hostdeps struct {
		dig.In

		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	type p2presult struct {
		dig.Out

		Host           host.Host
		NodePrivateKey crypto.PrivKey
	}

	if err := c.Provide(func(deps hostdeps) (p2presult, error) {

		res := p2presult{}

		peerStorePath := deps.NodeConfig.String(CfgP2PPeerStorePath)
		peerStoreExists = p2p.PeerStoreExists(peerStorePath)

		peerStore, err := p2p.NewPeerstore(peerStorePath)
		if err != nil {
			panic(err)
		}

		pubKeyFilePath := path.Join(peerStorePath, p2p.PubKeyFileName)
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
			panic(fmt.Sprintf("unable to load/create peer identity: %s", err))
		}

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
		res.NodePrivateKey = prvKey

		return res, nil
	}); err != nil {
		panic(err)
	}

	type mngdeps struct {
		dig.In

		Host   host.Host
		Config *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps mngdeps) *p2ppkg.Manager {
		return p2ppkg.NewManager(deps.Host,
			p2ppkg.WithManagerLogger(logger.NewLogger("P2P-Manager")),
			p2ppkg.WithManagerReconnectInterval(deps.Config.Duration(CfgP2PReconnectInterval), 1*time.Second),
		)
	}); err != nil {
		panic(err)
	}

	type configManagerDeps struct {
		dig.In

		PeeringConfig         *configuration.Configuration `name:"peeringConfig"`
		PeeringConfigFilePath string                       `name:"peeringConfigFilePath"`
	}

	if err := c.Provide(func(deps configManagerDeps) *p2ppkg.ConfigManager {

		p2pConfigManager := p2ppkg.NewConfigManager(func(peers []*p2ppkg.PeerConfig) error {
			if err := deps.PeeringConfig.Set(CfgPeers, peers); err != nil {
				return err
			}

			return deps.PeeringConfig.StoreFile(deps.PeeringConfigFilePath, []string{"p2p"})
		})

		// peers from peering config
		var peers []*p2ppkg.PeerConfig
		if err := deps.PeeringConfig.Unmarshal(CfgPeers, &peers); err != nil {
			panic(fmt.Sprintf("invalid peer config: %s", err))
		}

		for i, p := range peers {
			multiAddr, err := multiaddr.NewMultiaddr(p.MultiAddress)
			if err != nil {
				panic(fmt.Sprintf("invalid config peer address at pos %d: %s", i, err))
			}

			p2pConfigManager.AddPeer(multiAddr, p.Alias)
		}

		// peers from CLI arguments
		peerIDsStr := deps.PeeringConfig.Strings(CfgP2PPeers)
		peerAliases := deps.PeeringConfig.Strings(CfgP2PPeerAliases)

		applyAliases := true
		if len(peerIDsStr) != len(peerAliases) {
			log.Warnf("won't apply peer aliases: you must define aliases for all defined static peers (got %d aliases, %d peers).", len(peerAliases), len(peerIDsStr))
			applyAliases = false
		}

		for i, peerIDStr := range peerIDsStr {
			multiAddr, err := multiaddr.NewMultiaddr(peerIDStr)
			if err != nil {
				panic(fmt.Sprintf("invalid CLI peer address at pos %d: %s", i, err))
			}

			var alias string
			if applyAliases {
				alias = peerAliases[i]
			}

			p2pConfigManager.AddPeer(multiAddr, alias)
		}

		p2pConfigManager.StoreOnChange(true)

		return p2pConfigManager
	}); err != nil {
		panic(err)
	}
}

func configure() {

	// make sure nobody copies around the peer store since it contains the
	// private key of the node
	log.Infof("never share your %s folder as it contains your node's private key!", deps.NodeConfig.String(CfgP2PPeerStorePath))

	pubKeyFilePath := path.Join(deps.NodeConfig.String(CfgP2PPeerStorePath), p2p.PubKeyFileName)

	if !peerStoreExists {
		log.Infof("stored new peer identity under %s", pubKeyFilePath)
	} else {
		log.Infof("retrieved existing peer identity from %s", pubKeyFilePath)
	}

	log.Infof("peer configured, ID: %s", deps.Host.ID())
}

func run() {

	// register a daemon to disconnect all peers up on shutdown
	_ = CorePlugin.Daemon().BackgroundWorker("Manager", func(shutdownSignal <-chan struct{}) {
		log.Infof("listening on: %s", deps.Host.Addrs())
		go deps.Manager.Start(shutdownSignal)
		connectConfigKnownPeers()
		<-shutdownSignal
		if err := deps.Host.Peerstore().Close(); err != nil {
			log.Error("unable to cleanly closing peer store: %s", err)
		}
	}, shutdown.PriorityP2PManager)
}

// connects to the peers defined in the config.
func connectConfigKnownPeers() {
	for _, p := range deps.PeeringConfigManager.GetPeers() {
		multiAddr, err := multiaddr.NewMultiaddr(p.MultiAddress)
		if err != nil {
			panic(fmt.Sprintf("invalid peer address: %s", err))
		}

		addrInfo, err := peer.AddrInfoFromP2pAddr(multiAddr)
		if err != nil {
			panic(fmt.Sprintf("invalid peer address info: %s", err))
		}

		_ = deps.Manager.ConnectPeer(addrInfo, p2ppkg.PeerRelationKnown, p.Alias)
	}
}
