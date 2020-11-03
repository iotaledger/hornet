package database

import (
	"time"

	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/gohornet/hornet/pkg/shutdown"
)

var (
	CoreModule *node.CoreModule
	log        *logger.Logger

	garbageCollectionLock syncutils.Mutex

	tangleObj *tangle.Tangle
)

func init() {
	CoreModule = node.NewCoreModule("Database", configure, run)

	CoreModule.Events.Init.Attach(events.NewClosure(func(_ *node.CoreModule, c *dig.Container) {
		type tangledeps struct {
			config *configuration.Configuration `name:"nodeConfig"`
		}

		if err := c.Provide(func(params tangledeps) *tangle.Tangle {
			return tangle.New(params.config.String(config.CfgDatabasePath), &profile.LoadProfile().Caches)
		}); err != nil {
			panic(err)
		}

		if err := c.Provide(func(tangle *tangle.Tangle) *utxo.Manager {
			return tangle.UTXO()
		}); err != nil {
			panic(err)
		}
	}))
}

func configure(c *dig.Container) {
	log = logger.NewLogger(CoreModule.Name)

	if err := c.Invoke(func(tangle *tangle.Tangle) error {
		tangleObj = tangle
		return nil
	}); err != nil {
		panic(err)
	}

	if !tangleObj.IsCorrectDatabaseVersion() {
		if !tangleObj.UpdateDatabaseVersion() {
			log.Panic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new local snapshot.")
		}
	}

	CoreModule.Daemon().BackgroundWorker("Close database", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		tangleObj.MarkDatabaseHealthy()
		log.Info("Syncing databases to disk...")
		tangleObj.CloseDatabases()
		log.Info("Syncing databases to disk... done")
	}, shutdown.PriorityCloseDatabase)
}

func RunGarbageCollection() {
	if tangleObj.DatabaseSupportsCleanup() {

		garbageCollectionLock.Lock()
		defer garbageCollectionLock.Unlock()

		log.Info("running full database garbage collection. This can take a while...")

		start := time.Now()

		Events.DatabaseCleanup.Trigger(&DatabaseCleanup{
			Start: start,
		})

		err := tangleObj.CleanupDatabases()

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
}

func run(_ *dig.Container) {
	// do nothing
}
