package database

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/dgraph-io/badger/v2"

	"github.com/iotaledger/hive.go/database"

	"github.com/gohornet/hornet/packages/profile"
)

var (
	instance   *badger.DB
	once       sync.Once
	directory  = "mainnetdb"
	badgerOpts *profile.BadgerOpts
)

// Settings sets DB dir and the badger options
func Settings(dir string, options *profile.BadgerOpts) {
	directory = dir
	badgerOpts = options
}

func GetHornetBadgerInstance() *badger.DB {
	once.Do(func() {

		opts := badger.DefaultOptions(directory)
		opts.Logger = nil

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
			WithZSTDCompressionLevel(badgerOpts.ZSTDCompressionLevel).
			WithValueLogFileSize(badgerOpts.ValueLogFileSize).
			WithValueLogMaxEntries(badgerOpts.ValueLogMaxEntries).
			WithValueThreshold(badgerOpts.ValueThreshold).
			WithTruncate(badgerOpts.WithTruncate).
			WithLogRotatesToFlush(badgerOpts.LogRotatesToFlush).
			WithEventLogging(badgerOpts.EventLogging)

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

func CleanupHornetBadgerInstance() {

	db := GetHornetBadgerInstance()

	fmt.Println("Run badger garbage collection")

	var err error
	for err == nil {
		err = db.RunValueLogGC(0.7)
	}
}
