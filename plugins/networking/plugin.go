// TODO: obviously move all this into its separate pkg
package networking

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"

	"github.com/gohornet/hornet/pkg/config"
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
)

const (
	pubKeyFileName = "key.pub"
)

var (
	PLUGIN   = node.NewPlugin("Networking", node.Enabled, configure, run)
	log      *logger.Logger
	hostOnce sync.Once
	selfHost host.Host
)

// Peer returns the libp2p host instance of this node.
func Peer() host.Host {
	hostOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		peerStorePath := config.NodeConfig.GetString(config.CfgLibp2pPeerStorePath)
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
		log.Infof("never share your %s folder as it contains your node's private key", peerStorePath)

		// load up the previously generated identity or create a new one
		isPeerStoreNew := os.IsNotExist(statPeerStorePathErr)
		prvKey, err := loadOrCreateIdentity(isPeerStoreNew, peerStorePath, peerStore)
		if err != nil {
			panic(fmt.Sprintf("unable to load/create peer identity: %s", err))
		}

		var idht *dht.IpfsDHT
		selfHost, err = libp2p.New(ctx,
			libp2p.Identity(prvKey),
			libp2p.ListenAddrStrings(),
			libp2p.Peerstore(peerStore),
			libp2p.ConnectionManager(connmgr.NewConnManager(
				100,
				400,
				time.Minute,
			)),
			// attempt to open ports using uPNP for NATed hosts.
			libp2p.NATPortMap(),
			// let this host use the DHT to find other hosts
			libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
				idht, err = dht.New(ctx, h)
				return idht, err
			}),
			// let this host use relays and advertise itself on relays if
			// it finds it is behind NAT. Use libp2p.Relay(options...) to
			// enable active relays and more.
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
	Peer()
	log.Info("configuring network")
}

func run(_ *node.Plugin) {
	p := Peer()

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
