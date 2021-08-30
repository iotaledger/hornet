package shutdown

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
)

const (
	// the maximum amount of time to wait for background processes to terminate. After that the process is killed.
	waitToKillTimeInSeconds = 300
)

// ShutdownHandler waits until a shutdown signal was received or the node tried to shutdown itself,
// and shuts down all processes gracefully.
type ShutdownHandler struct {
	log              *logger.Logger
	daemon           daemon.Daemon
	gracefulStop     chan os.Signal
	nodeSelfShutdown chan string
}

// NewShutdownHandler creates a new shutdown handler.
func NewShutdownHandler(log *logger.Logger, daemon daemon.Daemon) *ShutdownHandler {

	gs := &ShutdownHandler{
		log:              log,
		daemon:           daemon,
		gracefulStop:     make(chan os.Signal, 1),
		nodeSelfShutdown: make(chan string),
	}

	signal.Notify(gs.gracefulStop, syscall.SIGTERM)
	signal.Notify(gs.gracefulStop, syscall.SIGINT)

	return gs
}

// SelfShutdown can be called in order to instruct the node to shutdown cleanly without receiving any interrupt signals.
func (gs *ShutdownHandler) SelfShutdown(msg string) {
	select {
	case gs.nodeSelfShutdown <- msg:
	default:
	}
}

// Run starts the ShutdownHandler go routine.
func (gs *ShutdownHandler) Run() {

	go func() {
		select {
		case <-gs.gracefulStop:
			gs.log.Warnf("Received shutdown request - waiting (max %d seconds) to finish processing ...", waitToKillTimeInSeconds)
		case msg := <-gs.nodeSelfShutdown:
			gs.log.Warnf("Node self-shutdown: %s; waiting (max %d seconds) to finish processing ...", msg, waitToKillTimeInSeconds)
		}

		go func() {
			start := time.Now()
			for x := range time.Tick(1 * time.Second) {
				secondsSinceStart := x.Sub(start).Seconds()

				if secondsSinceStart <= waitToKillTimeInSeconds {
					processList := ""
					runningBackgroundWorkers := gs.daemon.GetRunningBackgroundWorkers()
					if len(runningBackgroundWorkers) >= 1 {
						processList = "(" + strings.Join(runningBackgroundWorkers, ", ") + ") "
					}

					gs.log.Warnf("Received shutdown request - waiting (max %d seconds) to finish processing %s...", waitToKillTimeInSeconds-int(secondsSinceStart), processList)
				} else {
					gs.log.Fatal("Background processes did not terminate in time! Forcing shutdown ...")
				}
			}
		}()

		gs.daemon.ShutdownAndWait()
	}()
}
