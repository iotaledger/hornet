package gracefulshutdown

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/logger"
)

// the maximum amount of time to wait for background processes to terminate. After that the process is killed.
const waitToKillTimeInSeconds = 300

func init() {
	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
			Name:      "Graceful Shutdown",
			Configure: configure,
		},
	}
}

var (
	CorePlugin       *node.CorePlugin
	log              *logger.Logger
	nodeSelfShutdown = make(chan string)
)

// SelfShutdown can be called in order to instruct the node to shutdown cleanly without receiving any interrupt signals.
func SelfShutdown(msg string) {
	select {
	case nodeSelfShutdown <- msg:
	default:
	}
}

func configure() {
	log = logger.NewLogger(CorePlugin.Name)

	gracefulStop := make(chan os.Signal)

	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	go func() {
		select {
		case <-gracefulStop:
			log.Warnf("Received shutdown request - waiting (max %d seconds) to finish processing ...", waitToKillTimeInSeconds)
		case msg := <-nodeSelfShutdown:
			log.Warnf("Node self-shutdown: %s; waiting (max %d seconds) to finish processing ...", msg, waitToKillTimeInSeconds)
		}

		go func() {
			start := time.Now()
			for x := range time.Tick(1 * time.Second) {
				secondsSinceStart := x.Sub(start).Seconds()

				if secondsSinceStart <= waitToKillTimeInSeconds {
					processList := ""
					runningBackgroundWorkers := CorePlugin.Daemon().GetRunningBackgroundWorkers()
					if len(runningBackgroundWorkers) >= 1 {
						processList = "(" + strings.Join(runningBackgroundWorkers, ", ") + ") "
					}

					log.Warnf("Received shutdown request - waiting (max %d seconds) to finish processing %s...", waitToKillTimeInSeconds-int(secondsSinceStart), processList)
				} else {
					log.Fatal("Background processes did not terminate in time! Forcing shutdown ...")
				}
			}
		}()

		CorePlugin.Daemon().ShutdownAndWait()
	}()
}
