package tangle

import (
	"context"
	"runtime"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/runtime/syncutils"
	"github.com/iotaledger/hive.go/runtime/timeutil"
	"github.com/iotaledger/hive.go/runtime/valuenotifier"
	"github.com/iotaledger/hive.go/runtime/workerpool"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/migrator"
	"github.com/iotaledger/hornet/v2/pkg/model/milestonemanager"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/protocol"
	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
	iotago "github.com/iotaledger/iota.go/v3"
)

type Tangle struct {
	// the logger used to log events.
	*logger.WrappedLogger

	// context that is done when the node is shutting down.
	shutdownCtx context.Context
	// used to access the global daemon.
	daemon daemon.Daemon
	// used to access the node storage.
	storage *storage.Storage
	// used to determine the sync status of the node.
	syncManager *syncmanager.SyncManager
	// milestoneManager is used to retrieve, verify and store milestones.
	milestoneManager *milestonemanager.MilestoneManager
	// contains requests for needed blocks.
	requestQueue gossip.RequestQueue
	// used to access gossip gossipService.
	gossipService *gossip.Service
	// used to parses and emit new blocks.
	messageProcessor *gossip.MessageProcessor
	// shared server metrics instance.
	serverMetrics *metrics.ServerMetrics
	// used to request blocks from peers.
	requester *gossip.Requester
	// used to persist and validate batches of receipts.
	receiptService *migrator.ReceiptService
	// the protocol manager
	protocolManager *protocol.Manager

	milestoneTimeout             time.Duration
	whiteFlagParentsSolidTimeout time.Duration
	updateSyncedAtStartup        bool

	milestoneTimeoutTicker *timeutil.Ticker

	futureConeSolidifier *FutureConeSolidifier

	receiveBlockWorkerPool  *workerpool.WorkerPool
	receiveBlockWorkerCount int
	receiveBlockQueueSize   int

	futureConeSolidifierWorkerPool  *workerpool.WorkerPool
	futureConeSolidifierWorkerCount int

	processValidMilestoneWorkerPool  *workerpool.WorkerPool
	processValidMilestoneWorkerCount int

	milestoneSolidifierWorkerPool  *workerpool.WorkerPool
	milestoneSolidifierWorkerCount int

	lastIncomingBlocksCount    uint32
	lastIncomingNewBlocksCount uint32
	lastOutgoingBlocksCount    uint32

	lastIncomingBPS uint32
	lastNewBPS      uint32
	lastOutgoingBPS uint32

	startWaitGroup sync.WaitGroup

	blockProcessedNotifier *valuenotifier.Notifier[iotago.BlockID]
	blockSolidNotifier     *valuenotifier.Notifier[iotago.BlockID]

	milestoneSolidificationCtxLock    syncutils.Mutex
	milestoneSolidificationCancelFunc context.CancelFunc

	solidifierMilestoneIndex     iotago.MilestoneIndex
	solidifierMilestoneIndexLock syncutils.RWMutex

	solidifierLock syncutils.RWMutex

	oldNewBlocksCount        uint32
	oldReferencedBlocksCount uint32

	// Index of the first milestone that was sync after node start
	firstSyncedMilestone iotago.MilestoneIndex
	// Indicates that the node solidified more milestones than "below max depth" after becoming sync
	resyncPhaseDone *atomic.Bool

	lastConfirmedMilestoneMetricLock syncutils.RWMutex
	lastConfirmedMilestoneMetric     *ConfirmedMilestoneMetric

	Events *Events
}

func New(
	shutdownCtx context.Context,
	daemon daemon.Daemon,
	log *logger.Logger,
	dbStorage *storage.Storage,
	syncManager *syncmanager.SyncManager,
	milestoneManager *milestonemanager.MilestoneManager,
	requestQueue gossip.RequestQueue,
	gossipService *gossip.Service,
	messageProcessor *gossip.MessageProcessor,
	serverMetrics *metrics.ServerMetrics,
	requester *gossip.Requester,
	receiptService *migrator.ReceiptService,
	protocolManager *protocol.Manager,
	milestoneTimeout time.Duration,
	whiteFlagParentsSolidTimeout time.Duration,
	updateSyncedAtStartup bool) *Tangle {

	t := &Tangle{
		WrappedLogger:                logger.NewWrappedLogger(log),
		shutdownCtx:                  shutdownCtx,
		daemon:                       daemon,
		storage:                      dbStorage,
		syncManager:                  syncManager,
		milestoneManager:             milestoneManager,
		requestQueue:                 requestQueue,
		gossipService:                gossipService,
		messageProcessor:             messageProcessor,
		serverMetrics:                serverMetrics,
		requester:                    requester,
		receiptService:               receiptService,
		protocolManager:              protocolManager,
		milestoneTimeout:             milestoneTimeout,
		whiteFlagParentsSolidTimeout: whiteFlagParentsSolidTimeout,
		updateSyncedAtStartup:        updateSyncedAtStartup,

		milestoneTimeoutTicker:           nil,
		futureConeSolidifier:             nil,
		receiveBlockWorkerCount:          2 * runtime.NumCPU(),
		receiveBlockQueueSize:            10000,
		futureConeSolidifierWorkerCount:  1, // must be one, so there are no parallel solidifications of the same cone
		processValidMilestoneWorkerCount: 1, // must be one, so there are no parallel validations
		milestoneSolidifierWorkerCount:   2, // must be two, so a new request can abort another, in case it is an older milestone
		blockProcessedNotifier:           valuenotifier.New[iotago.BlockID](),
		blockSolidNotifier:               valuenotifier.New[iotago.BlockID](),
		resyncPhaseDone:                  atomic.NewBool(false),
		Events:                           newEvents(),
	}
	t.futureConeSolidifier = NewFutureConeSolidifier(t.storage, t.markBlockAsSolid)
	t.ResetMilestoneTimeoutTicker()

	return t
}

// SetUpdateSyncedAtStartup sets the flag if the isNodeSynced status should be updated at startup.
func (t *Tangle) SetUpdateSyncedAtStartup(updateSyncedAtStartup bool) {
	t.updateSyncedAtStartup = updateSyncedAtStartup
}

// ResetMilestoneTimeoutTicker stops a running milestone timeout ticker and starts a new one.
// MilestoneTimeout event is fired periodically if ResetMilestoneTimeoutTicker is not called within milestoneTimeout.
func (t *Tangle) ResetMilestoneTimeoutTicker() {
	if t.milestoneTimeoutTicker != nil {
		t.milestoneTimeoutTicker.Shutdown()
		t.milestoneTimeoutTicker.WaitForGracefulShutdown()
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
