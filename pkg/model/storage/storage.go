package storage

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/profile"
)

var (
	// ErrNothingToCleanUp is returned when nothing is there to clean up in the database.
	ErrNothingToCleanUp = errors.New("Nothing to clean up in the databases")
)

type packageEvents struct {
	ReceivedValidMilestone *events.Event
}

type Storage struct {

	// database
	databaseDir string
	store       kvstore.KVStore

	// kv storages
	healthStore   kvstore.KVStore
	snapshotStore kvstore.KVStore

	// object storages
	childrenStorage             *objectstorage.ObjectStorage
	indexationStorage           *objectstorage.ObjectStorage
	messagesStorage             *objectstorage.ObjectStorage
	metadataStorage             *objectstorage.ObjectStorage
	milestoneStorage            *objectstorage.ObjectStorage
	unreferencedMessagesStorage *objectstorage.ObjectStorage

	// solid entry points
	solidEntryPoints     *SolidEntryPoints
	solidEntryPointsLock sync.RWMutex

	// snapshot info
	snapshot      *SnapshotInfo
	snapshotMutex syncutils.RWMutex

	// milestones
	solidMilestoneIndex  milestone.Index
	solidMilestoneLock   syncutils.RWMutex
	latestMilestoneIndex milestone.Index
	latestMilestoneLock  syncutils.RWMutex

	// node synced
	isNodeSynced                  bool
	isNodeSyncedThreshold         bool
	waitForNodeSyncedChannelsLock syncutils.Mutex
	waitForNodeSyncedChannels     []chan struct{}

	// milestones
	keyManager              *keymanager.KeyManager
	milestonePublicKeyCount int

	// utxo
	utxoManager *utxo.Manager

	// events
	Events *packageEvents
}

func New(databaseDirectory string, store kvstore.KVStore, cachesProfile *profile.Caches) *Storage {

	utxoManager := utxo.New(store)

	s := &Storage{
		databaseDir: databaseDirectory,
		store:       store,
		utxoManager: utxoManager,
		Events: &packageEvents{
			ReceivedValidMilestone: events.NewEvent(MilestoneCaller),
		},
	}

	s.configureStorages(s.store, cachesProfile)
	s.loadSolidMilestoneFromDatabase()
	s.loadSnapshotInfo()
	s.loadSolidEntryPoints()

	return s
}

func (s *Storage) KVStore() kvstore.KVStore {
	return s.store
}

func (s *Storage) UTXO() *utxo.Manager {
	return s.utxoManager
}

func (s *Storage) configureStorages(store kvstore.KVStore, caches *profile.Caches) {

	s.configureHealthStore(store)
	s.configureMessageStorage(store, caches.Messages)
	s.configureChildrenStorage(store, caches.Children)
	s.configureMilestoneStorage(store, caches.Milestones)
	s.configureUnreferencedMessageStorage(store, caches.UnreferencedMessages)
	s.configureIndexationStorage(store, caches.Indexations)
	s.configureSnapshotStore(store)

	s.UTXO()
}

func (s *Storage) FlushStorages() {
	s.FlushMilestoneStorage()
	s.FlushMessagesStorage()
	s.FlushChildrenStorage()
	s.FlushIndexationStorage()
	s.FlushUnreferencedMessagesStorage()
}

func (s *Storage) ShutdownStorages() {

	s.ShutdownMilestoneStorage()
	s.ShutdownMessagesStorage()
	s.ShutdownChildrenStorage()
	s.ShutdownIndexationStorage()
	s.ShutdownUnreferencedMessagesStorage()
}

func (s *Storage) loadSolidMilestoneFromDatabase() {

	ledgerMilestoneIndex, err := s.UTXO().ReadLedgerIndex()
	if err != nil {
		panic(err)
	}

	// set the solid milestone index based on the ledger milestone
	s.SetSolidMilestoneIndex(ledgerMilestoneIndex, false)
}

func (s *Storage) DatabaseSupportsCleanup() bool {
	// Bolt does not support cleaning up anything
	return false
}

func (s *Storage) CleanupDatabases() error {
	// Bolt does not support cleaning up anything
	return ErrNothingToCleanUp
}

// GetDatabaseSize returns the size of the database.
func (s *Storage) GetDatabaseSize() (int64, error) {

	var size int64

	err := filepath.Walk(s.databaseDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			size += info.Size()
		}

		return err
	})

	return size, err
}
