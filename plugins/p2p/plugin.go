// TODO: obviously move all this into its separate pkg
package p2p

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gohornet/hornet/pkg/config"
	p2ppkg "github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	badger "github.com/ipfs/go-ds-badger"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	libp2pquic "github.com/libp2p/go-libp2p-quic-transport"
	"github.com/multiformats/go-multiaddr"
)

const (
	pubKeyFileName = "key.pub"
)

var (
	PLUGIN             = node.NewPlugin("P2P", node.Enabled, configure, run)
	log                *logger.Logger
	hostOnce           sync.Once
	selfHost           host.Host
	peeringService     *p2ppkg.PeeringService
	peeringServiceOnce sync.Once
)

// PeeringService returns the PeeringService.
func PeeringService() *p2ppkg.PeeringService {
	peeringServiceOnce.Do(func() {
		peeringService = p2ppkg.NewPeeringService(Host())

		// init PeeringService with peers from the config
		peerIDsStr := config.NodeConfig.GetStringSlice(config.CfgP2PPeers)
		for i, peerIDStr := range peerIDsStr {
			multiAddr, err := multiaddr.NewMultiaddr(peerIDStr)
			if err != nil {
				panic(fmt.Sprintf("unable to init PeeringService with peer multiaddr at pos %d: %s", i, err))
			}
			addrInfo, err := peer.AddrInfoFromP2pAddr(multiAddr)
			if err != nil {
				panic(fmt.Sprintf("unable to init PeeringService with peer address info at pos %d: %s", i, err))
			}
			peeringService.AddPeer(*addrInfo)
		}

	})
	return peeringService
}

// Host returns the host.Host instance of this node.
func Host() host.Host {
	hostOnce.Do(func() {
		ctx := context.Background()

		peerStorePath := config.NodeConfig.GetString(config.CfgP2PPeerStorePath)
		_, statPeerStorePathErr := os.Stat(peerStorePath)

		// TODO: switch out with impl. using KVStore
		badgerStore, err := badger.NewDatastore(peerStorePath, nil)
		if err != nil {
			panic(fmt.Sprintf("unable to initialize data store for peer store: %s", err))
		}

		// also takes care of this node's identity key pair
		peerStore, err := pstoreds.NewPeerstore(ctx, badgerStore, pstoreds.DefaultOpts())
		if err != nil {
			panic(fmt.Sprintf("unable to initialize peer store: %s", err))
		}

		// make sure nobody copies around the peer store since it contains the
		// private key of the node
		log.Infof("never share your %s folder as it contains your node's private key!", peerStorePath)

		// load up the previously generated identity or create a new one
		isPeerStoreNew := os.IsNotExist(statPeerStorePathErr)
		prvKey, err := loadOrCreateIdentity(isPeerStoreNew, peerStorePath, peerStore)
		if err != nil {
			panic(fmt.Sprintf("unable to load/create peer identity: %s", err))
		}

		var idht *dht.IpfsDHT
		staticPeers := config.NodeConfig.GetStringSlice(config.CfgP2PBindAddresses)

		selfHost, err = libp2p.New(ctx,
			libp2p.Identity(prvKey),
			libp2p.ListenAddrStrings(staticPeers...),
			libp2p.Peerstore(peerStore),
			libp2p.Transport(libp2pquic.NewTransport),
			libp2p.DefaultTransports,
			libp2p.ConnectionManager(connmgr.NewConnManager(
				config.NodeConfig.GetInt(config.CfgP2PConnMngLowWatermark),
				config.NodeConfig.GetInt(config.CfgP2PConnMngHighWatermark),
				time.Minute,
			)),
			libp2p.NATPortMap(),
			libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
				idht, err = dht.New(ctx, h)
				return idht, err
			}),
			libp2p.EnableAutoRelay(),
		)

		if err != nil {
			panic(fmt.Sprintf("unable to initialize peer: %s", err))
		}
	})
	return selfHost
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)
	Host()

	ps := PeeringService()

	ps.Events.Added.Attach(events.NewClosure(func(ph *p2ppkg.Peer) {
		log.Infof("added to peering service %s", ph.ID)
	}))
	ps.Events.Removed.Attach(events.NewClosure(func(ph *p2ppkg.Peer) {
		log.Infof("removed from peering service %s", ph.ID)
	}))
	ps.Events.UpdatedAddrs.Attach(events.NewClosure(func(ph *p2ppkg.Peer) {
		log.Infof("updated addresses for %s", ph.ID)
	}))
	ps.Events.UpdatedAddrs.Attach(events.NewClosure(func(ph *p2ppkg.Peer) {
		log.Infof("updated addresses for %s", ph.ID)
	}))
	ps.Events.Connected.Attach(events.NewClosure(func(ph *p2ppkg.Peer) {
		conns := Host().Network().ConnsToPeer(ph.ID)
		connsAddrsStr := make([]string, len(conns))
		for i, conn := range conns {
			connsAddrsStr[i] = conn.RemoteMultiaddr().String()
		}
		log.Infof("connected %s, connection addrs: %s", ph.ID, strings.Join(connsAddrsStr, ","))
	}))
	ps.Events.Disconnected.Attach(events.NewClosure(func(ph *p2ppkg.Peer) {
		log.Infof("disconnected %s", ph.ID)
	}))
	ps.Events.ClosedConnectionToUnknownPeer.Attach(events.NewClosure(func(peerID peer.ID) {
		log.Infof("closed connection to unknown peer %s", peerID)
	}))
	ps.Events.Reconnecting.Attach(events.NewClosure(func(ph *p2ppkg.Peer) {
		log.Infof("reconnecting to %s", ph.ID)
	}))
	ps.Events.Reconnected.Attach(events.NewClosure(func(ph *p2ppkg.Peer) {
		log.Infof("reconnected to %s", ph.ID)
	}))
	ps.Events.ReconnectFailed.Attach(events.NewClosure(func(ph *p2ppkg.Peer) {
		log.Infof("reconnect attempt failed to %s", ph.ID)
	}))

	ps.Events.ServiceStarted.Attach(events.NewClosure(func() {
		log.Info("started peering service")
	}))
	ps.Events.ServiceStopped.Attach(events.NewClosure(func() {
		log.Info("stopped peering service")
	}))

	log.Infof("peer configured, ID: %s", Host().ID())
}

func run(_ *node.Plugin) {
	p := Host()

	// register a daemon to disconnect all peers up on shutdown
	daemon.BackgroundWorker("PeeringService", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		if err := PeeringService().Stop(); err != nil {
			log.Error("unable to cleanly shutdown peering service: %s", err)
		}
		if err := Host().Peerstore().Close(); err != nil {
			log.Error("unable to cleanly closing peer store: %s", err)
		}
	}, shutdown.PriorityPeeringService)

	log.Infof("listening on: %s", p.Addrs())
	if err := PeeringService().Start(); err != nil {
		log.Errorf("unable to start PeeringService: %s", err)
	}
}

// creates a new Ed25519 based key pair or loads up the existing identity
// by reading the public key file from disk.
func loadOrCreateIdentity(peerStoreIsNew bool, peerStorePath string, peerStore peerstore.Peerstore) (crypto.PrivKey, error) {
	pubKeyFilePath := path.Join(peerStorePath, pubKeyFileName)
	if peerStoreIsNew {
		return createIdentity(pubKeyFilePath)
	}

	return loadExistingIdentity(pubKeyFilePath, peerStore)
}

// creates a new Ed25519 based identity and saves the public key
// as a separate file next to the peer store data.
func createIdentity(pubKeyFilePath string) (crypto.PrivKey, error) {
	log.Info("generating a new peer identity...")
	sk, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		return nil, fmt.Errorf("unable to generate Ed25519 key pair for peer identity: %w", err)
	}

	// even though the crypto.PrivKey is going to get stored
	// within the peer store, there is no way to retrieve the node's
	// identity via the peer store, so we must save the public key
	// separately to retrieve it later again
	// https://discuss.libp2p.io/t/generating-peer-id/111/2
	pubKeyPb, err := crypto.MarshalPublicKey(sk.GetPublic())
	if err != nil {
		return nil, fmt.Errorf("unable to marshal public key for public key identity file: %w", err)
	}

	if err := ioutil.WriteFile(pubKeyFilePath, pubKeyPb, 0666); err != nil {
		return nil, fmt.Errorf("unable to save public key identity file: %w", err)
	}

	log.Infof("stored public key under %s", pubKeyFilePath)
	return sk, nil
}

// loads an existing identity by reading in the public key from the public key identity file
// and then retrieving the associated private key from the given Peerstore.
func loadExistingIdentity(pubKeyFilePath string, peerStore peerstore.Peerstore) (crypto.PrivKey, error) {
	log.Infof("retrieving existing peer identity from %s", pubKeyFilePath)
	existingPubKeyBytes, err := ioutil.ReadFile(pubKeyFilePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read public key identity file: %w", err)
	}

	pubKey, err := crypto.UnmarshalPublicKey(existingPubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal public key from public key identity file: %w", err)
	}
	peerID, err := peer.IDFromPublicKey(pubKey)

	// retrieve this node's private key from the peer store
	return peerStore.PrivKey(peerID), nil
}
