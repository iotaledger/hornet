package gracefulshutdown

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/packages/parameter"
)

// maximum amount of time to wait for background processes to terminate. After that the process is killed.
const WAIT_TO_KILL_TIME_IN_SECONDS = 120

var (
	PLUGIN = node.NewPlugin("Graceful Shutdown", node.Enabled, configure)
	log    *logger.Logger
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("Graceful Shutdown", logger.LogLevel(parameter.NodeConfig.GetInt("node.logLevel")))

	gracefulStop := make(chan os.Signal)

	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	go func() {
		<-gracefulStop

		log.Warningf("Received shutdown request - waiting (max %d seconds) to finish processing ...", WAIT_TO_KILL_TIME_IN_SECONDS)

		go func() {
			start := time.Now()
			for x := range time.Tick(1 * time.Second) {
				secondsSinceStart := x.Sub(start).Seconds()

				if secondsSinceStart <= WAIT_TO_KILL_TIME_IN_SECONDS {
					processList := ""
					runningBackgroundWorkers := daemon.GetRunningBackgroundWorkers()
					if len(runningBackgroundWorkers) >= 1 {
						processList = "(" + strings.Join(runningBackgroundWorkers, ", ") + ") "
					}

					log.Warningf("Received shutdown request - waiting (max %d seconds) to finish processing %s...", WAIT_TO_KILL_TIME_IN_SECONDS-int(secondsSinceStart), processList)
				} else {
					log.Fatal("Background processes did not terminate in time! Forcing shutdown ...")
				}
			}
		}()

		daemon.ShutdownAndWait()
	}()
}
