package database

import (
	"context"
	"os"
	"path/filepath"
	"time"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	hivedb "github.com/iotaledger/hive.go/kvstore/database"
	"github.com/iotaledger/hornet/v2/pkg/components"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/profile"
	"github.com/iotaledger/hornet/v2/pkg/protocol"
)

const (
	// CfgTangleDeleteDatabase defines whether to delete the database at startup.
	CfgTangleDeleteDatabase = "deleteDatabase"
	// CfgTangleDeleteAll defines whether to delete the database and snapshots at startup.
	CfgTangleDeleteAll = "deleteAll"

	// TangleDatabaseDirectoryName defines the subfolder for the tangle database.
	TangleDatabaseDirectoryName = "tangle"
	// UTXODatabaseDirectoryName defines the subfolder for the UTXO database.
	UTXODatabaseDirectoryName = "utxo"
)

func init() {
	Component = &app.Component{
		Name:             "Database",
		DepsFunc:         func(cDeps dependencies) { deps = cDeps },
		Params:           params,
		InitConfigParams: initConfigParams,
		IsEnabled:        components.IsAutopeeringEntryNodeDisabled, // do not enable in "autopeering entry node" mode
		Provide:          provide,
		Configure:        configure,
		Run:              run,
	}
}

var (
	Component *app.Component
	deps      dependencies

	deleteDatabase = flag.Bool(CfgTangleDeleteDatabase, false, "whether to delete the database at startup")
	deleteAll      = flag.Bool(CfgTangleDeleteAll, false, "whether to delete the database and snapshots at startup")
)

type dependencies struct {
	dig.In
	TangleDatabase *database.Database `name:"tangleDatabase"`
	UTXODatabase   *database.Database `name:"utxoDatabase"`
	Storage        *storage.Storage
	StorageMetrics *metrics.StorageMetrics
}

func initConfigParams(c *dig.Container) error {

	type cfgResult struct {
		dig.Out
		DatabaseEngine           hivedb.Engine `name:"databaseEngine"`
		DatabasePath             string        `name:"databasePath"`
		TangleDatabasePath       string        `name:"tangleDatabasePath"`
		UTXODatabasePath         string        `name:"utxoDatabasePath"`
		DeleteDatabaseFlag       bool          `name:"deleteDatabase"`
		DeleteAllFlag            bool          `name:"deleteAll"`
		DatabaseDebug            bool          `name:"databaseDebug"`
		DatabaseAutoRevalidation bool          `name:"databaseAutoRevalidation"`
	}

	if err := c.Provide(func() cfgResult {
		dbEngine, err := hivedb.EngineFromStringAllowed(ParamsDatabase.Engine, database.AllowedEnginesDefault)
		if err != nil {
			Component.LogPanic(err)
		}

		return cfgResult{
			DatabaseEngine:           dbEngine,
			DatabasePath:             ParamsDatabase.Path,
			TangleDatabasePath:       filepath.Join(ParamsDatabase.Path, TangleDatabaseDirectoryName),
			UTXODatabasePath:         filepath.Join(ParamsDatabase.Path, UTXODatabaseDirectoryName),
			DeleteDatabaseFlag:       *deleteDatabase,
			DeleteAllFlag:            *deleteAll,
			DatabaseDebug:            ParamsDatabase.Debug,
			DatabaseAutoRevalidation: ParamsDatabase.AutoRevalidation,
		}
	}); err != nil {
		Component.LogPanic(err)
	}

	return nil
}

func provide(c *dig.Container) error {

	type databaseDeps struct {
		dig.In
		DeleteDatabaseFlag bool          `name:"deleteDatabase"`
		DeleteAllFlag      bool          `name:"deleteAll"`
		DatabaseEngine     hivedb.Engine `name:"databaseEngine"`
		DatabasePath       string        `name:"databasePath"`
		UTXODatabasePath   string        `name:"utxoDatabasePath"`
		TangleDatabasePath string        `name:"tangleDatabasePath"`
	}

	type databaseOut struct {
		dig.Out

		StorageMetrics *metrics.StorageMetrics

		TangleDatabase *database.Database `name:"tangleDatabase"`
		UTXODatabase   *database.Database `name:"utxoDatabase"`
	}

	if err := c.Provide(func(deps databaseDeps) databaseOut {

		checkDatabase := func() hivedb.Engine {

			if deps.DeleteDatabaseFlag || deps.DeleteAllFlag {
				// delete old database folder
				if err := os.RemoveAll(deps.DatabasePath); err != nil {
					Component.LogPanicf("deleting database folder failed: %s", err)
				}
			}

			allowedEngines := database.AllowedEnginesStorageAuto

			tangleTargetEngine, err := database.CheckEngine(deps.TangleDatabasePath, true, deps.DatabaseEngine, allowedEngines...)
			if err != nil {
				Component.LogPanic(err)
			}

			utxoTargetEngine, err := database.CheckEngine(deps.UTXODatabasePath, true, deps.DatabaseEngine, allowedEngines...)
			if err != nil {
				Component.LogPanic(err)
			}

			if tangleTargetEngine != utxoTargetEngine {
				Component.LogPanicf("Tangle database engine does not match UTXO database engine (%s != %s)", tangleTargetEngine, utxoTargetEngine)
			}

			return tangleTargetEngine
		}

		targetEngine := deps.DatabaseEngine
		if targetEngine != hivedb.EngineMapDB {
			// we only need to check the database engine if we don't use an in-memory database
			targetEngine = checkDatabase()
		}

		tangleDatabaseMetrics := &metrics.DatabaseMetrics{}
		utxoDatabaseMetrics := &metrics.DatabaseMetrics{}

		switch targetEngine {
		case hivedb.EnginePebble:
			return databaseOut{
				StorageMetrics: &metrics.StorageMetrics{},
				TangleDatabase: newPebble(deps.TangleDatabasePath, tangleDatabaseMetrics),
				UTXODatabase:   newPebble(deps.UTXODatabasePath, utxoDatabaseMetrics),
			}

		case hivedb.EngineRocksDB:
			return databaseOut{
				StorageMetrics: &metrics.StorageMetrics{},
				TangleDatabase: newRocksDB(deps.TangleDatabasePath, tangleDatabaseMetrics),
				UTXODatabase:   newRocksDB(deps.UTXODatabasePath, utxoDatabaseMetrics),
			}

		case hivedb.EngineMapDB:
			return databaseOut{
				StorageMetrics: &metrics.StorageMetrics{},
				TangleDatabase: newMapDB(tangleDatabaseMetrics),
				UTXODatabase:   newMapDB(utxoDatabaseMetrics),
			}

		default:
			Component.LogPanicf("unknown database engine: %s, supported engines: pebble/rocksdb/mapdb", targetEngine)

			return databaseOut{}
		}
	}); err != nil {
		Component.LogPanic(err)
	}

	type storageDeps struct {
		dig.In
		TangleDatabase *database.Database `name:"tangleDatabase"`
		UTXODatabase   *database.Database `name:"utxoDatabase"`
		Profile        *profile.Profile
	}

	type storageOut struct {
		dig.Out
		Storage     *storage.Storage
		UTXOManager *utxo.Manager
	}

	if err := c.Provide(func(deps storageDeps) storageOut {

		store, err := storage.New(deps.TangleDatabase.KVStore(), deps.UTXODatabase.KVStore(), deps.Profile.Caches)
		if err != nil {
			Component.LogPanicf("can't initialize storage: %s", err)
		}

		store.PrintSnapshotInfo()

		return storageOut{
			Storage:     store,
			UTXOManager: store.UTXOManager(),
		}
	}); err != nil {
		Component.LogPanic(err)
	}

	type syncManagerDeps struct {
		dig.In
		UTXOManager     *utxo.Manager
		ProtocolManager *protocol.Manager
	}

	if err := c.Provide(func(deps syncManagerDeps) *syncmanager.SyncManager {
		ledgerIndex, err := deps.UTXOManager.ReadLedgerIndex()
		if err != nil {
			Component.LogPanicf("can't initialize sync manager: %s", err)
		}

		sync, err := syncmanager.New(ledgerIndex, deps.ProtocolManager)
		if err != nil {
			Component.LogPanicf("can't initialize sync manager: %s", err)
		}

		return sync
	}); err != nil {
		Component.LogPanic(err)
	}

	return nil
}

func configure() error {

	correctDatabasesVersion, err := deps.Storage.CheckCorrectStoresVersion()
	if err != nil {
		Component.LogPanic(err)
	}

	if !correctDatabasesVersion {
		databaseVersionUpdated, err := deps.Storage.UpdateStoresVersion()
		if err != nil {
			Component.LogPanic(err)
		}

		if !databaseVersionUpdated {
			Component.LogPanic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new snapshot.")
		}
	}

	if ParamsDatabase.CheckLedgerStateOnStartup {
		Component.LogInfo("Checking ledger state ...")
		ledgerStateCheckStart := time.Now()
		if err := deps.Storage.CheckLedgerState(); err != nil {
			Component.LogErrorAndExit(err)
		}
		Component.LogInfof("Checking ledger state ... done. took %v", time.Since(ledgerStateCheckStart).Truncate(time.Millisecond))
	}

	if err = Component.Daemon().BackgroundWorker("Close database", func(ctx context.Context) {
		<-ctx.Done()

		if err = deps.Storage.MarkStoresHealthy(); err != nil {
			Component.LogPanic(err)
		}

		Component.LogInfo("Syncing databases to disk ...")
		if err = deps.Storage.FlushAndCloseStores(); err != nil {
			Component.LogPanicf("Syncing databases to disk ... failed: %s", err)
		}
		Component.LogInfo("Syncing databases to disk ... done")
	}, daemon.PriorityCloseDatabase); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}

func run() error {
	if err := Component.Daemon().BackgroundWorker("Database[Events]", func(ctx context.Context) {
		hook := deps.Storage.Events.PruningStateChanged.Hook(func(running bool) {
			deps.StorageMetrics.PruningRunning.Store(running)
			if running {
				deps.StorageMetrics.Prunings.Inc()
			}
		})
		defer hook.Unhook()
		<-ctx.Done()
	}, daemon.PriorityMetricsUpdater); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}
