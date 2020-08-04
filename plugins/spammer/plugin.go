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
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/plugins/coordinator"
	"github.com/gohornet/hornet/plugins/urts"
)

var (
	PLUGIN = node.NewPlugin("Spammer", node.Disabled, configure, run)
	log    *logger.Logger

	txAddress            string
	message              string
	tagSubstring         string
	tagSemiLazySubstring string
	cpuMaxUsage          float64
	rateLimit            float64
	mwm                  int
	bundleSize           int
	valueSpam            bool
	spammerWorkerCount   int
	semiLazyTipsLimit    uint32
	checkPeersConnected  bool
	spammerAvgHeap       *utils.TimeHeap
	spammerStartTime     time.Time
	lastSentSpamTxsCnt   uint32
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	// do not enable the spammer if URTS is disabled
	if node.IsSkipped(urts.PLUGIN) {
		plugin.Status = node.Disabled
		return
	}

	txAddress = trinary.MustPad(config.NodeConfig.GetString(config.CfgSpammerAddress), consts.AddressTrinarySize/3)[:consts.AddressTrinarySize/3]
	message = config.NodeConfig.GetString(config.CfgSpammerMessage)
	tagSubstring = trinary.MustPad(config.NodeConfig.GetString(config.CfgSpammerTag), consts.TagTrinarySize/3)[:consts.TagTrinarySize/3]
	tagSemiLazySubstring = trinary.MustPad(config.NodeConfig.GetString(config.CfgSpammerTag), consts.TagTrinarySize/3)[:consts.TagTrinarySize/3]
	if config.NodeConfig.GetString(config.CfgSpammerTagSemiLazy) != "" {
		tagSemiLazySubstring = trinary.MustPad(config.NodeConfig.GetString(config.CfgSpammerTagSemiLazy), consts.TagTrinarySize/3)[:consts.TagTrinarySize/3]
	}
	cpuMaxUsage = config.NodeConfig.GetFloat64(config.CfgSpammerCPUMaxUsage)
	rateLimit = config.NodeConfig.GetFloat64(config.CfgSpammerTPSRateLimit)
	mwm = config.NodeConfig.GetInt(config.CfgCoordinatorMWM)
	bundleSize = config.NodeConfig.GetInt(config.CfgSpammerBundleSize)
	valueSpam = config.NodeConfig.GetBool(config.CfgSpammerValueSpam)
	spammerWorkerCount = int(config.NodeConfig.GetUint(config.CfgSpammerWorkers))
	semiLazyTipsLimit = config.NodeConfig.GetUint32(config.CfgSpammerSemiLazyTipsLimit)
	checkPeersConnected = node.IsSkipped(coordinator.PLUGIN)
	spammerAvgHeap = utils.NewTimeHeap()

	if bundleSize < 1 {
		bundleSize = 1
	}

	if valueSpam && bundleSize < 2 {
		// minimum size for a value tx with SecurityLevelLow
		bundleSize = 2
	}

	if spammerWorkerCount >= runtime.NumCPU() || spammerWorkerCount == 0 {
		spammerWorkerCount = runtime.NumCPU() - 1
	}
	if spammerWorkerCount < 1 {
		spammerWorkerCount = 1
	}

	if cpuMaxUsage > 0.0 && runtime.GOOS == "windows" {
		log.Warn("spammer.cpuMaxUsage not supported on Windows. will be deactivated")
		cpuMaxUsage = 0.0
	}

	if cpuMaxUsage > 0.0 && runtime.NumCPU() == 1 {
		log.Warn("spammer.cpuMaxUsage not supported on single core machines. will be deactivated")
		cpuMaxUsage = 0.0
	}

	if cpuMaxUsage > 0.0 {
		cpuUsageUpdater()
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
	if len(tagSemiLazySubstring) > 20 {
		tagSemiLazySubstring = string([]rune(tagSemiLazySubstring)[:20])
	}
}

func run(_ *node.Plugin) {

	// do not enable the spammer if URTS is disabled
	if node.IsSkipped(urts.PLUGIN) {
		return
	}

	// create a background worker that "measures" the spammer averages values every second
	daemon.BackgroundWorker("Spammer Metrics Updater", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(measureSpammerMetrics, 1*time.Second, shutdownSignal)
	}, shutdown.PrioritySpammer)

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

// measures the spammer metrics
func measureSpammerMetrics() {
	if spammerStartTime.IsZero() {
		// Spammer not started yet
		return
	}

	sentSpamTxsCnt := metrics.SharedServerMetrics.SentSpamTransactions.Load()
	new := utils.GetUint32Diff(sentSpamTxsCnt, lastSentSpamTxsCnt)
	lastSentSpamTxsCnt = sentSpamTxsCnt

	spammerAvgHeap.Add(uint64(new))

	timeDiff := time.Since(spammerStartTime)
	if timeDiff > 60*time.Second {
		// Only filter over one minute maximum
		timeDiff = 60 * time.Second
	}

	// trigger events for outside listeners
	Events.AvgSpamMetricsUpdated.Trigger(&AvgSpamMetrics{
		New:              new,
		AveragePerSecond: spammerAvgHeap.GetAveragePerSecond(timeDiff),
	})
}
