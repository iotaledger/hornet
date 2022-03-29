package tangle

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/milestonemanager"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
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
	// the logger used to log events.
	*utils.WrappedLogger

	// used to access the global daemon.
	daemon daemon.Daemon
	// context that is done when the node is shutting down.
	shutdownCtx context.Context
	// used to access the node storage.
	storage *storage.Storage
	// used to determine the sync status of the node.
	syncManager *syncmanager.SyncManager
	// milestoneManager is used to retrieve, verify and store milestones.
	milestoneManager *milestonemanager.MilestoneManager
	// contains requests for needed messages.
	requestQueue gossip.RequestQueue
	// used to access gossip gossipService.
	gossipService *gossip.Service
	// used to parses and emit new messages.
	messageProcessor *gossip.MessageProcessor
	// shared server metrics instance.
	serverMetrics *metrics.ServerMetrics
	// used to request messages from peers.
	requester *gossip.Requester
	// used to persist and validate batches of receipts.
	receiptService *migrator.ReceiptService
	// belowMaxDepth is the maximum allowed delta value between OCRI of
	// a given message in relation to the current CMI before it gets lazy.
	belowMaxDepth         milestone.Index
	milestoneTimeout      time.Duration
	updateSyncedAtStartup bool

	milestoneTimeoutTicker *timeutil.Ticker

	futureConeSolidifier *FutureConeSolidifier

	receiveMsgWorkerPool  *workerpool.WorkerPool
	receiveMsgWorkerCount int
	receiveMsgQueueSize   int

	futureConeSolidifierWorkerPool  *workerpool.WorkerPool
	futureConeSolidifierWorkerCount int
	futureConeSolidifierQueueSize   int

	processValidMilestoneWorkerPool  *workerpool.WorkerPool
	processValidMilestoneWorkerCount int
	processValidMilestoneQueueSize   int

	milestoneSolidifierWorkerPool  *workerpool.WorkerPool
	milestoneSolidifierWorkerCount int
	milestoneSolidifierQueueSize   int

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

	milestoneSolidificationCtxLock    syncutils.Mutex
	milestoneSolidificationCancelFunc context.CancelFunc

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
	daemon daemon.Daemon,
	shutdownCtx context.Context,
	dbStorage *storage.Storage,
	syncManager *syncmanager.SyncManager,
	milestoneManager *milestonemanager.MilestoneManager,
	requestQueue gossip.RequestQueue,
	gossipService *gossip.Service,
	messageProcessor *gossip.MessageProcessor,
	serverMetrics *metrics.ServerMetrics,
	requester *gossip.Requester,
	receiptService *migrator.ReceiptService,
	belowMaxDepth int,
	milestoneTimeout time.Duration,
	updateSyncedAtStartup bool) *Tangle {

	t := &Tangle{
		WrappedLogger:         utils.NewWrappedLogger(log),
		daemon:                daemon,
		shutdownCtx:           shutdownCtx,
		storage:               dbStorage,
		syncManager:           syncManager,
		milestoneManager:      milestoneManager,
		requestQueue:          requestQueue,
		gossipService:         gossipService,
		messageProcessor:      messageProcessor,
		serverMetrics:         serverMetrics,
		requester:             requester,
		receiptService:        receiptService,
		belowMaxDepth:         milestone.Index(belowMaxDepth),
		milestoneTimeout:      milestoneTimeout,
		updateSyncedAtStartup: updateSyncedAtStartup,

		milestoneTimeoutTicker:           nil,
		futureConeSolidifier:             nil,
		receiveMsgWorkerCount:            2 * runtime.NumCPU(),
		receiveMsgQueueSize:              10000,
		futureConeSolidifierWorkerCount:  1, // must be one, so there are no parallel solidifications of the same cone
		futureConeSolidifierQueueSize:    10000,
		processValidMilestoneWorkerCount: 1, // must be one, so there are no parallel validations
		processValidMilestoneQueueSize:   1000,
		milestoneSolidifierWorkerCount:   2, // must be two, so a new request can abort another, in case it is an older milestone
		milestoneSolidifierQueueSize:     2,
		messageProcessedSyncEvent:        utils.NewSyncEvent(),
		messageSolidSyncEvent:            utils.NewSyncEvent(),
		milestoneConfirmedSyncEvent:      utils.NewSyncEvent(),
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
			LedgerUpdated:                  events.NewEvent(LedgerUpdatedCaller),
			NewUTXOOutput:                  events.NewEvent(UTXOOutputCaller),
			NewUTXOSpent:                   events.NewEvent(UTXOSpentCaller),
			NewReceipt:                     events.NewEvent(ReceiptCaller),
		},
	}
	t.futureConeSolidifier = NewFutureConeSolidifier(t.storage, t.markMessageAsSolid)
	t.ResetMilestoneTimeoutTicker()
	return t
}

// SetUpdateSyncedAtStartup sets the flag if the isNodeSynced status should be updated at startup
func (t *Tangle) SetUpdateSyncedAtStartup(updateSyncedAtStartup bool) {
	t.updateSyncedAtStartup = updateSyncedAtStartup
}

// ResetMilestoneTimeoutTicker stops a running milestone timeout ticker and starts a new one.
// MilestoneTimeout event is fired periodically if ResetMilestoneTimeoutTicker is not called within milestoneTimeout.
func (t *Tangle) ResetMilestoneTimeoutTicker() {
	if t.milestoneTimeoutTicker != nil {
		t.milestoneTimeoutTicker.Shutdown()
	}

	t.milestoneTimeoutTicker = timeutil.NewTicker(func() {
		t.Events.MilestoneTimeout.Trigger()
	}, t.milestoneTimeout)
}

// StopMilestoneTimeoutTicker stops the milestone timeout ticker.
func (t *Tangle) StopMilestoneTimeoutTicker() {
	if t.milestoneTimeoutTicker != nil {
		t.milestoneTimeoutTicker.Shutdown()
	}
}
