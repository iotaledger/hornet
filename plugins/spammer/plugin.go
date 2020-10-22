package spammer

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/timeutil"
	"go.uber.org/atomic"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/spammer"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/plugins/coordinator"
	"github.com/gohornet/hornet/plugins/gossip"
	p2pplug "github.com/gohornet/hornet/plugins/p2p"
	"github.com/gohornet/hornet/plugins/pow"
	"github.com/gohornet/hornet/plugins/urts"
)

var (
	PLUGIN = node.NewPlugin("Spammer", node.Disabled, configure, run)
	log    *logger.Logger

	spammerInstance *spammer.Spammer
	spammerLock     syncutils.RWMutex

	spammerStartTime    time.Time
	spammerAvgHeap      *utils.TimeHeap
	lastSentSpamMsgsCnt uint32

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

	// helper function to send the message to the network
	sendMessage := func(msg *tangle.Message) error {
		if err := gossip.MessageProcessor().Emit(msg); err != nil {
			return err
		}

		metrics.SharedServerMetrics.SentSpamMessages.Inc()
		return nil
	}

	spammerInstance = spammer.New(
		config.NodeConfig.GetString(config.CfgSpammerMessage),
		config.NodeConfig.GetString(config.CfgSpammerIndex),
		config.NodeConfig.GetString(config.CfgSpammerIndexSemiLazy),
		urts.TipSelector.SelectSpammerTips,
		pow.Handler(),
		sendMessage,
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
		Start(nil, nil)
	}
}

// Start starts the spammer to spam with the given settings, otherwise it uses the settings from the config.
func Start(mpsRateLimit *float64, cpuMaxUsage *float64) (float64, float64, error) {
	if spammerInstance == nil {
		return 0.0, 0.0, ErrSpammerDisabled
	}

	spammerLock.Lock()
	defer spammerLock.Unlock()

	stopWithoutLocking()

	mpsRateLimitCfg := config.NodeConfig.GetFloat64(config.CfgSpammerMPSRateLimit)
	cpuMaxUsageCfg := config.NodeConfig.GetFloat64(config.CfgSpammerCPUMaxUsage)
	spammerWorkerCount := config.NodeConfig.GetInt(config.CfgSpammerWorkers)
	checkPeersConnected := node.IsSkipped(coordinator.PLUGIN)

	if mpsRateLimit != nil {
		mpsRateLimitCfg = *mpsRateLimit
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

	if spammerWorkerCount >= runtime.NumCPU() || spammerWorkerCount == 0 {
		spammerWorkerCount = runtime.NumCPU() - 1
	}
	if spammerWorkerCount < 1 {
		spammerWorkerCount = 1
	}

	startSpammerWorkers(mpsRateLimitCfg, cpuMaxUsageCfg, spammerWorkerCount, checkPeersConnected)

	return mpsRateLimitCfg, cpuMaxUsageCfg, nil
}

func startSpammerWorkers(mpsRateLimit float64, cpuMaxUsage float64, spammerWorkerCount int, checkPeersConnected bool) {

	var rateLimitChannel chan struct{} = nil
	var rateLimitAbortSignal chan struct{} = nil

	if mpsRateLimit != 0.0 {
		rateLimitChannelSize := int64(mpsRateLimit) * 2
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
			}, time.Duration(int64(float64(time.Second)/mpsRateLimit)), done)

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

					if mpsRateLimit != 0 {
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

					if checkPeersConnected && p2pplug.Manager().ConnectedCount(p2p.PeerRelationKnown) == 0 {
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

					durationGTTA, durationPOW, err := spammerInstance.DoSpam(shutdownSignal)
					if err != nil {
						continue
					}
					Events.SpamPerformed.Trigger(&spammer.SpamStats{Tipselection: float32(durationGTTA.Seconds()), ProofOfWork: float32(durationPOW.Seconds())})
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

	sentSpamMsgsCnt := metrics.SharedServerMetrics.SentSpamMessages.Load()
	new := utils.GetUint32Diff(sentSpamMsgsCnt, lastSentSpamMsgsCnt)
	lastSentSpamMsgsCnt = sentSpamMsgsCnt

	spammerAvgHeap.Add(uint64(new))

	timeDiff := time.Since(spammerStartTime)
	if timeDiff > 60*time.Second {
		// Only filter over one minute maximum
		timeDiff = 60 * time.Second
	}

	// trigger events for outside listeners
	Events.AvgSpamMetricsUpdated.Trigger(&spammer.AvgSpamMetrics{
		NewMessages:              new,
		AverageMessagesPerSecond: spammerAvgHeap.GetAveragePerSecond(timeDiff),
	})
}
