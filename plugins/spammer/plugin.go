package spammer

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/spammer"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/plugins/coordinator"
	"github.com/gohornet/hornet/plugins/urts"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.Disabled,
		Pluggable: node.Pluggable{
			Name:      "Spammer",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *node.Plugin
	log    *logger.Logger
	deps   dependencies

	spammerInstance *spammer.Spammer
	spammerLock     syncutils.RWMutex

	spammerStartTime    time.Time
	spammerAvgHeap      *utils.TimeHeap
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
	MessageProcessor *gossip.MessageProcessor
	Storage          *storage.Storage
	ServerMetrics    *metrics.ServerMetrics
	PowHandler       *pow.Handler
	Manager          *p2p.Manager
	TipSelector      *tipselect.TipSelector
	NodeConfig       *configuration.Configuration `name:"nodeConfig"`
	NetworkID        uint64                       `name:"networkId"`
	Echo             *echo.Echo
}

func configure() {
	log = logger.NewLogger(Plugin.Name)

	// do not enable the spammer if URTS is disabled
	if Plugin.Node.IsSkipped(urts.Plugin) {
		Plugin.Status = node.Disabled
		return
	}

	g := deps.Echo.Group(RouteSpammer)

	g.GET("/", func(c echo.Context) error {
		resp, err := handleSpammerCommand(c)
		if err != nil {
			return err
		}
		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	spammerAvgHeap = utils.NewTimeHeap()

	// start the CPU usage updater
	cpuUsageUpdater()

	// helper function to send the message to the network
	sendMessage := func(msg *storage.Message) error {
		if err := deps.MessageProcessor.Emit(msg); err != nil {
			return err
		}

		deps.ServerMetrics.SentSpamMessages.Inc()
		return nil
	}

	mpsRateLimitRunning = deps.NodeConfig.Float64(CfgSpammerMPSRateLimit)
	cpuMaxUsageRunning = deps.NodeConfig.Float64(CfgSpammerCPUMaxUsage)
	spammerWorkersRunning = deps.NodeConfig.Int(CfgSpammerWorkers)
	if spammerWorkersRunning == 0 {
		spammerWorkersRunning = runtime.NumCPU() - 1
	}
	isRunning = false

	spammerInstance = spammer.New(
		deps.NetworkID,
		deps.NodeConfig.String(CfgSpammerMessage),
		deps.NodeConfig.String(CfgSpammerIndex),
		deps.NodeConfig.String(CfgSpammerIndexSemiLazy),
		deps.TipSelector.SelectSpammerTips,
		deps.PowHandler,
		sendMessage,
		deps.ServerMetrics,
	)
}

func run() {
	// do not enable the spammer if URTS is disabled
	if Plugin.Node.IsSkipped(urts.Plugin) {
		return
	}

	// create a background worker that "measures" the spammer averages values every second
	Plugin.Daemon().BackgroundWorker("Spammer Metrics Updater", func(shutdownSignal <-chan struct{}) {
		timeutil.NewTicker(measureSpammerMetrics, 1*time.Second, shutdownSignal)
	}, shutdown.PrioritySpammer)

	// automatically start the spammer on node startup if the flag is set
	if deps.NodeConfig.Bool(CfgSpammerAutostart) {
		start(nil, nil, nil)
	}
}

// start starts the spammer to spam with the given settings, otherwise it uses the settings from the config.
func start(mpsRateLimit *float64, cpuMaxUsage *float64, spammerWorkers *int) error {
	if spammerInstance == nil {
		return ErrSpammerDisabled
	}

	spammerLock.Lock()
	defer spammerLock.Unlock()

	stopWithoutLocking()

	mpsRateLimitCfg := deps.NodeConfig.Float64(CfgSpammerMPSRateLimit)
	cpuMaxUsageCfg := deps.NodeConfig.Float64(CfgSpammerCPUMaxUsage)
	spammerWorkerCount := deps.NodeConfig.Int(CfgSpammerWorkers)
	checkPeersConnected := Plugin.Node.IsSkipped(coordinator.Plugin)

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

	return nil
}

func startSpammerWorkers(mpsRateLimit float64, cpuMaxUsage float64, spammerWorkerCount int, checkPeersConnected bool) {
	mpsRateLimitRunning = mpsRateLimit
	cpuMaxUsageRunning = cpuMaxUsage
	spammerWorkersRunning = spammerWorkerCount
	isRunning = true

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
		Plugin.Daemon().BackgroundWorker("Spammer rate limit channel", func(shutdownSignal <-chan struct{}) {
			spammerWaitGroup.Add(1)
			done := make(chan struct{})
			currentProcessID := processID.Load()

			timeutil.NewTicker(func() {

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
		Plugin.Daemon().BackgroundWorker(fmt.Sprintf("Spammer_%d", i), func(shutdownSignal <-chan struct{}) {
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

					if !deps.Storage.IsNodeSyncedWithThreshold() {
						time.Sleep(time.Second)
						continue
					}

					if checkPeersConnected && deps.Manager.ConnectedCount(p2p.PeerRelationKnown) == 0 {
						time.Sleep(time.Second)
						continue
					}

					if err := waitForLowerCPUUsage(cpuMaxUsage, shutdownSignal); err != nil {
						if err != common.ErrOperationAborted {
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

	sentSpamMsgsCnt := deps.ServerMetrics.SentSpamMessages.Load()
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
