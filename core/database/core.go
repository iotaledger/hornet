package database

import (
	"sync"
	"time"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/gohornet/hornet/pkg/shutdown"
)

var (
	CoreModule *node.CoreModule
	log        *logger.Logger

	garbageCollectionLock syncutils.Mutex

	tangleOnce sync.Once
	tangleObj  *tangle.Tangle
)

func Tangle() *tangle.Tangle {
	tangleOnce.Do(func() {
		tangleObj = tangle.New(config.NodeConfig.String(config.CfgDatabasePath), &profile.LoadProfile().Caches)
	})

	return tangleObj
}

func init() {
	CoreModule = node.NewCoreModule("Database", configure, run)
}

func configure(coreModule *node.CoreModule) {
	log = logger.NewLogger(coreModule.Name)

	if !Tangle().IsCorrectDatabaseVersion() {
		if !Tangle().UpdateDatabaseVersion() {
			log.Panic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new local snapshot.")
		}
	}

	CoreModule.Daemon().BackgroundWorker("Close database", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		Tangle().MarkDatabaseHealthy()
		log.Info("Syncing databases to disk...")
		Tangle().CloseDatabases()
		log.Info("Syncing databases to disk... done")
	}, shutdown.PriorityCloseDatabase)
}

func RunGarbageCollection() {
	if Tangle().DatabaseSupportsCleanup() {

		garbageCollectionLock.Lock()
		defer garbageCollectionLock.Unlock()

		log.Info("running full database garbage collection. This can take a while...")

		start := time.Now()

		Events.DatabaseCleanup.Trigger(&DatabaseCleanup{
			Start: start,
		})

		err := Tangle().CleanupDatabases()

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

func run(_ *node.CoreModule) {
	// do nothing
}
