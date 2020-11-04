package database

import (
	"time"

	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/gohornet/hornet/pkg/shutdown"
)

func init() {
	CoreModule = &node.CoreModule{
		Name:      "Database",
		DepsFunc:  func(cDeps dependencies) { deps = cDeps },
		Provide:   provide,
		Configure: configure,
	}
}

var (
	CoreModule            *node.CoreModule
	log                   *logger.Logger
	garbageCollectionLock syncutils.Mutex
	deps                  dependencies
)

type dependencies struct {
	dig.In
	Tangle *tangle.Tangle
}

func provide(c *dig.Container) {
	type tangledeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps tangledeps) *tangle.Tangle {
		return tangle.New(deps.NodeConfig.String(config.CfgDatabasePath), &profile.LoadProfile().Caches)
	}); err != nil {
		panic(err)
	}

	if err := c.Provide(func(tangle *tangle.Tangle) *utxo.Manager {
		return tangle.UTXO()
	}); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(CoreModule.Name)

	if !deps.Tangle.IsCorrectDatabaseVersion() {
		if !deps.Tangle.UpdateDatabaseVersion() {
			log.Panic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new local snapshot.")
		}
	}

	CoreModule.Daemon().BackgroundWorker("Close database", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		deps.Tangle.MarkDatabaseHealthy()
		log.Info("Syncing databases to disk...")
		deps.Tangle.CloseDatabases()
		log.Info("Syncing databases to disk... done")
	}, shutdown.PriorityCloseDatabase)
}

func RunGarbageCollection() {
	if !deps.Tangle.DatabaseSupportsCleanup() {
		return
	}

	garbageCollectionLock.Lock()
	defer garbageCollectionLock.Unlock()

	log.Info("running full database garbage collection. This can take a while...")

	start := time.Now()

	Events.DatabaseCleanup.Trigger(&DatabaseCleanup{
		Start: start,
	})

	err := deps.Tangle.CleanupDatabases()

	end := time.Now()

	Events.DatabaseCleanup.Trigger(&DatabaseCleanup{
		Start: start,
		End:   end,
	})

	if err != nil {
		if err != tangle.ErrNothingToCleanUp {
			log.Warnf("full database garbage collection failed with error: %s. took: %v", err.Error(), end.Sub(start).Truncate(time.Millisecond))
			return
		}
	}

	log.Infof("full database garbage collection finished. took %v", end.Sub(start).Truncate(time.Millisecond))
}
