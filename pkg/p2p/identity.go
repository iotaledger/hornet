package p2p

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds"

	kvstoreds "github.com/iotaledger/go-ds-kvstore"
	hivedb "github.com/iotaledger/hive.go/core/database"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hornet/v2/pkg/database"
)

const (
	PrivKeyFileName = "identity.key"
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
	if err := psc.peerStore.Close(); err != nil {
		return err
	}

	if err := psc.store.Flush(); err != nil {
		return err
	}

	return psc.store.Close()
}

// NewPeerStoreContainer creates a peerstore using kvstore.
func NewPeerStoreContainer(peerStorePath string, dbEngine hivedb.Engine, createDatabaseIfNotExists bool) (*PeerStoreContainer, error) {

	dirPath := filepath.Dir(peerStorePath)

	if createDatabaseIfNotExists {
		if err := os.MkdirAll(dirPath, 0700); err != nil {
			return nil, fmt.Errorf("could not create peer store database dir '%s': %w", dirPath, err)
		}
	}

	store, err := database.StoreWithDefaultSettings(peerStorePath, createDatabaseIfNotExists, dbEngine, database.AllowedEnginesDefault...)
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
