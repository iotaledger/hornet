package p2p

import (
	"bytes"
	"context"
	stded25519 "crypto/ed25519"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ipfs/go-datastore/query"
	badger "github.com/ipfs/go-ds-badger"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/utils"
	kvstoreds "github.com/iotaledger/go-ds-kvstore"
	"github.com/iotaledger/hive.go/kvstore"
)

const (
	DeprecatedPubKeyFileName = "key.pub"
	PrivKeyFileName          = "identity.key"
)

var (
	ErrPrivKeyInvalid = errors.New("invalid private key")
	ErrNoPrivKeyFound = errors.New("no private key found")
)

// PeerStoreContainer is a container for a libp2p peer store.
type PeerStoreContainer struct {
	store     kvstore.KVStore
	peerStore peerstore.Peerstore
}

// Peerstore returns the libp2p peer store from the container.
func (psc *PeerStoreContainer) Peerstore() peerstore.Peerstore {
	return psc.peerStore
}

// Flush persists all outstanding write operations to disc.
func (psc *PeerStoreContainer) Flush() error {
	return psc.store.Flush()
}

// Close flushes all outstanding write operations and closes the store.
func (psc *PeerStoreContainer) Close() error {
	psc.peerStore.Close()

	if err := psc.store.Flush(); err != nil {
		return err
	}

	return psc.store.Close()
}

// NewPeerStoreContainer creates a peerstore using kvstore.
func NewPeerStoreContainer(peerStorePath string, dbEngine database.Engine, createDatabaseIfNotExists bool) (*PeerStoreContainer, error) {

	dirPath := filepath.Dir(peerStorePath)

	if createDatabaseIfNotExists {
		if err := os.MkdirAll(dirPath, 0700); err != nil {
			return nil, fmt.Errorf("could not create peer store database dir '%s': %w", dirPath, err)
		}
	}

	store, err := database.StoreWithDefaultSettings(peerStorePath, createDatabaseIfNotExists, dbEngine)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize peer store database: %w", err)
	}

	peerStore, err := pstoreds.NewPeerstore(context.Background(), kvstoreds.NewDatastore(store), pstoreds.DefaultOpts())
	if err != nil {
		return nil, fmt.Errorf("unable to initialize peer store: %w", err)
	}

	return &PeerStoreContainer{
		store:     store,
		peerStore: peerStore,
	}, nil
}

// parseEd25519PrivateKeyFromString parses an Ed25519 private key from a hex encoded string.
func parseEd25519PrivateKeyFromString(identityPrivKey string) (crypto.PrivKey, error) {
	if identityPrivKey == "" {
		return nil, ErrNoPrivKeyFound
	}

	hivePrivKey, err := utils.ParseEd25519PrivateKeyFromString(identityPrivKey)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %w", ErrPrivKeyInvalid)
	}

	stdPrvKey := stded25519.PrivateKey(hivePrivKey)
	p2pPrvKey, _, err := crypto.KeyPairFromStdKey(&stdPrvKey)
	if err != nil {
		return nil, fmt.Errorf("unable to convert private key: %w", err)
	}

	return p2pPrvKey, nil
}

// ReadEd25519PrivateKeyFromPEMFile reads an Ed25519 private key from a file with PEM format.
func ReadEd25519PrivateKeyFromPEMFile(filepath string) (crypto.PrivKey, error) {

	pemPrivateBlockBytes, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %w", err)
	}

	pemPrivateBlock, _ := pem.Decode(pemPrivateBlockBytes)
	if pemPrivateBlock == nil {
		return nil, fmt.Errorf("unable to decode private key: %w", err)
	}

	stdCryptoPrvKey, err := x509.ParsePKCS8PrivateKey(pemPrivateBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %w", err)
	}

	stdPrvKey, ok := stdCryptoPrvKey.(stded25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("unable to type assert private key: %w", err)
	}

	privKey, err := crypto.UnmarshalEd25519PrivateKey((stdPrvKey)[:])
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal private key: %w", err)
	}

	return privKey, nil
}

// WriteEd25519PrivateKeyToPEMFile stores an Ed25519 private key to a file with PEM format.
func WriteEd25519PrivateKeyToPEMFile(filepath string, privateKey crypto.PrivKey) error {

	stdCryptoPrvKey, err := crypto.PrivKeyToStdKey(privateKey)
	if err != nil {
		return fmt.Errorf("unable to convert private key: %w", err)
	}

	stdPrvKey, ok := stdCryptoPrvKey.(*stded25519.PrivateKey)
	if !ok {
		return fmt.Errorf("unable to type assert private key: %w", err)
	}

	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(*stdPrvKey)
	if err != nil {
		return fmt.Errorf("unable to mashal private key: %w", err)
	}

	pemPrivateBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	}

	var pemBuffer bytes.Buffer
	if err := pem.Encode(&pemBuffer, pemPrivateBlock); err != nil {
		return fmt.Errorf("unable to encode private key: %w", err)
	}

	if err := utils.WriteToFile(filepath, pemBuffer.Bytes(), 0660); err != nil {
		return fmt.Errorf("unable to write private key: %w", err)
	}

	return nil
}

// LoadOrCreateIdentityPrivateKey loads an existing Ed25519 based identity private key
// or creates a new one and stores it as a PEM file in the p2p store folder.
func LoadOrCreateIdentityPrivateKey(p2pStorePath string, identityPrivKey string) (crypto.PrivKey, bool, error) {

	privKeyFromConfig, err := parseEd25519PrivateKeyFromString(identityPrivKey)
	if err != nil {
		if errors.Is(err, ErrPrivKeyInvalid) {
			return nil, false, errors.New("configuration contains an invalid private key")
		}

		if !errors.Is(err, ErrNoPrivKeyFound) {
			return nil, false, fmt.Errorf("unable to parse private key from config: %w", err)
		}
	}

	privKeyFilePath := filepath.Join(p2pStorePath, PrivKeyFileName)

	_, err = os.Stat(privKeyFilePath)
	switch {
	case err == nil || os.IsExist(err):
		// private key already exists, load and return it
		privKey, err := ReadEd25519PrivateKeyFromPEMFile(privKeyFilePath)
		if err != nil {
			return nil, false, fmt.Errorf("unable to load Ed25519 private key for peer identity: %w", err)
		}

		if privKeyFromConfig != nil && !privKeyFromConfig.Equals(privKey) {
			storedPrivKeyBytes, err := crypto.MarshalPrivateKey(privKey)
			if err != nil {
				return nil, false, fmt.Errorf("unable to marshal stored Ed25519 private key for peer identity: %w", err)
			}
			configPrivKeyBytes, err := crypto.MarshalPrivateKey(privKeyFromConfig)
			if err != nil {
				return nil, false, fmt.Errorf("unable to marshal configured Ed25519 private key for peer identity: %w", err)
			}

			return nil, false, fmt.Errorf("stored Ed25519 private key (%s) for peer identity doesn't match private key in config (%s)", hex.EncodeToString(storedPrivKeyBytes[:]), hex.EncodeToString(configPrivKeyBytes[:]))
		}

		return privKey, false, nil

	case os.IsNotExist(err):
		var privKey crypto.PrivKey

		if privKeyFromConfig != nil {
			privKey = privKeyFromConfig
		} else {
			// private key does not exist, create a new one
			privKey, _, err = crypto.GenerateKeyPair(crypto.Ed25519, -1)
			if err != nil {
				return nil, false, fmt.Errorf("unable to generate Ed25519 private key for peer identity: %w", err)
			}
		}
		if err := WriteEd25519PrivateKeyToPEMFile(privKeyFilePath, privKey); err != nil {
			return nil, false, fmt.Errorf("unable to store private key file for peer identity: %w", err)
		}
		return privKey, true, nil

	default:
		return nil, false, fmt.Errorf("unable to check private key file for peer identity (%s): %w", privKeyFilePath, err)
	}
}

// MigrateDeprecatedPeerStore extracts the old peer identity private key from the configuration or peer store,
// migrates the old database and stores the private key in a new file with PEM format.
func MigrateDeprecatedPeerStore(p2pStorePath string, identityPrivKey string, newPeerStoreContainer *PeerStoreContainer) (bool, error) {

	privKeyFilePath := filepath.Join(p2pStorePath, PrivKeyFileName)

	_, err := os.Stat(privKeyFilePath)
	switch {
	case err == nil || os.IsExist(err):
		// migration not necessary since the private key file already exists
		return false, nil
	case os.IsNotExist(err):
		// migration maybe necessary
	default:
		return false, fmt.Errorf("unable to check private key file for peer identity (%s): %w", privKeyFilePath, err)
	}

	deprecatedPubKeyFilePath := filepath.Join(p2pStorePath, DeprecatedPubKeyFileName)
	if _, err := os.Stat(deprecatedPubKeyFilePath); err != nil {
		if os.IsNotExist(err) {
			// migration not necessary since no old public key file exists
			return false, nil
		}

		return false, fmt.Errorf("unable to check deprecated public key file for peer identity (%s): %w", deprecatedPubKeyFilePath, err)
	}

	// migrates the deprecated badger DB peerstore to the new kvstore based peerstore.
	migrateDeprecatedPeerStore := func(deprecatedPeerStorePath string, newStore kvstore.KVStore) error {
		defaultOpts := badger.DefaultOptions

		// needed under Windows otherwise peer store is 'corrupted' after a restart
		defaultOpts.Truncate = runtime.GOOS == "windows"

		badgerStore, err := badger.NewDatastore(deprecatedPeerStorePath, &defaultOpts)
		if err != nil {
			return fmt.Errorf("unable to initialize data store for deprecated peer store: %w", err)
		}
		defer func() { _ = badgerStore.Close() }()

		results, err := badgerStore.Query(query.Query{})
		if err != nil {
			return fmt.Errorf("unable to query deprecated peer store: %w", err)
		}

		for res := range results.Next() {
			if err := newStore.Set([]byte(res.Key), res.Value); err != nil {
				return fmt.Errorf("unable to migrate data to new peer store: %w", err)
			}
		}
		if err := newStore.Flush(); err != nil {
			return fmt.Errorf("unable to flush new peer store: %w", err)
		}

		return nil
	}

	if err := migrateDeprecatedPeerStore(p2pStorePath, newPeerStoreContainer.store); err != nil {
		return false, err
	}

	privKey, err := parseEd25519PrivateKeyFromString(identityPrivKey)
	if err != nil {
		if errors.Is(err, ErrPrivKeyInvalid) {
			return false, errors.New("configuration contains an invalid private key")
		}

		if !errors.Is(err, ErrNoPrivKeyFound) {
			return false, fmt.Errorf("unable to parse private key from config: %w", err)
		}

		// there was no private key specified, retrieve it from the peer store with the public key from the deprecated file
		existingPubKeyBytes, err := ioutil.ReadFile(deprecatedPubKeyFilePath)
		if err != nil {
			return false, fmt.Errorf("unable to read deprecated public key file for peer identity: %w", err)
		}

		pubKey, err := crypto.UnmarshalPublicKey(existingPubKeyBytes)
		if err != nil {
			return false, fmt.Errorf("unable to unmarshal deprecated public key for peer identity: %w", err)
		}

		peerID, err := peer.IDFromPublicKey(pubKey)
		if err != nil {
			return false, fmt.Errorf("unable to get peer identity from deprecated public key: %w", err)
		}

		// retrieve this node's private key from the new peer store
		privKey = newPeerStoreContainer.peerStore.PrivKey(peerID)
		if privKey == nil {
			return false, errors.New("error while fetching private key for peer identity from peer store")
		}
	}

	if err := WriteEd25519PrivateKeyToPEMFile(privKeyFilePath, privKey); err != nil {
		return false, err
	}

	// delete the deprecated public key file
	if err := os.Remove(deprecatedPubKeyFilePath); err != nil {
		return false, fmt.Errorf("unable to remove deprecated public key file for peer identity: %w", err)
	}

	return true, nil
}
