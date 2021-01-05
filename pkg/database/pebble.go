package database

import (
	pebbleDB "github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/bloom"

	"github.com/iotaledger/hive.go/kvstore/pebble"
)

// NewPebbleDB creates a new pebble DB instance.
func NewPebbleDB(directory string, verbose bool) *pebbleDB.DB {
	cache := pebbleDB.NewCache(1 << 30) // 1 GB
	defer cache.Unref()

	opts := &pebbleDB.Options{
		Cache:                       cache,
		DisableWAL:                  true,
		L0CompactionThreshold:       2,
		L0StopWritesThreshold:       1000,
		LBaseMaxBytes:               64 << 20, // 64 MB
		Levels:                      make([]pebbleDB.LevelOptions, 7),
		MaxConcurrentCompactions:    3,
		MaxOpenFiles:                16384,
		MemTableSize:                64 << 20,
		MemTableStopWritesThreshold: 4,
	}

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
