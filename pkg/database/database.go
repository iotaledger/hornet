package database

import (
	"encoding/json"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
)

type Engine string

const (
	EngineUnknown = "unknown"
	EngineRocksDB = "rocksdb"
	EnginePebble  = "pebble"
)

var (
	// ErrNothingToCleanUp is returned when nothing is there to clean up in the database.
	ErrNothingToCleanUp = errors.New("Nothing to clean up in the databases")
)

type DatabaseCleanup struct {
	Start time.Time
	End   time.Time
}

func (c *DatabaseCleanup) MarshalJSON() ([]byte, error) {

	cleanup := struct {
		Start int64 `json:"start"`
		End   int64 `json:"end"`
	}{
		Start: 0,
		End:   0,
	}

	if !c.Start.IsZero() {
		cleanup.Start = c.Start.Unix()
	}

	if !c.End.IsZero() {
		cleanup.End = c.End.Unix()
	}

	return json.Marshal(cleanup)
}

func DatabaseCleanupCaller(handler interface{}, params ...interface{}) {
	handler.(func(*DatabaseCleanup))(params[0].(*DatabaseCleanup))
}

type Events struct {
	DatabaseCleanup    *events.Event
	DatabaseCompaction *events.Event
}

// Database holds the underlying KVStore and database specific functions.
type Database struct {
	databaseDir           string
	store                 kvstore.KVStore
	events                *Events
	compactionSupported   bool
	compactionRunningFunc func() bool
}

// New creates a new Database instance.
func New(databaseDirectory string, kvStore kvstore.KVStore, events *Events, compactionSupported bool, compactionRunningFunc func() bool) *Database {
	return &Database{
		databaseDir:           databaseDirectory,
		store:                 kvStore,
		events:                events,
		compactionSupported:   compactionSupported,
		compactionRunningFunc: compactionRunningFunc,
	}
}

// KVStore returns the underlying KVStore.
func (db *Database) KVStore() kvstore.KVStore {
	return db.store
}

// Events returns the events of the database.
func (db *Database) Events() *Events {
	return db.events
}

// CompactionSupported returns whether the database engine supports compaction.
func (db *Database) CompactionSupported() bool {
	return db.compactionSupported
}

// CompactionRunning returns whether a compaction is running.
func (db *Database) CompactionRunning() bool {
	return db.compactionRunningFunc()
}

// Size returns the size of the database.
func (db *Database) Size() (int64, error) {
	return utils.FolderSize(db.databaseDir)
}
