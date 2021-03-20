package database

import (
	"fmt"
	"time"

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
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/bolt"
	"github.com/iotaledger/hive.go/kvstore/pebble"
	"github.com/iotaledger/hive.go/kvstore/rocksdb"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
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

	garbageCollectionLock syncutils.Mutex

	// Closures
	onPruningStateChanged *events.Closure
)

type dependencies struct {
	dig.In
	Store          kvstore.KVStore
	Storage        *storage.Storage
	Events         *Events
	StorageMetrics *metrics.StorageMetrics
}

func provide(c *dig.Container) {

	if err := c.Provide(func() *metrics.DatabaseMetrics {
		return &metrics.DatabaseMetrics{}
	}); err != nil {
		panic(err)
	}

	if err := c.Provide(func() *metrics.StorageMetrics {
		return &metrics.StorageMetrics{}
	}); err != nil {
		panic(err)
	}

	if err := c.Provide(func() *Events {
		return &Events{
			DatabaseCleanup:    events.NewEvent(DatabaseCleanupCaller),
			DatabaseCompaction: events.NewEvent(events.BoolCaller),
		}
	}); err != nil {
		panic(err)
	}

	type pebbledeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
		Events     *Events
		Metrics    *metrics.DatabaseMetrics
	}

	if err := c.Provide(func(deps pebbledeps) kvstore.KVStore {

		reportCompactionRunning := func(running bool) {
			deps.Metrics.CompactionRunning.Store(running)
			if running {
				deps.Metrics.CompactionCount.Inc()
			}
			deps.Events.DatabaseCompaction.Trigger(running)
		}

		switch deps.NodeConfig.String(CfgDatabaseEngine) {
		case "pebble":
			return pebble.New(database.NewPebbleDB(deps.NodeConfig.String(CfgDatabasePath), reportCompactionRunning, true))
		case "bolt":
			return bolt.New(database.NewBoltDB(deps.NodeConfig.String(CfgDatabasePath), "tangle.db"))
		case "rocksdb":
			return rocksdb.New(database.NewRocksDB(deps.NodeConfig.String(CfgDatabasePath)))
		default:
			panic(fmt.Sprintf("unknown database engine: %s, supported engines: pebble/bolt/rocksdb", deps.NodeConfig.String(CfgDatabaseEngine)))
		}
	}); err != nil {
		panic(err)
	}

	type storagedeps struct {
		dig.In
		NodeConfig                 *configuration.Configuration `name:"nodeConfig"`
		Store                      kvstore.KVStore
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

		return storage.New(deps.NodeConfig.String(CfgDatabasePath), deps.Store, deps.Profile.Caches, deps.BelowMaxDepth, keyManager, deps.NodeConfig.Int(protocfg.CfgProtocolMilestonePublicKeyCount))
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

	if err := deps.Store.Flush(); err != nil {
		return err
	}

	if err := deps.Store.Close(); err != nil {
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
