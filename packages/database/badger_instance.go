package database

import (
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/pkg/errors"
)

var (
	instance  *badger.DB
	once      sync.Once
	directory string = "mainnetdb"
	light     bool
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
func Settings(dir string, lightMode bool) {
	directory = dir
	light = lightMode
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

	if light {
		opts = opts.WithLevelOneSize(256 << 18).
			WithLevelSizeMultiplier(10).
			WithTableLoadingMode(options.FileIO).
			WithValueLogLoadingMode(options.FileIO).
			WithMaxLevels(5).
			WithMaxTableSize(64 << 18).
			WithNumCompactors(1). // Compactions can be expensive. Only run 2.
			WithNumLevelZeroTables(1).
			WithNumLevelZeroTablesStall(2).
			WithNumMemtables(1).
			WithSyncWrites(false).
			WithNumVersionsToKeep(1).
			WithCompactL0OnClose(true).
			WithValueLogFileSize(1<<25 - 1).
			WithValueLogMaxEntries(250000).
			WithValueThreshold(32).
			WithLogRotatesToFlush(2).
			WithMaxCacheSize(50000000)
	} else {
		opts = opts.WithLevelOneSize(256 << 20).
			WithLevelSizeMultiplier(10).
			WithTableLoadingMode(options.MemoryMap).
			WithValueLogLoadingMode(options.MemoryMap).
			WithMaxLevels(7).
			WithMaxTableSize(64 << 20).
			WithNumCompactors(2). // Compactions can be expensive. Only run 2.
			WithNumLevelZeroTables(5).
			WithNumLevelZeroTablesStall(10).
			WithNumMemtables(5).
			WithSyncWrites(true).
			WithNumVersionsToKeep(1).
			WithCompactL0OnClose(true).
			WithValueLogFileSize(1<<30 - 1).
			WithValueLogMaxEntries(1000000).
			WithValueThreshold(32).
			WithLogRotatesToFlush(2).
			WithMaxCacheSize(50000000)

		// must be used under Windows otherwise restarts don't work
		if runtime.GOOS == "windows" {
			opts = opts.WithTruncate(true)
		}
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
