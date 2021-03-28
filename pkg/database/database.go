package database

import (
	"github.com/iotaledger/hive.go/kvstore"
)

// New creates a new Database instance.
func New(kvStore kvstore.KVStore, compactionRunningFunc func() bool) *Database {
	return &Database{
		store:                 kvStore,
		compactionRunningFunc: compactionRunningFunc,
	}
}

// Database holds the underlying KVStore and database specific functions.
type Database struct {
	store                 kvstore.KVStore
	compactionRunningFunc func() bool
}

// KVStore returns the underlying KVStore.
func (db *Database) KVStore() kvstore.KVStore {
	return db.store
}

// CompactionRunning returns whether a compaction is running.
func (db *Database) CompactionRunning() bool {
	return db.compactionRunningFunc()
}
