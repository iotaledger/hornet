package database

import (
	"fmt"
	"os"
	"time"

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
	"github.com/iotaledger/hive.go/kvstore/bolt"
	"github.com/iotaledger/hive.go/kvstore/pebble"
	"github.com/iotaledger/hive.go/kvstore/rocksdb"
	"github.com/iotaledger/hive.go/logger"
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
	log        *logger.Logger
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

	type dbresult struct {
		dig.Out

		StorageMetrics     *metrics.StorageMetrics
		DatabaseMetrics    *metrics.DatabaseMetrics
		DatabaseEvents     *Events
		DeleteDatabaseFlag bool `name:"deleteDatabase"`
		DeleteAllFlag      bool `name:"deleteAll"`
	}

	if err := c.Provide(func() dbresult {

		res := dbresult{
			StorageMetrics:  &metrics.StorageMetrics{},
			DatabaseMetrics: &metrics.DatabaseMetrics{},
			DatabaseEvents: &Events{
				DatabaseCleanup:    events.NewEvent(DatabaseCleanupCaller),
				DatabaseCompaction: events.NewEvent(events.BoolCaller),
			},
			DeleteDatabaseFlag: *deleteDatabase,
			DeleteAllFlag:      *deleteAll,
		}
		return res
	}); err != nil {
		panic(err)
	}

	type dbdeps struct {
		dig.In
		DeleteDatabaseFlag bool                         `name:"deleteDatabase"`
		DeleteAllFlag      bool                         `name:"deleteAll"`
		NodeConfig         *configuration.Configuration `name:"nodeConfig"`
		Events             *Events
		Metrics            *metrics.DatabaseMetrics
	}

	if err := c.Provide(func(deps dbdeps) *database.Database {

		if deps.DeleteDatabaseFlag || deps.DeleteAllFlag {
			// delete old database folder
			if err := os.RemoveAll(deps.NodeConfig.String(CfgDatabasePath)); err != nil {
				log.Panicf("deleting database folder failed: %s", err)
			}
		}

		switch deps.NodeConfig.String(CfgDatabaseEngine) {
		case "pebble":
			reportCompactionRunning := func(running bool) {
				deps.Metrics.CompactionRunning.Store(running)
				if running {
					deps.Metrics.CompactionCount.Inc()
				}
				deps.Events.DatabaseCompaction.Trigger(running)
			}

			return database.New(
				pebble.New(database.NewPebbleDB(deps.NodeConfig.String(CfgDatabasePath), reportCompactionRunning, true)),
				func() bool { return deps.Metrics.CompactionRunning.Load() },
			)

		case "bolt":
			return database.New(
				bolt.New(database.NewBoltDB(deps.NodeConfig.String(CfgDatabasePath), "tangle.db")),
				func() bool { return false },
			)

		case "rocksdb":
			rocksDB := database.NewRocksDB(deps.NodeConfig.String(CfgDatabasePath))
			return database.New(
				rocksdb.New(rocksDB),
				func() bool {
					if numCompactions, success := rocksDB.GetIntProperty("rocksdb.num-running-compactions"); success {
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
			panic(fmt.Sprintf("unknown database engine: %s, supported engines: pebble/bolt/rocksdb", deps.NodeConfig.String(CfgDatabaseEngine)))
		}
	}); err != nil {
		panic(err)
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
				panic(fmt.Sprintf("can't load public key ranges: %s", err))
			}

			keyManager.AddKeyRange(pubKey, keyRange.StartIndex, keyRange.EndIndex)
		}

		return storage.New(deps.NodeConfig.String(CfgDatabasePath), deps.Database.KVStore(), deps.Profile.Caches, deps.BelowMaxDepth, keyManager, deps.NodeConfig.Int(protocfg.CfgProtocolMilestonePublicKeyCount))
	}); err != nil {
		panic(err)
	}

	if err := c.Provide(func(storage *storage.Storage) *utxo.Manager {
		return storage.UTXO()
	}); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(CorePlugin.Name)

	if !deps.Storage.IsCorrectDatabaseVersion() {
		if !deps.Storage.UpdateDatabaseVersion() {
			log.Panic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new snapshot.")
		}
	}

	CorePlugin.Daemon().BackgroundWorker("Close database", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		deps.Storage.MarkDatabaseHealthy()
		log.Info("Syncing databases to disk...")
		closeDatabases()
		log.Info("Syncing databases to disk... done")
	}, shutdown.PriorityCloseDatabase)

	configureEvents()
}

func run() {
	CorePlugin.Daemon().BackgroundWorker("Database[Events]", func(shutdownSignal <-chan struct{}) {
		attachEvents()
		<-shutdownSignal
		detachEvents()
	}, shutdown.PriorityMetricsUpdater)
}

func RunGarbageCollection() {
	if !deps.Storage.DatabaseSupportsCleanup() {
		return
	}

	garbageCollectionLock.Lock()
	defer garbageCollectionLock.Unlock()

	log.Info("running full database garbage collection. This can take a while...")

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
		if err != storage.ErrNothingToCleanUp {
			log.Warnf("full database garbage collection failed with error: %s. took: %v", err, end.Sub(start).Truncate(time.Millisecond))
			return
		}
	}

	log.Infof("full database garbage collection finished. took %v", end.Sub(start).Truncate(time.Millisecond))
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
