package gracefulshutdown

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gohornet/hornet/pkg/node"
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
	CorePlugin *node.CorePlugin

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

	gracefulStop := make(chan os.Signal)

	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	go func() {
		select {
		case <-gracefulStop:
			CorePlugin.LogWarnf("Received shutdown request - waiting (max %d seconds) to finish processing ...", waitToKillTimeInSeconds)
		case msg := <-nodeSelfShutdown:
			CorePlugin.LogWarnf("Node self-shutdown: %s; waiting (max %d seconds) to finish processing ...", msg, waitToKillTimeInSeconds)
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

					CorePlugin.LogWarnf("Received shutdown request - waiting (max %d seconds) to finish processing %s...", waitToKillTimeInSeconds-int(secondsSinceStart), processList)
				} else {
					CorePlugin.LogFatal("Background processes did not terminate in time! Forcing shutdown ...")
				}
			}
		}()

		CorePlugin.Daemon().ShutdownAndWait()
	}()
}
