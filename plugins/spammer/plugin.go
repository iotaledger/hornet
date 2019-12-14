package spammer

import (
	"runtime"
	"time"

	"github.com/gohornet/hornet/packages/logger"
	"github.com/gohornet/hornet/packages/node"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/packages/timeutil"
	daemon "github.com/iotaledger/hive.go/daemon/ordered"
)

var (
	PLUGIN = node.NewPlugin("Spammer", node.Disabled, configure, run)
	log    = logger.NewLogger("Spammer")

	spammerWorkerCount = runtime.NumCPU()
)

func configure(plugin *node.Plugin) {

	if rateLimit != 0 {
		rateLimitChannel = make(chan struct{}, rateLimit*2)

		// create a background worker that fills rateLimitChannel every second
		daemon.BackgroundWorker("Spammer rate limit channel", func(shutdownSignal <-chan struct{}) {
			timeutil.Ticker(func() {
				select {
				case rateLimitChannel <- struct{}{}:
				default:
					// Channel full
				}
			}, time.Duration(int64(time.Second)/int64(rateLimit)), shutdownSignal)
		}, shutdown.ShutdownPrioritySpammer)
	}

	if len(tagSubstring) > 20 {
		tagSubstring = string([]rune(tagSubstring)[:20])
	}
}

func run(plugin *node.Plugin) {

	for i := 0; i < spammerWorkerCount; i++ {
		daemon.BackgroundWorker("Spammer", func(shutdownSignal <-chan struct{}) {
			log.Infof("Starting Spammer %d... done", i)

			for {
				select {
				case <-shutdownSignal:
					log.Infof("Stopping Spammer %d...", i)
					log.Infof("Stopping Spammer %d... done", i)
					return
				default:
					doSpam(shutdownSignal)
				}
			}
		}, shutdown.ShutdownPrioritySpammer)
	}
}
