package database

import (
	pebbleDB "github.com/cockroachdb/pebble"

	"github.com/iotaledger/hive.go/kvstore/pebble"
)

// NewPebbleDB creates a new pebble DB instance.
func NewPebbleDB(directory string, verbose bool) *pebbleDB.DB {
	opts := &pebbleDB.Options{
		DisableWAL: true,
	}
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
