package p2p

import (
	"context"
	stded25519 "crypto/ed25519"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"

	badger "github.com/ipfs/go-ds-badger"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/utils"
)

const (
	PubKeyFileName = "key.pub"
)

var (
	ErrPrivKeyInvalid = errors.New("invalid private key")
	ErrNoPrivKeyFound = errors.New("no private key found")
)

// PeerStoreExists checks if files exists in the peer store folder.
func PeerStoreExists(peerStorePath string) bool {
	if _, statPeerStorePathErr := os.Stat(peerStorePath); os.IsNotExist(statPeerStorePathErr) {
		return false
	}

	// directory exists, check if it contains files (e.g. for docker setups)
	dir, err := os.Open(peerStorePath)
	if err != nil {
		return false
	}
	defer dir.Close()

	if _, err = dir.Readdirnames(1); err == io.EOF {
		// directory doesn't contain files
		return false
	}

	return true
}

// NewPeerstore creates a peerstore using badger DB.
func NewPeerstore(peerStorePath string) (peerstore.Peerstore, error) {
	// TODO: switch out with impl. using KVStore
	defaultOpts := badger.DefaultOptions

	// needed under Windows otherwise peer store is 'corrupted' after a restart
	defaultOpts.Truncate = runtime.GOOS == "windows"

	badgerStore, err := badger.NewDatastore(peerStorePath, &defaultOpts)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize data store for peer store: %w", err)
	}

	// also takes care of this node's identity key pair
	peerStore, err := pstoreds.NewPeerstore(context.Background(), badgerStore, pstoreds.DefaultOpts())
	if err != nil {
		return nil, fmt.Errorf("unable to initialize peer store: %w", err)
	}

	return peerStore, nil
}

// ParsePrivateKeyFromString parses the libp2p private key from a string.
func ParsePrivateKeyFromString(identityPrivKey string) (crypto.PrivKey, error) {
	if identityPrivKey == "" {
		return nil, ErrNoPrivKeyFound
	}

	prvKey, err := utils.ParseEd25519PrivateKeyFromString(identityPrivKey)
	if err != nil {
		return nil, ErrPrivKeyInvalid
	}

	stdPrvKey := stded25519.PrivateKey(prvKey)
	p2pPrvKey, _, err := crypto.KeyPairFromStdKey(&stdPrvKey)
	if err != nil {
		return nil, fmt.Errorf("unable to load Ed25519 key pair for peer identity: %w", err)
	}

	return p2pPrvKey, nil
}

// CreateIdentity creates a new Ed25519 based identity and saves the public key
// as a separate file next to the peer store data.
func CreateIdentity(pubKeyFilePath string, identityPrivKey string) (crypto.PrivKey, error) {

	prvKey, err := ParsePrivateKeyFromString(identityPrivKey)
	if err != nil {
		if !errors.Is(err, ErrNoPrivKeyFound) {
			return nil, err
		}

		prvKey, _, err = crypto.GenerateKeyPair(crypto.Ed25519, -1)
		if err != nil {
			return nil, fmt.Errorf("unable to generate Ed25519 key pair for peer identity: %w", err)
		}
	}

	// even though the crypto.PrivKey is going to get stored
	// within the peer store, there is no way to retrieve the node's
	// identity via the peer store, so we must save the public key
	// separately to retrieve it later again
	// https://discuss.libp2p.io/t/generating-peer-id/111/2
	pubKey, err := crypto.MarshalPublicKey(prvKey.GetPublic())
	if err != nil {
		return nil, fmt.Errorf("unable to marshal public key for public key identity file: %w", err)
	}

	if err := ioutil.WriteFile(pubKeyFilePath, pubKey, 0666); err != nil {
		return nil, fmt.Errorf("unable to save public key identity file: %w", err)
	}

	return prvKey, nil
}

// LoadIdentityFromFile loads the public key from a file and returns the p2p identity.
func LoadIdentityFromFile(pubKeyFilePath string) (peer.ID, error) {
	existingPubKeyBytes, err := ioutil.ReadFile(pubKeyFilePath)
	if err != nil {
		return "", fmt.Errorf("unable to read public key identity file: %w", err)
	}

	pubKey, err := crypto.UnmarshalPublicKey(existingPubKeyBytes)
	if err != nil {
		return "", fmt.Errorf("unable to unmarshal public key from public key identity file: %w", err)
	}

	peerID, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return "", fmt.Errorf("unable to convert read public key to peer ID: %w", err)
	}

	return peerID, nil
}

// LoadPrivateKeyFromStore loads an existing private key from the given Peerstore with the given peer identity.
// An optional private key can be passed to check if the result matches.
func LoadPrivateKeyFromStore(peerID peer.ID, peerStore peerstore.Peerstore, identityPrivKey ...string) (crypto.PrivKey, error) {

	// retrieve this node's private key from the peer store
	storedPrivKey := peerStore.PrivKey(peerID)

	if len(identityPrivKey) > 0 {
		// load an optional private key from the config and compare it to the stored private key
		prvKey, err := ParsePrivateKeyFromString(identityPrivKey[0])
		if err != nil {
			if !errors.Is(err, ErrNoPrivKeyFound) {
				return nil, err
			}

			return storedPrivKey, nil
		}

		if !storedPrivKey.Equals(prvKey) {
			storedPrivKeyBytes, _ := storedPrivKey.Bytes()
			configPrivKeyBytes, _ := prvKey.Bytes()
			return nil, fmt.Errorf("stored Ed25519 private key (%s) for peer identity doesn't match private key in config (%s)", hex.EncodeToString(storedPrivKeyBytes[:]), hex.EncodeToString(configPrivKeyBytes[:]))
		}
	}

	return storedPrivKey, nil
}
