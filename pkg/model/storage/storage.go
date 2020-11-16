package storage

import (
	"crypto"
	"os"
	"path/filepath"
	"sync"

	pebbleDB "github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/bloom"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/pebble"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/profile"
)

const (
	cacheSize = 1 << 30 // 1 GB
)

var (
	// ErrNothingToCleanUp is returned when nothing is there to clean up in the database.
	ErrNothingToCleanUp = errors.New("Nothing to clean up in the databases")
)

func getPebbleDB(directory string, verbose bool) *pebbleDB.DB {

	cache := pebbleDB.NewCache(cacheSize)
	defer cache.Unref()

	opts := &pebbleDB.Options{
		Cache:                       cache,
		DisableWAL:                  false,
		L0CompactionThreshold:       2,
		L0StopWritesThreshold:       1000,
		LBaseMaxBytes:               64 << 20, // 64 MB
		Levels:                      make([]pebbleDB.LevelOptions, 7),
		MaxConcurrentCompactions:    3,
		MaxOpenFiles:                16384,
		MemTableSize:                64 << 20,
		MemTableStopWritesThreshold: 4,
	}
	opts.Experimental.L0SublevelCompactions = true

	for i := 0; i < len(opts.Levels); i++ {
		l := &opts.Levels[i]
		l.BlockSize = 32 << 10       // 32 KB
		l.IndexBlockSize = 256 << 10 // 256 KB
		l.FilterPolicy = bloom.FilterPolicy(10)
		l.FilterType = pebbleDB.TableFilter
		if i > 0 {
			l.TargetFileSize = opts.Levels[i-1].TargetFileSize * 2
		}
		l.EnsureDefaults()
	}
	opts.Levels[6].FilterPolicy = nil
	opts.Experimental.FlushSplitBytes = opts.Levels[0].TargetFileSize

	opts.EnsureDefaults()

	if verbose {
		opts.EventListener = pebbleDB.MakeLoggingEventListener(nil)
		opts.EventListener.TableDeleted = nil
		opts.EventListener.TableIngested = nil
		opts.EventListener.WALCreated = nil
		opts.EventListener.WALDeleted = nil
	}

	db, err := pebble.CreateDB(directory, opts)
	if err != nil {
		panic(err)
	}
	return db
}

type packageEvents struct {
	ReceivedValidMilestone *events.Event
}

type Storage struct {

	// database
	databaseDir    string
	pebbleInstance *pebbleDB.DB
	pebbleStore    kvstore.KVStore

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
	keyManager                         *keymanager.KeyManager
	milestonePublicKeyCount            int
	coordinatorMilestoneMerkleHashFunc crypto.Hash

	// utxo
	utxoManager *utxo.Manager

	// events
	Events *packageEvents
}

func New(databaseDirectory string, cachesProfile *profile.Caches) *Storage {

	pebbleInstance := getPebbleDB(databaseDirectory, false)
	pebbleStore := pebble.New(pebbleInstance)
	utxoManager := utxo.New(pebbleStore)

	s := &Storage{
		databaseDir:    databaseDirectory,
		pebbleInstance: pebbleInstance,
		pebbleStore:    pebbleStore,
		utxoManager:    utxoManager,
		Events: &packageEvents{
			ReceivedValidMilestone: events.NewEvent(MilestoneCaller),
		},
	}

	s.ConfigureStorages(pebbleStore, cachesProfile)
	s.loadSolidMilestoneFromDatabase()

	return s
}

func (s *Storage) UTXO() *utxo.Manager {
	return s.utxoManager
}

func (s *Storage) ConfigureStorages(store kvstore.KVStore, caches *profile.Caches) {

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
	s.FlushMessagesStorage()
	s.FlushChildrenStorage()
	s.FlushUnreferencedMessagesStorage()
}

func (s *Storage) ShutdownStorages() {

	s.ShutdownMilestoneStorage()
	s.ShutdownMessagesStorage()
	s.ShutdownMessagesStorage()
	s.ShutdownChildrenStorage()
	s.ShutdownUnreferencedMessagesStorage()
}

func (s *Storage) LoadInitialValuesFromDatabase() {
	s.loadSnapshotInfo()
	s.loadSolidEntryPoints()
}

func (s *Storage) loadSolidMilestoneFromDatabase() {

	ledgerMilestoneIndex, err := s.UTXO().ReadLedgerIndex()
	if err != nil {
		panic(err)
	}

	// set the solid milestone index based on the ledger milestone
	s.SetSolidMilestoneIndex(ledgerMilestoneIndex, false)
}

func (s *Storage) CloseDatabases() error {

	if err := s.pebbleInstance.Flush(); err != nil {
		return err
	}

	if err := s.pebbleInstance.Close(); err != nil {
		return err
	}

	return nil
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
