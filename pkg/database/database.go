package database

import (
	"github.com/iotaledger/hive.go/kvstore"
)

type Engine string

const (
	EngineUnknown = "unknown"
	EngineRocksDB = "rocksdb"
	EnginePebble  = "pebble"
)

// New creates a new Database instance.
func New(kvStore kvstore.KVStore, compactionSupported bool, compactionRunningFunc func() bool) *Database {
	return &Database{
		store:                 kvStore,
		compactionSupported:   compactionSupported,
		compactionRunningFunc: compactionRunningFunc,
	}
}

// Database holds the underlying KVStore and database specific functions.
type Database struct {
	store                 kvstore.KVStore
	compactionSupported   bool
	compactionRunningFunc func() bool
}

// KVStore returns the underlying KVStore.
func (db *Database) KVStore() kvstore.KVStore {
	return db.store
}

// CompactionSupported returns whether the database engine supports compaction.
func (db *Database) CompactionSupported() bool {
	return db.compactionSupported
}

// CompactionRunning returns whether a compaction is running.
func (db *Database) CompactionRunning() bool {
	return db.compactionRunningFunc()
}
