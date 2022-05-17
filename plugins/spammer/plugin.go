package spammer

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/atomic"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/daemon"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/spammer"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/restapi"
	"github.com/gohornet/hornet/plugins/urts"
	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/datastructure/timeheap"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/math"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/timeutil"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	Plugin = &app.Plugin{
		Status: app.StatusDisabled,
		Component: &app.Component{
			Name:      "Spammer",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *app.Plugin
	deps   dependencies

	spammerInstance *spammer.Spammer
	spammerLock     syncutils.RWMutex

	spammerStartTime    time.Time
	spammerAvgHeap      *timeheap.TimeHeap
	lastSentSpamMsgsCnt uint32

	isRunning             bool
	mpsRateLimitRunning   float64
	cpuMaxUsageRunning    float64
	spammerWorkersRunning int

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
	ErrSpammerDisabled = errors.New("spammer plugin disabled")
)

type dependencies struct {
	dig.In
	MessageProcessor   *gossip.MessageProcessor
	SyncManager        *syncmanager.SyncManager
	ServerMetrics      *metrics.ServerMetrics
	PoWHandler         *pow.Handler
	PeeringManager     *p2p.Manager
	TipSelector        *tipselect.TipSelector `optional:"true"`
	ProtocolParameters *iotago.ProtocolParameters
	RestPluginManager  *restapi.RestPluginManager `optional:"true"`
}

func configure() error {
	// check if RestAPI plugin is disabled
	if Plugin.App.IsPluginSkipped(restapi.Plugin) {
		Plugin.LogPanic("RestAPI plugin needs to be enabled to use the Spammer plugin")
	}

	// check if URTS plugin is disabled
	if Plugin.App.IsPluginSkipped(urts.Plugin) {
		Plugin.LogPanic("URTS plugin needs to be enabled to use the Spammer plugin")
	}

	routeGroup := deps.RestPluginManager.AddPlugin("spammer/v1")
	setupRoutes(routeGroup)

	spammerAvgHeap = timeheap.NewTimeHeap()

	// start the CPU usage updater
	cpuUsageUpdater()

	// helper function to send the message to the network
	sendMessage := func(msg *storage.Block) error {
		if err := deps.MessageProcessor.Emit(msg); err != nil {
			return err
		}

		deps.ServerMetrics.SentSpamBlocks.Inc()
		return nil
	}

	mpsRateLimitRunning = ParamsSpammer.MPSRateLimit
	cpuMaxUsageRunning = ParamsSpammer.CPUMaxUsage
	spammerWorkersRunning = ParamsSpammer.Workers
	if spammerWorkersRunning == 0 {
		spammerWorkersRunning = runtime.NumCPU() - 1
	}
	isRunning = false

	spammerInstance = spammer.New(
		deps.ProtocolParameters,
		ParamsSpammer.Message,
		ParamsSpammer.Tag,
		ParamsSpammer.TagSemiLazy,
		deps.TipSelector.SelectSpammerTips,
		deps.PoWHandler,
		sendMessage,
		deps.ServerMetrics,
	)
	return nil
}

func run() error {
	// do not enable the spammer if URTS is disabled
	if Plugin.App.IsPluginSkipped(urts.Plugin) {
		return nil
	}

	// create a background worker that "measures" the spammer averages values every second
	if err := Plugin.Daemon().BackgroundWorker("Spammer Metrics Updater", func(ctx context.Context) {
		ticker := timeutil.NewTicker(measureSpammerMetrics, 1*time.Second, ctx)
		ticker.WaitForGracefulShutdown()
	}, daemon.PrioritySpammer); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}

	// automatically start the spammer on node startup if the flag is set
	if ParamsSpammer.Autostart {
		_ = start(nil, nil, nil)
	}

	return nil
}

// start starts the spammer to spam with the given settings, otherwise it uses the settings from the config.
func start(mpsRateLimit *float64, cpuMaxUsage *float64, spammerWorkers *int) error {
	if spammerInstance == nil {
		return ErrSpammerDisabled
	}

	spammerLock.Lock()
	defer spammerLock.Unlock()

	stopWithoutLocking()

	mpsRateLimitCfg := ParamsSpammer.MPSRateLimit
	cpuMaxUsageCfg := ParamsSpammer.CPUMaxUsage
	spammerWorkerCount := ParamsSpammer.Workers

	if mpsRateLimit != nil {
		mpsRateLimitCfg = *mpsRateLimit
	}

	if cpuMaxUsage != nil {
		cpuMaxUsageCfg = *cpuMaxUsage
	}

	if spammerWorkers != nil {
		spammerWorkerCount = *spammerWorkers
	}

	if cpuMaxUsageCfg > 0.0 && runtime.GOOS == "windows" {
		Plugin.LogWarn("spammer.cpuMaxUsage not supported on Windows. will be deactivated")
		cpuMaxUsageCfg = 0.0
	}

	if cpuMaxUsageCfg > 0.0 && runtime.NumCPU() == 1 {
		Plugin.LogWarn("spammer.cpuMaxUsage not supported on single core machines. will be deactivated")
		cpuMaxUsageCfg = 0.0
	}

	if spammerWorkerCount >= runtime.NumCPU() || spammerWorkerCount == 0 {
		spammerWorkerCount = runtime.NumCPU() - 1
	}
	if spammerWorkerCount < 1 {
		spammerWorkerCount = 1
	}

	startSpammerWorkers(mpsRateLimitCfg, cpuMaxUsageCfg, spammerWorkerCount)

	return nil
}

func startSpammerWorkers(mpsRateLimit float64, cpuMaxUsage float64, spammerWorkerCount int) {
	mpsRateLimitRunning = mpsRateLimit
	cpuMaxUsageRunning = cpuMaxUsage
	spammerWorkersRunning = spammerWorkerCount
	isRunning = true

	var rateLimitChannel chan struct{} = nil
	var rateLimitAbortSignal chan struct{} = nil

	if mpsRateLimit != 0.0 {
		rateLimitChannel = make(chan struct{}, spammerWorkerCount*2)
		rateLimitAbortSignal = make(chan struct{})

		// create a background worker that fills rateLimitChannel every second
		if err := Plugin.Daemon().BackgroundWorker("Spammer rate limit channel", func(ctx context.Context) {
			spammerWaitGroup.Add(1)
			defer spammerWaitGroup.Done()

			currentProcessID := processID.Load()
			interval := time.Duration(int64(float64(time.Second) / mpsRateLimit))
			timeout := interval * 2
			if timeout < time.Second {
				timeout = time.Second
			}

			var lastDuration time.Duration
		rateLimitLoop:
			for {
				timeStart := time.Now()

				rateLimitCtx, rateLimitCtxCancel := context.WithTimeout(context.Background(), timeout)

				if currentProcessID != processID.Load() {
					close(rateLimitAbortSignal)
					rateLimitCtxCancel()
					break rateLimitLoop
				}

				// measure the last interval error and multiply by two to compensate (to reach target BPS)
				lastIntervalError := (lastDuration - interval) * 2.0
				if lastIntervalError < 0 {
					lastIntervalError = 0
				}
				time.Sleep(interval - lastIntervalError)

				select {
				case <-ctx.Done():
					// received shutdown signal
					close(rateLimitAbortSignal)
					rateLimitCtxCancel()
					break rateLimitLoop

				case rateLimitChannel <- struct{}{}:
					// wait until a worker is free

				case <-rateLimitCtx.Done():
					// timeout if the channel is not free in time
					// maybe the consumer was shut down
				}

				rateLimitCtxCancel()
				lastDuration = time.Since(timeStart)
			}

		}, daemon.PrioritySpammer); err != nil {
			Plugin.LogWarnf("failed to start worker: %s", err)
		}
	}

	spammerCnt := atomic.NewInt32(0)
	for i := 0; i < spammerWorkerCount; i++ {
		if err := Plugin.Daemon().BackgroundWorker(fmt.Sprintf("Spammer_%d", i), func(ctx context.Context) {
			spammerWaitGroup.Add(1)
			defer spammerWaitGroup.Done()

			spammerIndex := spammerCnt.Inc()
			currentProcessID := processID.Load()

			Plugin.LogInfof("Starting Spammer %d... done", spammerIndex)

		spammerLoop:
			for {
				select {
				case <-ctx.Done():
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
						case <-ctx.Done():
							break spammerLoop
						case <-rateLimitChannel:
						}
					}

					if !deps.SyncManager.IsNodeAlmostSynced() {
						time.Sleep(time.Second)
						continue
					}

					if deps.PeeringManager.ConnectedCount() == 0 {
						time.Sleep(time.Second)
						continue
					}

					if err := waitForLowerCPUUsage(ctx, cpuMaxUsage); err != nil {
						if !errors.Is(err, common.ErrOperationAborted) {
							Plugin.LogWarn(err)
						}
						continue
					}

					if spammerStartTime.IsZero() {
						// set the start time for the metrics
						spammerStartTime = time.Now()
					}

					durationGTTA, durationPOW, err := spammerInstance.DoSpam(ctx)
					if err != nil {
						continue
					}
					Events.SpamPerformed.Trigger(&spammer.SpamStats{Tipselection: float32(durationGTTA.Seconds()), ProofOfWork: float32(durationPOW.Seconds())})
				}
			}

			Plugin.LogInfof("Stopping Spammer %d...", spammerIndex)
			Plugin.LogInfof("Stopping Spammer %d... done", spammerIndex)
		}, daemon.PrioritySpammer); err != nil {
			Plugin.LogWarnf("failed to start worker: %s", err)
		}
	}
}

// stop stops the spammer.
func stop() error {
	if spammerInstance == nil {
		return ErrSpammerDisabled
	}

	spammerLock.Lock()
	defer spammerLock.Unlock()

	stopWithoutLocking()

	isRunning = false

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

	sentSpamMsgsCnt := deps.ServerMetrics.SentSpamBlocks.Load()
	newMessagesCnt := math.Uint32Diff(sentSpamMsgsCnt, lastSentSpamMsgsCnt)
	lastSentSpamMsgsCnt = sentSpamMsgsCnt

	spammerAvgHeap.Add(uint64(newMessagesCnt))

	timeDiff := time.Since(spammerStartTime)
	if timeDiff > 60*time.Second {
		// Only filter over one minute maximum
		timeDiff = 60 * time.Second
	}

	// trigger events for outside listeners
	Events.AvgSpamMetricsUpdated.Trigger(&spammer.AvgSpamMetrics{
		NewBlocks:              newMessagesCnt,
		AverageBlocksPerSecond: spammerAvgHeap.AveragePerSecond(timeDiff),
	})
}
