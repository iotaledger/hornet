package spammer

import (
	"runtime"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/parameter"
	"github.com/gohornet/hornet/packages/shutdown"
)

var (
	PLUGIN = node.NewPlugin("Spammer", node.Disabled, configure, run)
	log    *logger.Logger

	address            string
	message            string
	tagSubstring       string
	depth              uint
	rateLimit          float64
	mwm                int
	spammerWorkerCount int
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("Spammer", logger.LogLevel(parameter.NodeConfig.GetInt("node.logLevel")))

	address = trinary.Pad(parameter.NodeConfig.GetString("spammer.address"), consts.AddressTrinarySize/3)[:consts.AddressTrinarySize/3]
	message = parameter.NodeConfig.GetString("spammer.message")
	tagSubstring = trinary.Pad(parameter.NodeConfig.GetString("spammer.tag"), consts.TagTrinarySize/3)[:consts.TagTrinarySize/3]
	depth = parameter.NodeConfig.GetUint("spammer.depth")
	rateLimit = parameter.NodeConfig.GetFloat64("spammer.tpsRateLimit")
	mwm = parameter.NodeConfig.GetInt("protocol.mwm")
	spammerWorkerCount = int(parameter.NodeConfig.GetUint("spammer.workers"))

	if spammerWorkerCount >= runtime.NumCPU() {
		spammerWorkerCount = runtime.NumCPU() - 1
	}
	if spammerWorkerCount < 1 {
		spammerWorkerCount = 1
	}

	if int64(rateLimit) != 0 {
		rateLimitChannelSize := int64(rateLimit) * 2
		if rateLimitChannelSize < 2 {
			rateLimitChannelSize = 2
		}
		rateLimitChannel = make(chan struct{}, rateLimitChannelSize)

		// create a background worker that fills rateLimitChannel every second
		daemon.BackgroundWorker("Spammer rate limit channel", func(shutdownSignal <-chan struct{}) {
			timeutil.Ticker(func() {
				select {
				case rateLimitChannel <- struct{}{}:
				default:
					// Channel full
				}
			}, time.Duration(int64(float64(time.Second)/rateLimit)), shutdownSignal)
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
					if tangle.IsNodeSynced() {
						doSpam(shutdownSignal)
					}
				}
			}
		}, shutdown.ShutdownPrioritySpammer)
	}
}
