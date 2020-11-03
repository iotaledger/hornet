package gracefulshutdown

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/logger"
	"go.uber.org/dig"
)

// the maximum amount of time to wait for background processes to terminate. After that the process is killed.
const waitToKillTimeInSeconds = 300

var (
	CoreModule *node.CoreModule
	log        *logger.Logger
)

func init() {
	CoreModule = node.NewCoreModule("Graceful Shutdown", configure, run)
}

func configure(_ *dig.Container) {
	log = logger.NewLogger(CoreModule.Name)

	gracefulStop := make(chan os.Signal)

	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	go func() {
		<-gracefulStop

		log.Warnf("Received shutdown request - waiting (max %d seconds) to finish processing ...", waitToKillTimeInSeconds)

		go func() {
			start := time.Now()
			for x := range time.Tick(1 * time.Second) {
				secondsSinceStart := x.Sub(start).Seconds()

				if secondsSinceStart <= waitToKillTimeInSeconds {
					processList := ""
					runningBackgroundWorkers := CoreModule.Daemon().GetRunningBackgroundWorkers()
					if len(runningBackgroundWorkers) >= 1 {
						processList = "(" + strings.Join(runningBackgroundWorkers, ", ") + ") "
					}

					log.Warnf("Received shutdown request - waiting (max %d seconds) to finish processing %s...", waitToKillTimeInSeconds-int(secondsSinceStart), processList)
				} else {
					log.Fatal("Background processes did not terminate in time! Forcing shutdown ...")
				}
			}
		}()

		CoreModule.Daemon().ShutdownAndWait()
	}()
}

func run(_ *dig.Container) {
	// nothing
}
