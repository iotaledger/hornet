package spammer

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/transaction"
	"go.uber.org/atomic"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/spammer"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/plugins/coordinator"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/peering"
	"github.com/gohornet/hornet/plugins/pow"
	"github.com/gohornet/hornet/plugins/urts"
)

var (
	PLUGIN = node.NewPlugin("Spammer", node.Disabled, configure, run)
	log    *logger.Logger

	spammerInstance *spammer.Spammer
	spammerLock     syncutils.RWMutex

	spammerStartTime   time.Time
	spammerAvgHeap     *utils.TimeHeap
	lastSentSpamTxsCnt uint32

	cpuUsageLock   syncutils.RWMutex
	cpuUsageResult float64
	cpuUsageError  error

	processID        atomic.Uint32
	spammerWaitGroup sync.WaitGroup

	// events of the spammer
	Events = &spammer.SpammerEvents{
		SpamPerformed:         events.NewEvent(spammer.SpamStatsCaller),
		AvgSpamMetricsUpdated: events.NewEvent(spammer.AvgSpamMetricsCaller),
	}

	// ErrSpammerDisabled is returned if the spammer plugin is disabled.
	ErrSpammerDisabled = errors.New("Spammer plugin disabled")
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	// do not enable the spammer if URTS is disabled
	if node.IsSkipped(urts.PLUGIN) {
		plugin.Status = node.Disabled
		return
	}

	spammerAvgHeap = utils.NewTimeHeap()

	// start the CPU usage updater
	cpuUsageUpdater()

	// helper function to send the bundle to the network
	sendBundle := func(b bundle.Bundle) error {
		for _, t := range b {
			tx := t // assign to new variable, otherwise it would be overwritten by the loop before processed
			txTrits, _ := transaction.TransactionToTrits(&tx)
			if err := gossip.Processor().CompressAndEmit(&tx, txTrits); err != nil {
				return err
			}
			metrics.SharedServerMetrics.SentSpamTransactions.Inc()
		}
		return nil
	}

	spammerInstance = spammer.New(
		config.NodeConfig.GetString(config.CfgSpammerAddress),
		config.NodeConfig.GetString(config.CfgSpammerMessage),
		config.NodeConfig.GetString(config.CfgSpammerTag),
		config.NodeConfig.GetString(config.CfgSpammerTagSemiLazy),
		urts.TipSelector.SelectSpammerTips,
		config.NodeConfig.GetInt(config.CfgCoordinatorMWM),
		pow.Handler(),
		sendBundle,
	)
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

	// automatically start the spammer on node startup if the flag is set
	if config.NodeConfig.GetBool(config.CfgSpammerAutostart) {
		Start(nil, nil, nil, nil)
	}
}

// Start starts the spammer to spam with the given settings, otherwise it uses the settings from the config.
func Start(tpsRateLimit *float64, cpuMaxUsage *float64, bundleSize *int, valueSpam *bool) (float64, float64, int, bool, error) {
	if spammerInstance == nil {
		return 0.0, 0.0, 0, false, ErrSpammerDisabled
	}

	spammerLock.Lock()
	defer spammerLock.Unlock()

	stopWithoutLocking()

	tpsRateLimitCfg := config.NodeConfig.GetFloat64(config.CfgSpammerTPSRateLimit)
	cpuMaxUsageCfg := config.NodeConfig.GetFloat64(config.CfgSpammerCPUMaxUsage)
	bundleSizeCfg := config.NodeConfig.GetInt(config.CfgSpammerBundleSize)
	valueSpamCfg := config.NodeConfig.GetBool(config.CfgSpammerValueSpam)
	spammerWorkerCount := config.NodeConfig.GetInt(config.CfgSpammerWorkers)
	checkPeersConnected := node.IsSkipped(coordinator.PLUGIN)

	if tpsRateLimit != nil {
		tpsRateLimitCfg = *tpsRateLimit
	}

	if cpuMaxUsage != nil {
		cpuMaxUsageCfg = *cpuMaxUsage
	}

	if cpuMaxUsageCfg > 0.0 && runtime.GOOS == "windows" {
		log.Warn("spammer.cpuMaxUsage not supported on Windows. will be deactivated")
		cpuMaxUsageCfg = 0.0
	}

	if cpuMaxUsageCfg > 0.0 && runtime.NumCPU() == 1 {
		log.Warn("spammer.cpuMaxUsage not supported on single core machines. will be deactivated")
		cpuMaxUsageCfg = 0.0
	}

	if bundleSize != nil {
		bundleSizeCfg = *bundleSize
	}

	if valueSpam != nil {
		valueSpamCfg = *valueSpam
	}

	if bundleSizeCfg < 1 {
		bundleSizeCfg = 1
	}

	if valueSpamCfg && bundleSizeCfg < 2 {
		// minimum size for a value tx with SecurityLevelLow
		bundleSizeCfg = 2
	}

	if spammerWorkerCount >= runtime.NumCPU() || spammerWorkerCount == 0 {
		spammerWorkerCount = runtime.NumCPU() - 1
	}
	if spammerWorkerCount < 1 {
		spammerWorkerCount = 1
	}

	startSpammerWorkers(tpsRateLimitCfg, cpuMaxUsageCfg, bundleSizeCfg, valueSpamCfg, spammerWorkerCount, checkPeersConnected)

	return tpsRateLimitCfg, cpuMaxUsageCfg, bundleSizeCfg, valueSpamCfg, nil
}

func startSpammerWorkers(tpsRateLimit float64, cpuMaxUsage float64, bundleSize int, valueSpam bool, spammerWorkerCount int, checkPeersConnected bool) {

	var rateLimitChannel chan struct{} = nil
	var rateLimitAbortSignal chan struct{} = nil

	if tpsRateLimit != 0.0 {
		rateLimitChannelSize := int64(tpsRateLimit) * 2
		if rateLimitChannelSize < 2 {
			rateLimitChannelSize = 2
		}
		rateLimitChannel = make(chan struct{}, rateLimitChannelSize)
		rateLimitAbortSignal = make(chan struct{})

		// create a background worker that fills rateLimitChannel every second
		daemon.BackgroundWorker("Spammer rate limit channel", func(shutdownSignal <-chan struct{}) {
			spammerWaitGroup.Add(1)
			done := make(chan struct{})
			currentProcessID := processID.Load()

			timeutil.Ticker(func() {

				if currentProcessID != processID.Load() {
					close(rateLimitAbortSignal)
					close(done)
					return
				}

				select {
				case <-shutdownSignal:
					close(rateLimitAbortSignal)
					close(done)
				case rateLimitChannel <- struct{}{}:
				default:
					// Channel full
				}
			}, time.Duration(int64(float64(time.Second)/tpsRateLimit)), done)

			spammerWaitGroup.Done()
		}, shutdown.PrioritySpammer)
	}

	spammerCnt := atomic.NewInt32(0)
	for i := 0; i < spammerWorkerCount; i++ {
		daemon.BackgroundWorker(fmt.Sprintf("Spammer_%d", i), func(shutdownSignal <-chan struct{}) {
			spammerWaitGroup.Add(1)
			spammerIndex := spammerCnt.Inc()
			currentProcessID := processID.Load()

			log.Infof("Starting Spammer %d... done", spammerIndex)

		spammerLoop:
			for {
				select {
				case <-shutdownSignal:
					break spammerLoop
				default:
					if currentProcessID != processID.Load() {
						break spammerLoop
					}

					if tpsRateLimit != 0 {
						// if rateLimit is activated, wait until this spammer thread gets a signal
						select {
						case <-rateLimitAbortSignal:
							break spammerLoop
						case <-shutdownSignal:
							break spammerLoop
						case <-rateLimitChannel:
						}
					}

					if !tangle.IsNodeSyncedWithThreshold() {
						time.Sleep(time.Second)
						continue
					}

					if checkPeersConnected && peering.Manager().ConnectedPeerCount() == 0 {
						time.Sleep(time.Second)
						continue
					}

					if err := waitForLowerCPUUsage(cpuMaxUsage, shutdownSignal); err != nil {
						if err != tangle.ErrOperationAborted {
							log.Warn(err.Error())
						}
						continue
					}

					if spammerStartTime.IsZero() {
						// set the start time for the metrics
						spammerStartTime = time.Now()
					}

					durationGTTA, durationPOW, err := spammerInstance.DoSpam(bundleSize, valueSpam, shutdownSignal)
					if err != nil {
						continue
					}
					Events.SpamPerformed.Trigger(&spammer.SpamStats{GTTA: float32(durationGTTA.Seconds()), POW: float32(durationPOW.Seconds())})
				}
			}

			log.Infof("Stopping Spammer %d...", spammerIndex)
			log.Infof("Stopping Spammer %d... done", spammerIndex)
			spammerWaitGroup.Done()

		}, shutdown.PrioritySpammer)
	}
}

// Stop stops the spammer.
func Stop() error {
	if spammerInstance == nil {
		return ErrSpammerDisabled
	}

	spammerLock.Lock()
	defer spammerLock.Unlock()

	stopWithoutLocking()

	return nil
}

func stopWithoutLocking() {
	// increase the process ID to stop all running workers
	processID.Inc()

	// wait until all spammers are stopped
	spammerWaitGroup.Wait()

	// reset the start time to stop the metrics
	spammerStartTime = time.Time{}

	// clear the metrics heap
	for spammerAvgHeap.Len() > 0 {
		spammerAvgHeap.Pop()
	}
}

// measureSpammerMetrics measures the spammer metrics.
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
	Events.AvgSpamMetricsUpdated.Trigger(&spammer.AvgSpamMetrics{
		New:              new,
		AveragePerSecond: spammerAvgHeap.GetAveragePerSecond(timeDiff),
	})
}
