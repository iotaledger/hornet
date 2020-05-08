package database

import (
	"github.com/iotaledger/hive.go/database/badgerdb"
	"runtime"
	"sync"

	"github.com/dgraph-io/badger/v2"
)

var (
	badgerInstance *badger.DB
	badgerOnce     sync.Once
)

// getBadgerInstance returns the badger DB instance.
func getBadgerInstance() *badger.DB {
	badgerOnce.Do(func() {

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

		db, err := badgerdb.CreateDB(directory, opts)
		if err != nil {
			// errors should cause a panic to avoid singleton deadlocks
			panic(err)
		}
		badgerInstance = db
	})
	return badgerInstance
}

// cleanupBadgerInstance runs the badger garbage collector.
func cleanupBadgerInstance(discardRatio ...float64) error {

	valueLogDiscardRatio := badgerOpts.ValueLogGCDiscardRatio
	if len(discardRatio) > 0 {
		valueLogDiscardRatio = discardRatio[0]
	}
	return getBadgerInstance().RunValueLogGC(valueLogDiscardRatio)
}
