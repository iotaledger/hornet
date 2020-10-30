package tangle

import (
	"crypto"
	"errors"
	"os"
	"path/filepath"
	"sync"

	pebbleDB "github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/bloom"

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

type Tangle struct {
	dbDir string

	pebbleInstance *pebbleDB.DB
	pebbleStore    kvstore.KVStore

	healthStore kvstore.KVStore

	childrenStorage             *objectstorage.ObjectStorage
	indexationStorage           *objectstorage.ObjectStorage
	messagesStorage             *objectstorage.ObjectStorage
	metadataStorage             *objectstorage.ObjectStorage
	milestoneStorage            *objectstorage.ObjectStorage
	unreferencedMessagesStorage *objectstorage.ObjectStorage

	snapshotStore kvstore.KVStore

	solidEntryPoints     *SolidEntryPoints
	solidEntryPointsLock sync.RWMutex

	snapshot      *SnapshotInfo
	snapshotMutex syncutils.RWMutex

	solidMilestoneIndex   milestone.Index
	solidMilestoneLock    syncutils.RWMutex
	latestMilestoneIndex  milestone.Index
	latestMilestoneLock   syncutils.RWMutex
	isNodeSynced          bool
	isNodeSyncedThreshold bool

	waitForNodeSyncedChannelsLock syncutils.Mutex
	waitForNodeSyncedChannels     []chan struct{}

	keyManager                         *keymanager.KeyManager
	milestonePublicKeyCount            int
	coordinatorMilestoneMerkleHashFunc crypto.Hash

	Events *packageEvents

	utxoOnce    sync.Once
	utxoManager *utxo.Manager
}

func New(directory string, cachesProfile *profile.Caches) *Tangle {

	pebbleInstance := getPebbleDB(directory, false)
	pebbleStore := pebble.New(pebbleInstance)

	t := &Tangle{
		dbDir:          directory,
		pebbleInstance: pebbleInstance,
		pebbleStore:    pebbleStore,
		Events: &packageEvents{
			ReceivedValidMilestone: events.NewEvent(MilestoneCaller),
			AddressSpent:           events.NewEvent(events.StringCaller),
		},
	}

	t.ConfigureStorages(pebbleStore, cachesProfile)
	t.loadSolidMilestoneFromDatabase()

	return t
}

func (t *Tangle) UTXO() *utxo.Manager {
	t.utxoOnce.Do(func() {
		t.utxoManager = utxo.New(t.pebbleStore)
	})
	return t.utxoManager
}

func (t *Tangle) ConfigureStorages(store kvstore.KVStore, caches *profile.Caches) {

	t.configureHealthStore(store)
	t.configureMessageStorage(store, caches.Messages)
	t.configureChildrenStorage(store, caches.Children)
	t.configureMilestoneStorage(store, caches.Milestones)
	t.configureUnreferencedMessageStorage(store, caches.UnreferencedMessages)
	t.configureIndexationStorage(store, caches.Indexations)
	t.configureSnapshotStore(store)

	t.UTXO()
}

func (t *Tangle) FlushStorages() {
	t.FlushMilestoneStorage()
	t.FlushMessagesStorage()
	t.FlushMessagesStorage()
	t.FlushChildrenStorage()
	t.FlushUnreferencedMessagesStorage()
}

func (t *Tangle) ShutdownStorages() {

	t.ShutdownMilestoneStorage()
	t.ShutdownMessagesStorage()
	t.ShutdownMessagesStorage()
	t.ShutdownChildrenStorage()
	t.ShutdownUnreferencedMessagesStorage()
}

func (t *Tangle) LoadInitialValuesFromDatabase() {
	t.loadSnapshotInfo()
	t.loadSolidEntryPoints()
}

func (t *Tangle) loadSolidMilestoneFromDatabase() {

	ledgerMilestoneIndex, err := t.UTXO().ReadLedgerIndex()
	if err != nil {
		panic(err)
	}

	// set the solid milestone index based on the ledger milestone
	t.SetSolidMilestoneIndex(ledgerMilestoneIndex, false)
}

func (t *Tangle) CloseDatabases() error {

	if err := t.pebbleInstance.Flush(); err != nil {
		return err
	}

	if err := t.pebbleInstance.Close(); err != nil {
		return err
	}

	return nil
}

func (t *Tangle) DatabaseSupportsCleanup() bool {
	// Bolt does not support cleaning up anything
	return false
}

func (t *Tangle) CleanupDatabases() error {
	// Bolt does not support cleaning up anything
	return ErrNothingToCleanUp
}

// GetDatabaseSize returns the size of the database.
func (t *Tangle) GetDatabaseSize() (int64, error) {

	var size int64

	err := filepath.Walk(t.dbDir, func(_ string, info os.FileInfo, err error) error {
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
