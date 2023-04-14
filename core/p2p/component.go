package p2p

import (
	"context"
	"path/filepath"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/app/configuration"
	hivep2p "github.com/iotaledger/hive.go/crypto/p2p"
	hivedb "github.com/iotaledger/hive.go/kvstore/database"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/p2p"
)

func init() {
	Component = &app.Component{
		Name:             "P2P",
		DepsFunc:         func(cDeps dependencies) { deps = cDeps },
		Params:           params,
		InitConfigParams: initConfigParams,
		Provide:          provide,
		Configure:        configure,
		Run:              run,
	}
}

var (
	Component *app.Component
	deps      dependencies
)

type dependencies struct {
	dig.In
	PeeringManager       *p2p.Manager
	Host                 host.Host
	PeerStoreContainer   *p2p.PeerStoreContainer
	PeeringConfig        *configuration.Configuration `name:"peeringConfig"`
	PeeringConfigManager *p2p.ConfigManager
}

func initConfigParams(c *dig.Container) error {

	type cfgResult struct {
		dig.Out
		P2PDatabasePath       string   `name:"p2pDatabasePath"`
		P2PBindMultiAddresses []string `name:"p2pBindMultiAddresses"`
	}

	if err := c.Provide(func() cfgResult {
		return cfgResult{
			P2PDatabasePath:       ParamsP2P.Database.Path,
			P2PBindMultiAddresses: ParamsP2P.BindMultiAddresses,
		}
	}); err != nil {
		Component.LogPanic(err)
	}

	return nil
}

func provide(c *dig.Container) error {

	type hostDeps struct {
		dig.In
		DatabaseEngine        hivedb.Engine `name:"databaseEngine"`
		P2PDatabasePath       string        `name:"p2pDatabasePath"`
		P2PBindMultiAddresses []string      `name:"p2pBindMultiAddresses"`
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
			Component.LogPanic(err)
		}
		res.PeerStoreContainer = peerStoreContainer

		// make sure nobody copies around the peer store since it contains the private key of the node
		Component.LogInfof(`WARNING: never share your "%s" folder as it contains your node's private key!`, deps.P2PDatabasePath)

		// load up the previously generated identity or create a new one
		privKey, newlyCreated, err := hivep2p.LoadOrCreateIdentityPrivateKey(privKeyFilePath, ParamsP2P.IdentityPrivateKey)
		if err != nil {
			Component.LogPanic(err)
		}
		res.NodePrivateKey = privKey

		if newlyCreated {
			Component.LogInfof(`stored new private key for peer identity under "%s"`, privKeyFilePath)
		} else {
			Component.LogInfof(`loaded existing private key for peer identity from "%s"`, privKeyFilePath)
		}

		connManager, err := connmgr.NewConnManager(
			ParamsP2P.ConnectionManager.LowWatermark,
			ParamsP2P.ConnectionManager.HighWatermark,
			connmgr.WithGracePeriod(time.Minute),
		)
		if err != nil {
			Component.LogPanicf("unable to initialize connection manager: %s", err)
		}

		createdHost, err := libp2p.New(libp2p.Identity(privKey),
			libp2p.ListenAddrStrings(deps.P2PBindMultiAddresses...),
			libp2p.Peerstore(peerStoreContainer.Peerstore()),
			libp2p.Transport(tcp.NewTCPTransport),
			libp2p.ConnectionManager(connManager),
			libp2p.NATPortMap(),
		)
		if err != nil {
			Component.LogPanicf("unable to initialize peer: %s", err)
		}
		res.Host = createdHost

		return res
	}); err != nil {
		Component.LogPanic(err)
	}

	type mngDeps struct {
		dig.In
		Host                      host.Host
		AutopeeringRunAsEntryNode bool `name:"autopeeringRunAsEntryNode"`
	}

	if err := c.Provide(func(deps mngDeps) *p2p.Manager {
		if !deps.AutopeeringRunAsEntryNode {
			return p2p.NewManager(deps.Host,
				p2p.WithManagerLogger(Component.App().NewLogger("P2P-Manager")),
				p2p.WithManagerReconnectInterval(ParamsP2P.ReconnectInterval, 1*time.Second),
			)
		}

		return nil
	}); err != nil {
		Component.LogPanic(err)
	}

	type configManagerDeps struct {
		dig.In
		PeeringConfig         *configuration.Configuration `name:"peeringConfig"`
		PeeringConfigFilePath *string                      `name:"peeringConfigFilePath"`
	}

	if err := c.Provide(func(deps configManagerDeps) *p2p.ConfigManager {

		p2pConfigManager := p2p.NewConfigManager(func(peers []*p2p.PeerConfig) error {
			if err := deps.PeeringConfig.Set(CfgPeers, peers); err != nil {
				return err
			}

			return deps.PeeringConfig.StoreFile(*deps.PeeringConfigFilePath, 0o600, []string{"p2p"})
		})

		// peers from peering config
		var peers []*p2p.PeerConfig
		if err := deps.PeeringConfig.Unmarshal(CfgPeers, &peers); err != nil {
			Component.LogPanicf("invalid peer config: %s", err)
		}

		for i, p := range peers {
			multiAddr, err := multiaddr.NewMultiaddr(p.MultiAddress)
			if err != nil {
				Component.LogPanicf("invalid config peer address at pos %d: %s", i, err)
			}

			if err = p2pConfigManager.AddPeer(multiAddr, p.Alias); err != nil {
				Component.LogWarnf("unable to add peer to config manager %s: %s", p.MultiAddress, err)
			}
		}

		// peers from CLI arguments
		peerIDsStr := ParamsPeers.Peers
		peerAliases := ParamsPeers.PeerAliases

		applyAliases := true
		if len(peerIDsStr) != len(peerAliases) {
			Component.LogWarnf("won't apply peer aliases: you must define aliases for all defined static peers (got %d aliases, %d peers).", len(peerAliases), len(peerIDsStr))
			applyAliases = false
		}

		peerAdded := false
		for i, peerIDStr := range peerIDsStr {
			multiAddr, err := multiaddr.NewMultiaddr(peerIDStr)
			if err != nil {
				Component.LogPanicf("invalid CLI peer address at pos %d: %s", i, err)
			}

			var alias string
			if applyAliases {
				alias = peerAliases[i]
			}

			if err = p2pConfigManager.AddPeer(multiAddr, alias); err != nil {
				Component.LogWarnf("unable to add peer to config manager %s: %s", peerIDStr, err)
			}

			peerAdded = true
		}

		p2pConfigManager.StoreOnChange(true)

		if peerAdded {
			if err := p2pConfigManager.Store(); err != nil {
				Component.LogWarnf("failed to store peering config: %s", err)
			}
		}

		return p2pConfigManager
	}); err != nil {
		Component.LogPanic(err)
	}

	return nil
}

func configure() error {

	Component.LogInfof("peer configured, ID: %s", deps.Host.ID())

	if err := Component.Daemon().BackgroundWorker("Close p2p peer database", func(ctx context.Context) {
		<-ctx.Done()

		closeDatabases := func() error {
			if err := deps.PeerStoreContainer.Flush(); err != nil {
				return err
			}

			return deps.PeerStoreContainer.Close()
		}

		Component.LogInfo("Syncing p2p peer database to disk ...")
		if err := closeDatabases(); err != nil {
			Component.LogPanicf("Syncing p2p peer database to disk ... failed: %s", err)
		}
		Component.LogInfo("Syncing p2p peer database to disk ... done")
	}, daemon.PriorityCloseDatabase); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}

func run() error {
	if deps.PeeringManager == nil {
		// Manager is optional, due to autopeering entry node
		return nil
	}

	// register a daemon to disconnect all peers up on shutdown
	if err := Component.Daemon().BackgroundWorker("Manager", func(ctx context.Context) {
		Component.LogInfof("listening on: %s", deps.Host.Addrs())
		go deps.PeeringManager.Start(ctx)
		connectConfigKnownPeers()
		<-ctx.Done()
		if err := deps.Host.Peerstore().Close(); err != nil {
			Component.LogError("unable to cleanly closing peer store: %s", err)
		}
	}, daemon.PriorityP2PManager); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}

// connects to the peers defined in the config.
func connectConfigKnownPeers() {
	for _, p := range deps.PeeringConfigManager.Peers() {
		multiAddr, err := multiaddr.NewMultiaddr(p.MultiAddress)
		if err != nil {
			Component.LogPanicf("invalid peer address: %s", err)
		}

		addrInfo, err := peer.AddrInfoFromP2pAddr(multiAddr)
		if err != nil {
			Component.LogPanicf("invalid peer address info: %s", err)
		}

		if err = deps.PeeringManager.ConnectPeer(addrInfo, p2p.PeerRelationKnown, p.Alias); err != nil {
			Component.LogInfof("can't connect to peer (%s): %s", multiAddr.String(), err)
		}
	}
}
