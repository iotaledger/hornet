package database

import (
	"os"
	"time"

	"github.com/pkg/errors"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/pebble"
	"github.com/iotaledger/hive.go/kvstore/rocksdb"
	"github.com/iotaledger/hive.go/syncutils"
)

const (
	// whether to delete the database at startup
	CfgTangleDeleteDatabase = "deleteDatabase"
	// whether to delete the database and snapshots at startup
	CfgTangleDeleteAll = "deleteAll"
)

func init() {
	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
			Name:      "Database",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	CorePlugin *node.CorePlugin
	deps       dependencies

	deleteDatabase = flag.Bool(CfgTangleDeleteDatabase, false, "whether to delete the database at startup")
	deleteAll      = flag.Bool(CfgTangleDeleteAll, false, "whether to delete the database and snapshots at startup")

	garbageCollectionLock syncutils.Mutex

	// Closures
	onPruningStateChanged *events.Closure
)

type dependencies struct {
	dig.In
	Database       *database.Database
	Storage        *storage.Storage
	Events         *Events
	StorageMetrics *metrics.StorageMetrics
}

func provide(c *dig.Container) {

	type dbdeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	type dbresult struct {
		dig.Out

		StorageMetrics     *metrics.StorageMetrics
		DatabaseMetrics    *metrics.DatabaseMetrics
		DatabaseEvents     *Events
		DeleteDatabaseFlag bool            `name:"deleteDatabase"`
		DeleteAllFlag      bool            `name:"deleteAll"`
		DatabaseEngine     database.Engine `name:"databaseEngine"`
	}

	if err := c.Provide(func(deps dbdeps) dbresult {

		engine, err := database.DatabaseEngine(deps.NodeConfig.String(CfgDatabaseEngine))
		if err != nil {
			CorePlugin.Panic(err)
		}

		res := dbresult{
			StorageMetrics:  &metrics.StorageMetrics{},
			DatabaseMetrics: &metrics.DatabaseMetrics{},
			DatabaseEvents: &Events{
				DatabaseCleanup:    events.NewEvent(DatabaseCleanupCaller),
				DatabaseCompaction: events.NewEvent(events.BoolCaller),
			},
			DeleteDatabaseFlag: *deleteDatabase,
			DeleteAllFlag:      *deleteAll,
			DatabaseEngine:     engine,
		}
		return res
	}); err != nil {
		CorePlugin.Panic(err)
	}

	type databasedeps struct {
		dig.In
		DeleteDatabaseFlag bool                         `name:"deleteDatabase"`
		DeleteAllFlag      bool                         `name:"deleteAll"`
		NodeConfig         *configuration.Configuration `name:"nodeConfig"`
		DatabaseEngine     database.Engine              `name:"databaseEngine"`
		Events             *Events
		Metrics            *metrics.DatabaseMetrics
	}

	if err := c.Provide(func(deps databasedeps) *database.Database {

		if deps.DeleteDatabaseFlag || deps.DeleteAllFlag {
			// delete old database folder
			if err := os.RemoveAll(deps.NodeConfig.String(CfgDatabasePath)); err != nil {
				CorePlugin.Panicf("deleting database folder failed: %s", err)
			}
		}

		targetEngine, err := database.CheckDatabaseEngine(deps.NodeConfig.String(CfgDatabasePath), true, deps.DatabaseEngine)
		if err != nil {
			CorePlugin.Panic(err)
		}

		switch targetEngine {
		case database.EnginePebble:
			reportCompactionRunning := func(running bool) {
				deps.Metrics.CompactionRunning.Store(running)
				if running {
					deps.Metrics.CompactionCount.Inc()
				}
				deps.Events.DatabaseCompaction.Trigger(running)
			}

			db, err := database.NewPebbleDB(deps.NodeConfig.String(CfgDatabasePath), reportCompactionRunning, true)
			if err != nil {
				CorePlugin.Panicf("database initialization failed: %s", err)
			}

			return database.New(
				pebble.New(db),
				true,
				func() bool { return deps.Metrics.CompactionRunning.Load() },
			)

		case database.EngineRocksDB:
			db, err := database.NewRocksDB(deps.NodeConfig.String(CfgDatabasePath))
			if err != nil {
				CorePlugin.Panicf("database initialization failed: %s", err)
			}

			return database.New(
				rocksdb.New(db),
				true,
				func() bool {
					if numCompactions, success := db.GetIntProperty("rocksdb.num-running-compactions"); success {
						runningBefore := deps.Metrics.CompactionRunning.Load()
						running := numCompactions != 0

						deps.Metrics.CompactionRunning.Store(running)
						if running && !runningBefore {
							// we may miss some compactions, since this is only calculated if polled.
							deps.Metrics.CompactionCount.Inc()
							deps.Events.DatabaseCompaction.Trigger(running)
						}
						return running
					}
					return false
				},
			)
		default:
			CorePlugin.Panicf("unknown database engine: %s, supported engines: pebble/rocksdb", targetEngine)
			return nil
		}
	}); err != nil {
		CorePlugin.Panic(err)
	}

	type storagedeps struct {
		dig.In
		NodeConfig                 *configuration.Configuration `name:"nodeConfig"`
		Database                   *database.Database
		Profile                    *profile.Profile
		BelowMaxDepth              int `name:"belowMaxDepth"`
		CoordinatorPublicKeyRanges coordinator.PublicKeyRanges
	}

	if err := c.Provide(func(deps storagedeps) *storage.Storage {

		keyManager := keymanager.New()
		for _, keyRange := range deps.CoordinatorPublicKeyRanges {
			pubKey, err := utils.ParseEd25519PublicKeyFromString(keyRange.Key)
			if err != nil {
				CorePlugin.Panicf("can't load public key ranges: %s", err)
			}

			keyManager.AddKeyRange(pubKey, keyRange.StartIndex, keyRange.EndIndex)
		}

		store, err := storage.New(deps.NodeConfig.String(CfgDatabasePath), deps.Database.KVStore(), deps.Profile.Caches, deps.BelowMaxDepth, keyManager, deps.NodeConfig.Int(protocfg.CfgProtocolMilestonePublicKeyCount))
		if err != nil {
			CorePlugin.Panicf("can't initialize storage: %s", err)
		}
		return store
	}); err != nil {
		CorePlugin.Panic(err)
	}

	if err := c.Provide(func(storage *storage.Storage) *utxo.Manager {
		return storage.UTXO()
	}); err != nil {
		CorePlugin.Panic(err)
	}
}

func configure() {

	correctDatabaseVersion, err := deps.Storage.IsCorrectDatabaseVersion()
	if err != nil {
		CorePlugin.Panic(err)
	}

	if !correctDatabaseVersion {
		databaseVersionUpdated, err := deps.Storage.UpdateDatabaseVersion()
		if err != nil {
			CorePlugin.Panic(err)
		}

		if !databaseVersionUpdated {
			CorePlugin.Panic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new snapshot.")
		}
	}

	if err = CorePlugin.Daemon().BackgroundWorker("Close database", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal

		if err = deps.Storage.MarkDatabaseHealthy(); err != nil {
			CorePlugin.Panic(err)
		}

		CorePlugin.LogInfo("Syncing databases to disk...")
		if err = closeDatabases(); err != nil {
			CorePlugin.Panicf("Syncing databases to disk... failed: %s", err)
		}
		CorePlugin.LogInfo("Syncing databases to disk... done")
	}, shutdown.PriorityCloseDatabase); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}

	configureEvents()
}

func run() {
	if err := CorePlugin.Daemon().BackgroundWorker("Database[Events]", func(shutdownSignal <-chan struct{}) {
		attachEvents()
		<-shutdownSignal
		detachEvents()
	}, shutdown.PriorityMetricsUpdater); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}
}

func RunGarbageCollection() {
	if !deps.Storage.DatabaseSupportsCleanup() {
		return
	}

	garbageCollectionLock.Lock()
	defer garbageCollectionLock.Unlock()

	CorePlugin.LogInfo("running full database garbage collection. This can take a while...")

	start := time.Now()

	deps.Events.DatabaseCleanup.Trigger(&DatabaseCleanup{
		Start: start,
	})

	err := deps.Storage.CleanupDatabases()

	end := time.Now()

	deps.Events.DatabaseCleanup.Trigger(&DatabaseCleanup{
		Start: start,
		End:   end,
	})

	if err != nil {
		if !errors.Is(err, storage.ErrNothingToCleanUp) {
			CorePlugin.LogWarnf("full database garbage collection failed with error: %s. took: %v", err, end.Sub(start).Truncate(time.Millisecond))
			return
		}
	}

	CorePlugin.LogInfof("full database garbage collection finished. took %v", end.Sub(start).Truncate(time.Millisecond))
}

func closeDatabases() error {

	if err := deps.Database.KVStore().Flush(); err != nil {
		return err
	}

	if err := deps.Database.KVStore().Close(); err != nil {
		return err
	}

	return nil
}

func configureEvents() {
	onPruningStateChanged = events.NewClosure(func(running bool) {
		deps.StorageMetrics.PruningRunning.Store(running)
		if running {
			deps.StorageMetrics.Prunings.Inc()
		}
	})
}

func attachEvents() {
	deps.Storage.Events.PruningStateChanged.Attach(onPruningStateChanged)
}

func detachEvents() {
	deps.Storage.Events.PruningStateChanged.Detach(onPruningStateChanged)
}
