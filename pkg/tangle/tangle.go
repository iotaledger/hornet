package tangle

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/hive.go/workerpool"
)

type Tangle struct {
	log                   *logger.Logger
	storage               *storage.Storage
	requestQueue          gossip.RequestQueue
	service               *gossip.Service
	messageProcessor      *gossip.MessageProcessor
	serverMetrics         *metrics.ServerMetrics
	requester             *gossip.Requester
	receiptService        *migrator.ReceiptService
	daemon                daemon.Daemon
	shutdownCtx           context.Context
	belowMaxDepth         milestone.Index
	milestoneTimeout      time.Duration
	updateSyncedAtStartup bool

	milestoneTimeoutTicker *timeutil.Ticker

	receiveMsgWorkerCount int
	receiveMsgQueueSize   int
	receiveMsgWorkerPool  *workerpool.WorkerPool

	lastIncomingMsgCnt    uint32
	lastIncomingNewMsgCnt uint32
	lastOutgoingMsgCnt    uint32

	lastIncomingMPS uint32
	lastNewMPS      uint32
	lastOutgoingMPS uint32

	startWaitGroup sync.WaitGroup

	messageProcessedSyncEvent   *utils.SyncEvent
	messageSolidSyncEvent       *utils.SyncEvent
	milestoneConfirmedSyncEvent *utils.SyncEvent

	processValidMilestoneWorkerPool *workerpool.WorkerPool
	milestoneSolidifierWorkerPool   *workerpool.WorkerPool

	signalChanMilestoneStopSolidification     chan struct{}
	signalChanMilestoneStopSolidificationLock syncutils.Mutex

	solidifierMilestoneIndex     milestone.Index
	solidifierMilestoneIndexLock syncutils.RWMutex

	solidifierLock syncutils.RWMutex

	oldNewMsgCount        uint32
	oldReferencedMsgCount uint32

	// Index of the first milestone that was sync after node start
	firstSyncedMilestone milestone.Index

	lastConfirmedMilestoneMetricLock syncutils.RWMutex
	lastConfirmedMilestoneMetric     *ConfirmedMilestoneMetric

	Events *Events
}

func New(
	log *logger.Logger,
	s *storage.Storage,
	requestQueue gossip.RequestQueue,
	service *gossip.Service,
	messageProcessor *gossip.MessageProcessor,
	serverMetrics *metrics.ServerMetrics,
	requester *gossip.Requester,
	receiptService *migrator.ReceiptService,
	daemon daemon.Daemon,
	shutdownCtx context.Context,
	belowMaxDepth int,
	milestoneTimeout time.Duration,
	updateSyncedAtStartup bool) *Tangle {

	t := &Tangle{
		log:                   log,
		storage:               s,
		requestQueue:          requestQueue,
		service:               service,
		messageProcessor:      messageProcessor,
		serverMetrics:         serverMetrics,
		requester:             requester,
		receiptService:        receiptService,
		daemon:                daemon,
		shutdownCtx:           shutdownCtx,
		belowMaxDepth:         milestone.Index(belowMaxDepth),
		milestoneTimeout:      milestoneTimeout,
		updateSyncedAtStartup: updateSyncedAtStartup,

		receiveMsgWorkerCount:       2 * runtime.NumCPU(),
		receiveMsgQueueSize:         10000,
		messageProcessedSyncEvent:   utils.NewSyncEvent(),
		messageSolidSyncEvent:       utils.NewSyncEvent(),
		milestoneConfirmedSyncEvent: utils.NewSyncEvent(),
		Events: &Events{
			MPSMetricsUpdated:              events.NewEvent(MPSMetricsCaller),
			ReceivedNewMessage:             events.NewEvent(storage.NewMessageCaller),
			ReceivedKnownMessage:           events.NewEvent(storage.MessageCaller),
			ProcessedMessage:               events.NewEvent(storage.MessageIDCaller),
			MessageSolid:                   events.NewEvent(storage.MessageMetadataCaller),
			MessageReferenced:              events.NewEvent(storage.MessageReferencedCaller),
			ReceivedNewMilestone:           events.NewEvent(storage.MilestoneCaller),
			LatestMilestoneChanged:         events.NewEvent(storage.MilestoneCaller),
			LatestMilestoneIndexChanged:    events.NewEvent(milestone.IndexCaller),
			MilestoneConfirmed:             events.NewEvent(ConfirmedMilestoneCaller),
			ConfirmedMilestoneChanged:      events.NewEvent(storage.MilestoneCaller),
			ConfirmedMilestoneIndexChanged: events.NewEvent(milestone.IndexCaller),
			NewConfirmedMilestoneMetric:    events.NewEvent(NewConfirmedMilestoneMetricCaller),
			ConfirmationMetricsUpdated:     events.NewEvent(ConfirmationMetricsCaller),
			MilestoneSolidificationFailed:  events.NewEvent(milestone.IndexCaller),
			MilestoneTimeout:               events.NewEvent(events.VoidCaller),
			NewUTXOOutput:                  events.NewEvent(UTXOOutputCaller),
			NewUTXOSpent:                   events.NewEvent(UTXOSpentCaller),
			NewReceipt:                     events.NewEvent(ReceiptCaller),
		},
	}
	t.ResetMilestoneTimeoutTicker()
	return t
}

// SetUpdateSyncedAtStartup sets the flag if the isNodeSynced status should be updated at startup
func (t *Tangle) SetUpdateSyncedAtStartup(updateSyncedAtStartup bool) {
	t.updateSyncedAtStartup = updateSyncedAtStartup
}

// ResetMilestoneTimeoutTicker stops a running milestone timeout ticker and starts a new one.
// MilestoneTimeout event is fired if ResetMilestoneTimeoutTicker is not called within milestoneTimeout.
func (t *Tangle) ResetMilestoneTimeoutTicker() {
	if t.milestoneTimeoutTicker != nil {
		t.milestoneTimeoutTicker.Shutdown()
	}

	t.milestoneTimeoutTicker = timeutil.NewTicker(func() {
		t.Events.MilestoneTimeout.Trigger()
	}, t.milestoneTimeout)
}
