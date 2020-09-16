package tangle

import (
	"errors"
	"os"
	"path/filepath"

	pebbleDB "github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/bloom"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/pebble"

	"github.com/gohornet/hornet/pkg/profile"
)

const (
	cacheSize = 1 << 30 // 1 GB
)

var (
	dbDir string

	pebbleInstance *pebbleDB.DB

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
		MinCompactionRate:           4 << 20, // 4 MB/s
		MinFlushRate:                4 << 20, // 4 MB/s
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

func ConfigureDatabases(directory string) {

	dbDir = directory

	pebbleInstance = getPebbleDB(directory, false)

	ConfigureStorages(pebble.New(pebbleInstance), profile.LoadProfile().Caches)
}

func ConfigureStorages(store kvstore.KVStore, caches profile.Caches) {

	configureHealthStore(store)
	configureMessageStorage(store, caches.Transactions)
	configureApproversStorage(store, caches.Approvers)
	configureMilestoneStorage(store, caches.Milestones)
	configureUnconfirmedMessageStorage(store, caches.UnconfirmedTx)
	configureSnapshotStore(store)
}

func FlushStorages() {
	FlushMilestoneStorage()
	FlushMessagesStorage()
	FlushMessagesStorage()
	FlushChildrenStorage()
	FlushUnconfirmedMessagesStorage()
}

func ShutdownStorages() {

	ShutdownMilestoneStorage()
	ShutdownMessagesStorage()
	ShutdownMessagesStorage()
	ShutdownChildrenStorage()
	ShutdownUnconfirmedMessagesStorage()
}

func LoadInitialValuesFromDatabase() {
	loadSnapshotInfo()
	loadSolidEntryPoints()
}

func CloseDatabases() error {

	if err := pebbleInstance.Flush(); err != nil {
		return err
	}

	if err := pebbleInstance.Close(); err != nil {
		return err
	}

	return nil
}

func DatabaseSupportsCleanup() bool {
	// Bolt does not support cleaning up anything
	return false
}

func CleanupDatabases() error {
	// Bolt does not support cleaning up anything
	return ErrNothingToCleanUp
}

// GetDatabaseSize returns the size of the database.
func GetDatabaseSize() (int64, error) {

	var size int64

	err := filepath.Walk(dbDir, func(_ string, info os.FileInfo, err error) error {
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
