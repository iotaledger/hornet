package database

import (
	"runtime"
	"sync"

	"github.com/dgraph-io/badger/v2"

	"github.com/iotaledger/hive.go/database"

	"github.com/gohornet/hornet/pkg/profile"
)

var (
	instance   *badger.DB
	once       sync.Once
	directory  = "mainnetdb"
	badgerOpts *profile.BadgerOpts

	ErrKeyNotFound = database.ErrKeyNotFound
)

type (
	Database     = database.Database
	KeyPrefix    = database.KeyPrefix
	Key          = database.Key
	Value        = database.Value
	KeyOnlyEntry = database.KeyOnlyEntry
	Entry        = database.Entry
)

func Get(dbPrefix byte, optionalBadger ...*badger.DB) (Database, error) {
	return database.Get(dbPrefix, optionalBadger...)
}

// Settings sets DB dir and the badger options
func Settings(dir string, options *profile.BadgerOpts) {
	directory = dir
	badgerOpts = options
}

// GetDatabaseSize returns the size of the database keys and values.
func GetDatabaseSize() (keys int64, values int64) {
	return GetHornetBadgerInstance().Size()
}

// GetHornetBadgerInstance returns the badger DB instance.
func GetHornetBadgerInstance() *badger.DB {
	once.Do(func() {

		opts := badger.DefaultOptions(directory)

		opts = opts.WithLevelOneSize(badgerOpts.LevelOneSize).
			WithLevelSizeMultiplier(badgerOpts.LevelSizeMultiplier).
			WithTableLoadingMode(badgerOpts.TableLoadingMode).
			WithValueLogLoadingMode(badgerOpts.ValueLogLoadingMode).
			WithMaxLevels(badgerOpts.MaxLevels).
			WithMaxTableSize(badgerOpts.MaxTableSize).
			WithNumCompactors(badgerOpts.NumCompactors).
			WithNumLevelZeroTables(badgerOpts.NumLevelZeroTables).
			WithNumLevelZeroTablesStall(badgerOpts.NumLevelZeroTablesStall).
			WithNumMemtables(badgerOpts.NumMemtables).
			WithBloomFalsePositive(badgerOpts.BloomFalsePositive).
			WithBlockSize(badgerOpts.BlockSize).
			WithSyncWrites(badgerOpts.SyncWrites).
			WithNumVersionsToKeep(badgerOpts.NumVersionsToKeep).
			WithCompactL0OnClose(badgerOpts.CompactLevel0OnClose).
			WithKeepL0InMemory(badgerOpts.KeepL0InMemory).
			WithVerifyValueChecksum(badgerOpts.VerifyValueChecksum).
			WithMaxCacheSize(badgerOpts.MaxCacheSize).
			WithMaxBfCacheSize(badgerOpts.MaxBfCacheSize).
			WithLoadBloomsOnOpen(badgerOpts.LoadBloomsOnOpen).
			WithZSTDCompressionLevel(badgerOpts.ZSTDCompressionLevel).
			WithCompression(badgerOpts.CompressionType).
			WithValueLogFileSize(badgerOpts.ValueLogFileSize).
			WithValueLogMaxEntries(badgerOpts.ValueLogMaxEntries).
			WithValueThreshold(badgerOpts.ValueThreshold).
			WithTruncate(badgerOpts.WithTruncate).
			WithLogRotatesToFlush(badgerOpts.LogRotatesToFlush).
			WithEventLogging(badgerOpts.EventLogging).
			WithLogger(badgerOpts.Logger)

		if runtime.GOOS == "windows" {
			opts = opts.WithTruncate(true)
		}

		db, err := database.CreateDB(directory, opts)
		if err != nil {
			// errors should cause a panic to avoid singleton deadlocks
			panic(err)
		}
		instance = db
	})
	return instance
}

// CleanupHornetBadgerInstance runs the badger garbage collector.
func CleanupHornetBadgerInstance(discardRatio ...float64) error {
	valueLogDiscardRatio := badgerOpts.ValueLogGCDiscardRatio
	if len(discardRatio) > 0 {
		valueLogDiscardRatio = discardRatio[0]
	}
	return GetHornetBadgerInstance().RunValueLogGC(valueLogDiscardRatio)
}
