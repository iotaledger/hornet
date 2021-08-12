package storage

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"
)

var (
	// ErrNothingToCleanUp is returned when nothing is there to clean up in the database.
	ErrNothingToCleanUp = errors.New("Nothing to clean up in the databases")
)

type packageEvents struct {
	ReceivedValidMilestone *events.Event
	PruningStateChanged    *events.Event
}

type ReadOption = objectstorage.ReadOption
type IteratorOption = objectstorage.IteratorOption

type Storage struct {

	// database
	databaseDir   string
	store         kvstore.KVStore
	belowMaxDepth milestone.Index

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
	confirmedMilestoneIndex milestone.Index
	confirmedMilestoneLock  syncutils.RWMutex
	latestMilestoneIndex    milestone.Index
	latestMilestoneLock     syncutils.RWMutex

	// node synced
	isNodeSynced                    bool
	isNodeAlmostSynced              bool
	isNodeSyncedWithinBelowMaxDepth bool
	waitForNodeSyncedChannelsLock   syncutils.Mutex
	waitForNodeSyncedChannels       []chan struct{}

	// milestones
	keyManager              *keymanager.KeyManager
	milestonePublicKeyCount int

	// utxo
	utxoManager *utxo.Manager

	// events
	Events *packageEvents
}

func New(databaseDirectory string, store kvstore.KVStore, cachesProfile *profile.Caches, belowMaxDepth int, keyManager *keymanager.KeyManager, milestonePublicKeyCount int) (*Storage, error) {

	utxoManager := utxo.New(store)

	s := &Storage{
		databaseDir:             databaseDirectory,
		store:                   store,
		keyManager:              keyManager,
		milestonePublicKeyCount: milestonePublicKeyCount,
		utxoManager:             utxoManager,
		belowMaxDepth:           milestone.Index(belowMaxDepth),
		Events: &packageEvents{
			ReceivedValidMilestone: events.NewEvent(MilestoneWithRequestedCaller),
			PruningStateChanged:    events.NewEvent(events.BoolCaller),
		},
	}

	if err := s.configureStorages(s.store, cachesProfile); err != nil {
		return nil, err
	}

	if err := s.loadConfirmedMilestoneFromDatabase(); err != nil {
		return nil, err
	}

	s.loadSnapshotInfo()
	s.loadSolidEntryPoints()

	return s, nil
}

func (s *Storage) KVStore() kvstore.KVStore {
	return s.store
}

func (s *Storage) UTXO() *utxo.Manager {
	return s.utxoManager
}

func (s *Storage) configureStorages(store kvstore.KVStore, caches *profile.Caches) error {

	if err := s.configureHealthStore(store); err != nil {
		return err
	}

	if err := s.configureMessageStorage(store, caches.Messages); err != nil {
		return err
	}

	if err := s.configureChildrenStorage(store, caches.Children); err != nil {
		return err
	}

	if err := s.configureMilestoneStorage(store, caches.Milestones); err != nil {
		return err
	}

	if err := s.configureUnreferencedMessageStorage(store, caches.UnreferencedMessages); err != nil {
		return err
	}

	if err := s.configureIndexationStorage(store, caches.Indexations); err != nil {
		return err
	}

	s.configureSnapshotStore(store)

	return nil
}

// FlushStorages flushes all storages.
func (s *Storage) FlushStorages() {
	s.FlushMilestoneStorage()
	s.FlushMessagesStorage()
	s.FlushChildrenStorage()
	s.FlushIndexationStorage()
	s.FlushUnreferencedMessagesStorage()
}

// ShutdownStorages shuts down all storages.
func (s *Storage) ShutdownStorages() {

	s.ShutdownMilestoneStorage()
	s.ShutdownMessagesStorage()
	s.ShutdownChildrenStorage()
	s.ShutdownIndexationStorage()
	s.ShutdownUnreferencedMessagesStorage()
}

func (s *Storage) loadConfirmedMilestoneFromDatabase() error {

	ledgerMilestoneIndex, err := s.UTXO().ReadLedgerIndex()
	if err != nil {
		return err
	}

	// set the confirmed milestone index based on the ledger milestone
	return s.SetConfirmedMilestoneIndex(ledgerMilestoneIndex, false)
}

func (s *Storage) DatabaseSupportsCleanup() bool {
	// ToDo: add this to the KVStore interface
	return false
}

func (s *Storage) CleanupDatabases() error {
	// ToDo: add this to the KVStore interface
	return ErrNothingToCleanUp
}

// DatabaseSize returns the size of the database.
func (s *Storage) DatabaseSize() (int64, error) {

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
