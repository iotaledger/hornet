package database

import (
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/gohornet/hornet/packages/profile"
	"github.com/pkg/errors"
)

var (
	instance  *badger.DB
	once      sync.Once
	directory = "mainnetdb"
)

// Returns whether the given file or directory exists.
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Settings sets DB dir and light mode
func Settings(dir string) {
	directory = dir
}

func checkDir(dir string) error {
	exists, err := exists(dir)
	if err != nil {
		return err
	}

	if !exists {
		return os.Mkdir(dir, 0700)
	}
	return nil
}

func createDB() (*badger.DB, error) {
	if err := checkDir(directory); err != nil {
		return nil, errors.Wrap(err, "Could not check directory")
	}

	opts := badger.DefaultOptions(directory)
	opts.Logger = &logger{}

	badgerOpts := profile.GetProfile().Badger
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

	db, err := badger.Open(opts)
	if err != nil {
		return nil, errors.Wrap(err, "Could not open new DB")
	}

	return db, nil
}

func GetBadgerInstance() *badger.DB {
	once.Do(func() {
		db, err := createDB()
		if err != nil {
			// errors should cause a panic to avoid singleton deadlocks
			panic(err)
		}
		instance = db
	})
	return instance
}

func CleanupBadgerInstance() {

	db := GetBadgerInstance()

	fmt.Println("Run badger garbage collection")

	var err error
	for err == nil {
		err = db.RunValueLogGC(0.7)
	}
}
