package database

import (
	"fmt"
	"time"

	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/badger"
	"github.com/iotaledger/hive.go/kvstore/bolt"
	"github.com/iotaledger/hive.go/kvstore/pebble"
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
		},
	}
}

var (
	CorePlugin *node.CorePlugin
	log        *logger.Logger
	deps       dependencies

	garbageCollectionLock syncutils.Mutex
)

type dependencies struct {
	dig.In
	Store   kvstore.KVStore
	Storage *storage.Storage
	Events  *Events
}

func provide(c *dig.Container) {

	if err := c.Provide(func() *metrics.DatabaseMetrics {
		return &metrics.DatabaseMetrics{}
	}); err != nil {
		panic(err)
	}

	if err := c.Provide(func() *Events {
		return &Events{
			DatabaseCleanup:    events.NewEvent(DatabaseCleanupCaller),
			DatabaseCompaction: events.NewEvent(DatabaseCompactionCaller),
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
		switch deps.NodeConfig.String(CfgDatabaseEngine) {
		case "pebble":
			return pebble.New(database.NewPebbleDB(deps.NodeConfig.String(CfgDatabasePath), false))
		case "bolt":
			return bolt.New(database.NewBoltDB(deps.NodeConfig.String(CfgDatabasePath), "tangle.db"))
		case "badger":
			return badger.New(database.NewBadgerDB(deps.NodeConfig.String(CfgDatabasePath)))
		default:
			panic(fmt.Sprintf("unknown database engine: %s, supported engines: pebble/bolt/badger", deps.NodeConfig.String(CfgDatabaseEngine)))
		}
	}); err != nil {
		panic(err)
	}

	type storagedeps struct {
		dig.In
		NodeConfig    *configuration.Configuration `name:"nodeConfig"`
		Store         kvstore.KVStore
		Profile       *profile.Profile
		BelowMaxDepth int `name:"belowMaxDepth"`
	}

	if err := c.Provide(func(deps storagedeps) *storage.Storage {
		return storage.New(deps.NodeConfig.String(CfgDatabasePath), deps.Store, deps.Profile.Caches, deps.BelowMaxDepth)
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
