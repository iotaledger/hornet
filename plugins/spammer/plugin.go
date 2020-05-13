package spammer

import (
	"fmt"
	"runtime"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"
	"go.uber.org/atomic"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/shutdown"
)

var (
	PLUGIN = node.NewPlugin("Spammer", node.Disabled, configure, run)
	log    *logger.Logger

	address            string
	message            string
	tagSubstring       string
	depth              uint
	cpuMaxUsage        float64
	rateLimit          float64
	mwm                int
	spammerWorkerCount int
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	address = trinary.MustPad(config.NodeConfig.GetString(config.CfgSpammerAddress), consts.AddressTrinarySize/3)[:consts.AddressTrinarySize/3]
	message = config.NodeConfig.GetString(config.CfgSpammerMessage)
	tagSubstring = trinary.MustPad(config.NodeConfig.GetString(config.CfgSpammerTag), consts.TagTrinarySize/3)[:consts.TagTrinarySize/3]
	depth = config.NodeConfig.GetUint(config.CfgSpammerDepth)
	cpuMaxUsage = config.NodeConfig.GetFloat64(config.CfgSpammerCPUMaxUsage)
	rateLimit = config.NodeConfig.GetFloat64(config.CfgSpammerTPSRateLimit)
	mwm = config.NodeConfig.GetInt(config.CfgCoordinatorMWM)
	spammerWorkerCount = int(config.NodeConfig.GetUint(config.CfgSpammerWorkers))

	if spammerWorkerCount >= runtime.NumCPU() {
		spammerWorkerCount = runtime.NumCPU() - 1
	}
	if spammerWorkerCount < 1 {
		spammerWorkerCount = 1
	}

	if cpuMaxUsage > 0.0 {
		if runtime.GOOS == "windows" {
			log.Panic("spammer.cpuMaxUsage not supported on Windows")
		}
		rateLimit = 0.0 // disable rateLimit because we want to spam as much as possible with cpu usage constrains
		spammerWorkerCount = runtime.NumCPU() - 1
	}

	if rateLimit != 0 {
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
		}, shutdown.PrioritySpammer)
	}

	if len(tagSubstring) > 20 {
		tagSubstring = string([]rune(tagSubstring)[:20])
	}
}

func run(_ *node.Plugin) {

	spammerCnt := atomic.NewInt32(0)

	for i := 0; i < spammerWorkerCount; i++ {
		daemon.BackgroundWorker(fmt.Sprintf("Spammer_%d", i), func(shutdownSignal <-chan struct{}) {
			spammerIndex := spammerCnt.Inc()
			log.Infof("Starting Spammer %d... done", spammerIndex)

			for {
				select {
				case <-shutdownSignal:
					log.Infof("Stopping Spammer %d...", spammerIndex)
					log.Infof("Stopping Spammer %d... done", spammerIndex)
					return
				default:
					doSpam(shutdownSignal)
				}
			}
		}, shutdown.PrioritySpammer)
	}
}
