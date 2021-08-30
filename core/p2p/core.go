// TODO: obviously move all this into its separate pkg
package p2p

import (
	"context"
	"path/filepath"
	"time"

	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"
)

func init() {
	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
			Name:           "P2P",
			DepsFunc:       func(cDeps dependencies) { deps = cDeps },
			Params:         params,
			InitConfigPars: initConfigPars,
			Provide:        provide,
			Configure:      configure,
			Run:            run,
		},
	}
}

var (
	CorePlugin *node.CorePlugin
	deps       dependencies
)

type dependencies struct {
	dig.In
	Manager              *p2p.Manager
	Host                 host.Host
	NodeConfig           *configuration.Configuration `name:"nodeConfig"`
	PeerStoreContainer   *p2p.PeerStoreContainer
	PeeringConfig        *configuration.Configuration `name:"peeringConfig"`
	PeeringConfigManager *p2p.ConfigManager
}

func initConfigPars(c *dig.Container) {

	type cfgDeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	type cfgResult struct {
		dig.Out
		P2PDatabasePath       string   `name:"p2pDatabasePath"`
		P2PBindMultiAddresses []string `name:"p2pBindMultiAddresses"`
	}

	if err := c.Provide(func(deps cfgDeps) cfgResult {
		return cfgResult{
			P2PDatabasePath:       deps.NodeConfig.String(CfgP2PDatabasePath),
			P2PBindMultiAddresses: deps.NodeConfig.Strings(CfgP2PBindMultiAddresses),
		}
	}); err != nil {
		CorePlugin.Panic(err)
	}
}

func provide(c *dig.Container) {

	type hostDeps struct {
		dig.In
		NodeConfig            *configuration.Configuration `name:"nodeConfig"`
		DatabaseEngine        database.Engine              `name:"databaseEngine"`
		P2PDatabasePath       string                       `name:"p2pDatabasePath"`
		P2PBindMultiAddresses []string                     `name:"p2pBindMultiAddresses"`
	}

	type p2presult struct {
		dig.Out
		PeerStoreContainer *p2p.PeerStoreContainer
		NodePrivateKey     crypto.PrivKey `name:"nodePrivateKey"`
		Host               host.Host
	}

	if err := c.Provide(func(deps hostDeps) p2presult {

		res := p2presult{}

		privKeyFilePath := filepath.Join(deps.P2PDatabasePath, p2p.PrivKeyFileName)

		peerStoreContainer, err := p2p.NewPeerStoreContainer(filepath.Join(deps.P2PDatabasePath, "peers"), deps.DatabaseEngine, true)
		if err != nil {
			CorePlugin.Panic(err)
		}
		res.PeerStoreContainer = peerStoreContainer

		// TODO: temporary migration logic
		// this should be removed after some time / hornet versions (20.08.21: muXxer)
		identityPrivKey := deps.NodeConfig.String(CfgP2PIdentityPrivKey)
		migrated, err := p2p.MigrateDeprecatedPeerStore(deps.P2PDatabasePath, identityPrivKey, peerStoreContainer)
		if err != nil {
			CorePlugin.Panicf("migration of deprecated peer store failed: %s", err)
		}
		if migrated {
			CorePlugin.LogInfof(`The peer store was migrated successfully!

Your node identity private key can now be found at "%s".
`, privKeyFilePath)
		}

		// make sure nobody copies around the peer store since it contains the private key of the node
		CorePlugin.LogInfof(`WARNING: never share your "%s" folder as it contains your node's private key!`, deps.P2PDatabasePath)

		// load up the previously generated identity or create a new one
		privKey, newlyCreated, err := p2p.LoadOrCreateIdentityPrivateKey(deps.P2PDatabasePath, identityPrivKey)
		if err != nil {
			CorePlugin.Panic(err)
		}
		res.NodePrivateKey = privKey

		if newlyCreated {
			CorePlugin.LogInfof(`stored new private key for peer identity under "%s"`, privKeyFilePath)
		} else {
			CorePlugin.LogInfof(`loaded existing private key for peer identity from "%s"`, privKeyFilePath)
		}

		createdHost, err := libp2p.New(context.Background(),
			libp2p.Identity(privKey),
			libp2p.ListenAddrStrings(deps.P2PBindMultiAddresses...),
			libp2p.Peerstore(peerStoreContainer.Peerstore()),
			libp2p.DefaultTransports,
			libp2p.ConnectionManager(connmgr.NewConnManager(
				deps.NodeConfig.Int(CfgP2PConnMngLowWatermark),
				deps.NodeConfig.Int(CfgP2PConnMngHighWatermark),
				time.Minute,
			)),
			libp2p.NATPortMap(),
		)
		if err != nil {
			CorePlugin.Panicf("unable to initialize peer: %s", err)
		}
		res.Host = createdHost

		return res
	}); err != nil {
		CorePlugin.Panic(err)
	}

	type mngDeps struct {
		dig.In
		Host                      host.Host
		Config                    *configuration.Configuration `name:"nodeConfig"`
		AutopeeringRunAsEntryNode bool                         `name:"autopeeringRunAsEntryNode"`
	}

	if err := c.Provide(func(deps mngDeps) *p2p.Manager {
		if !deps.AutopeeringRunAsEntryNode {
			return p2p.NewManager(deps.Host,
				p2p.WithManagerLogger(logger.NewLogger("P2P-Manager")),
				p2p.WithManagerReconnectInterval(deps.Config.Duration(CfgP2PReconnectInterval), 1*time.Second),
			)
		}
		return nil
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

	CorePlugin.LogInfof("peer configured, ID: %s", deps.Host.ID())

	if err := CorePlugin.Daemon().BackgroundWorker("Close p2p peer database", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal

		closeDatabases := func() error {
			if err := deps.PeerStoreContainer.Flush(); err != nil {
				return err
			}

			return deps.PeerStoreContainer.Close()
		}

		CorePlugin.LogInfo("Syncing p2p peer database to disk...")
		if err := closeDatabases(); err != nil {
			CorePlugin.Panicf("Syncing p2p peer database to disk... failed: %s", err)
		}
		CorePlugin.LogInfo("Syncing p2p peer database to disk... done")
	}, shutdown.PriorityCloseDatabase); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}
}

func run() {
	if deps.Manager == nil {
		// Manager is optional, due to autopeering entry node
		return
	}

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
