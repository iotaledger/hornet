package database

import (
	"context"
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/pebble"
	"github.com/iotaledger/hive.go/kvstore/rocksdb"
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
			Name:           "Database",
			DepsFunc:       func(cDeps dependencies) { deps = cDeps },
			Params:         params,
			InitConfigPars: initConfigPars,
			Provide:        provide,
			Configure:      configure,
			Run:            run,
		},
	}
}

var (
	CorePlugin *node.CorePlugin
	deps       dependencies

	deleteDatabase = flag.Bool(CfgTangleDeleteDatabase, false, "whether to delete the database at startup")
	deleteAll      = flag.Bool(CfgTangleDeleteAll, false, "whether to delete the database and snapshots at startup")

	// Closures
	onPruningStateChanged *events.Closure
)

type dependencies struct {
	dig.In
	TangleDatabase *database.Database `name:"tangleDatabase"`
	UTXODatabase   *database.Database `name:"utxoDatabase"`
	Storage        *storage.Storage
	StorageMetrics *metrics.StorageMetrics
}

func initConfigPars(c *dig.Container) {

	type cfgDeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	type cfgResult struct {
		dig.Out
		DatabaseEngine           database.Engine `name:"databaseEngine"`
		DatabasePath             string          `name:"databasePath"`
		TangleDatabasePath       string          `name:"tangleDatabasePath"`
		UTXODatabasePath         string          `name:"utxoDatabasePath"`
		DeleteDatabaseFlag       bool            `name:"deleteDatabase"`
		DeleteAllFlag            bool            `name:"deleteAll"`
		DatabaseDebug            bool            `name:"databaseDebug"`
		DatabaseAutoRevalidation bool            `name:"databaseAutoRevalidation"`
	}

	if err := c.Provide(func(deps cfgDeps) cfgResult {
		dbEngine, err := database.DatabaseEngine(deps.NodeConfig.String(CfgDatabaseEngine))
		if err != nil {
			CorePlugin.Panic(err)
		}

		databasePath := deps.NodeConfig.String(CfgDatabasePath)

		return cfgResult{
			DatabaseEngine:           dbEngine,
			DatabasePath:             databasePath,
			TangleDatabasePath:       filepath.Join(databasePath, "tangle"),
			UTXODatabasePath:         filepath.Join(databasePath, "utxo"),
			DeleteDatabaseFlag:       *deleteDatabase,
			DeleteAllFlag:            *deleteAll,
			DatabaseDebug:            deps.NodeConfig.Bool(CfgDatabaseDebug),
			DatabaseAutoRevalidation: deps.NodeConfig.Bool(CfgDatabaseAutoRevalidation),
		}
	}); err != nil {
		CorePlugin.Panic(err)
	}
}

func provide(c *dig.Container) {

	type dbResult struct {
		dig.Out
		StorageMetrics  *metrics.StorageMetrics
		DatabaseMetrics *metrics.DatabaseMetrics
	}

	if err := c.Provide(func() dbResult {
		return dbResult{
			StorageMetrics:  &metrics.StorageMetrics{},
			DatabaseMetrics: &metrics.DatabaseMetrics{},
		}
	}); err != nil {
		CorePlugin.Panic(err)
	}

	type databaseDeps struct {
		dig.In
		DeleteDatabaseFlag bool                         `name:"deleteDatabase"`
		DeleteAllFlag      bool                         `name:"deleteAll"`
		NodeConfig         *configuration.Configuration `name:"nodeConfig"`
		DatabaseEngine     database.Engine              `name:"databaseEngine"`
		DatabasePath       string                       `name:"databasePath"`
		UTXODatabasePath   string                       `name:"utxoDatabasePath"`
		TangleDatabasePath string                       `name:"tangleDatabasePath"`
		Metrics            *metrics.DatabaseMetrics
	}

	type databaseOut struct {
		dig.Out
		TangleDatabase *database.Database `name:"tangleDatabase"`
		UTXODatabase   *database.Database `name:"utxoDatabase"`
	}

	if err := c.Provide(func(deps databaseDeps) databaseOut {

		events := &database.Events{
			DatabaseCleanup:    events.NewEvent(database.DatabaseCleanupCaller),
			DatabaseCompaction: events.NewEvent(events.BoolCaller),
		}

		if deps.DeleteDatabaseFlag || deps.DeleteAllFlag {
			// delete old database folder
			if err := os.RemoveAll(deps.DatabasePath); err != nil {
				CorePlugin.Panicf("deleting database folder failed: %s", err)
			}
		}

		targetEngine, err := database.CheckDatabaseEngine(deps.TangleDatabasePath, true, deps.DatabaseEngine)
		if err != nil {
			CorePlugin.Panic(err)
		}

		_, err = database.CheckDatabaseEngine(deps.UTXODatabasePath, true, deps.DatabaseEngine)
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
				events.DatabaseCompaction.Trigger(running)
			}

			db, err := database.NewPebbleDB(deps.TangleDatabasePath, reportCompactionRunning, true)
			if err != nil {
				CorePlugin.Panicf("database initialization failed: %s", err)
			}

			tangleDatabase := database.New(
				CorePlugin.Logger(),
				deps.TangleDatabasePath,
				pebble.New(db),
				events,
				true,
				func() bool {
					return deps.Metrics.CompactionRunning.Load()
				},
			)

			//TODO: handle compaction here and events
			utxoDb, err := database.NewPebbleDB(deps.UTXODatabasePath, func(running bool) {}, true)
			if err != nil {
				CorePlugin.Panicf("database initialization failed: %s", err)
			}

			utxoDatabase := database.New(
				CorePlugin.Logger(),
				deps.UTXODatabasePath,
				pebble.New(utxoDb),
				nil,
				false,
				func() bool {
					return false
				},
			)

			return databaseOut{
				TangleDatabase: tangleDatabase,
				UTXODatabase:   utxoDatabase,
			}

		case database.EngineRocksDB:
			db, err := database.NewRocksDB(deps.TangleDatabasePath)
			if err != nil {
				CorePlugin.Panicf("tangle database initialization failed: %s", err)
			}

			tangleDatabase := database.New(
				CorePlugin.Logger(),
				deps.TangleDatabasePath,
				rocksdb.New(db),
				events,
				true,
				func() bool {
					if numCompactions, success := db.GetIntProperty("rocksdb.num-running-compactions"); success {
						runningBefore := deps.Metrics.CompactionRunning.Load()
						running := numCompactions != 0

						deps.Metrics.CompactionRunning.Store(running)
						if running && !runningBefore {
							// we may miss some compactions, since this is only calculated if polled.
							deps.Metrics.CompactionCount.Inc()
							events.DatabaseCompaction.Trigger(running)
						}
						return running
					}
					return false
				},
			)

			utxoDb, err := database.NewRocksDB(deps.UTXODatabasePath)
			if err != nil {
				CorePlugin.Panicf("utxo database initialization failed: %s", err)
			}

			//TODO: handle compaction here and events
			utxoDatabase := database.New(
				CorePlugin.Logger(),
				deps.UTXODatabasePath,
				rocksdb.New(utxoDb),
				nil,
				false,
				func() bool {
					return false
				},
			)

			return databaseOut{
				TangleDatabase: tangleDatabase,
				UTXODatabase:   utxoDatabase,
			}

		default:
			CorePlugin.Panicf("unknown database engine: %s, supported engines: pebble/rocksdb", targetEngine)
			return databaseOut{}
		}
	}); err != nil {
		CorePlugin.Panic(err)
	}

	if err := c.Provide(func(coordinatorPublicKeyRanges coordinator.PublicKeyRanges) *keymanager.KeyManager {
		keyManager := keymanager.New()
		for _, keyRange := range coordinatorPublicKeyRanges {
			pubKey, err := utils.ParseEd25519PublicKeyFromString(keyRange.Key)
			if err != nil {
				CorePlugin.Panicf("can't load public key ranges: %s", err)
			}

			keyManager.AddKeyRange(pubKey, keyRange.StartIndex, keyRange.EndIndex)
		}

		return keyManager
	}); err != nil {
		CorePlugin.Panic(err)
	}

	type storageDeps struct {
		dig.In
		TangleDatabase *database.Database `name:"tangleDatabase"`
		UTXODatabase   *database.Database `name:"utxoDatabase"`
		Profile        *profile.Profile
	}

	if err := c.Provide(func(deps storageDeps) *storage.Storage {

		store, err := storage.New(deps.TangleDatabase.KVStore(), deps.UTXODatabase.KVStore(), deps.Profile.Caches)
		if err != nil {
			CorePlugin.Panicf("can't initialize storage: %s", err)
		}
		return store
	}); err != nil {
		CorePlugin.Panic(err)
	}

	if err := c.Provide(func(storage *storage.Storage) *utxo.Manager {
		return storage.UTXOManager()
	}); err != nil {
		CorePlugin.Panic(err)
	}

	type syncManagerDeps struct {
		dig.In
		UTXOManager   *utxo.Manager
		BelowMaxDepth int `name:"belowMaxDepth"`
	}

	if err := c.Provide(func(deps syncManagerDeps) *syncmanager.SyncManager {
		sync, err := syncmanager.New(deps.UTXOManager, deps.BelowMaxDepth)
		if err != nil {
			CorePlugin.Panicf("can't initialize sync manager: %s", err)
		}
		return sync
	}); err != nil {
		CorePlugin.Panic(err)
	}
}

func configure() {

	correctDatabasesVersion, err := deps.Storage.CheckCorrectDatabasesVersion()
	if err != nil {
		CorePlugin.Panic(err)
	}

	if !correctDatabasesVersion {
		databaseVersionUpdated, err := deps.Storage.UpdateDatabasesVersion()
		if err != nil {
			CorePlugin.Panic(err)
		}

		if !databaseVersionUpdated {
			CorePlugin.Panic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new snapshot.")
		}
	}

	if err = CorePlugin.Daemon().BackgroundWorker("Close database", func(ctx context.Context) {
		<-ctx.Done()

		if err = deps.Storage.MarkDatabasesHealthy(); err != nil {
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
	if err := CorePlugin.Daemon().BackgroundWorker("Database[Events]", func(ctx context.Context) {
		attachEvents()
		<-ctx.Done()
		detachEvents()
	}, shutdown.PriorityMetricsUpdater); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}
}

func closeDatabases() error {

	if err := deps.TangleDatabase.KVStore().Flush(); err != nil {
		return err
	}

	if err := deps.TangleDatabase.KVStore().Close(); err != nil {
		return err
	}

	if err := deps.UTXODatabase.KVStore().Flush(); err != nil {
		return err
	}

	if err := deps.UTXODatabase.KVStore().Close(); err != nil {
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
