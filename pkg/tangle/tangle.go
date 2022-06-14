package tangle

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/hornet/pkg/metrics"
	"github.com/iotaledger/hornet/pkg/model/migrator"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/milestonemanager"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/pkg/protocol/gossip"
	iotago "github.com/iotaledger/iota.go/v3"
)

type Tangle struct {
	// the logger used to log events.
	*logger.WrappedLogger

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
	// the protocol parameters
	protoParas *iotago.ProtocolParameters

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
	futureConeSolidifierQueueSize   int

	processValidMilestoneWorkerPool  *workerpool.WorkerPool
	processValidMilestoneWorkerCount int
	processValidMilestoneQueueSize   int

	milestoneSolidifierWorkerPool  *workerpool.WorkerPool
	milestoneSolidifierWorkerCount int
	milestoneSolidifierQueueSize   int

	lastIncomingBlocksCount    uint32
	lastIncomingNewBlocksCount uint32
	lastOutgoingBlocksCount    uint32

	lastIncomingBPS uint32
	lastNewBPS      uint32
	lastOutgoingBPS uint32

	startWaitGroup sync.WaitGroup

	blockProcessedSyncEvent     *events.SyncEvent
	blockSolidSyncEvent         *events.SyncEvent
	milestoneConfirmedSyncEvent *events.SyncEvent

	milestoneSolidificationCtxLock    syncutils.Mutex
	milestoneSolidificationCancelFunc context.CancelFunc

	solidifierMilestoneIndex     milestone.Index
	solidifierMilestoneIndexLock syncutils.RWMutex

	solidifierLock syncutils.RWMutex

	oldNewBlocksCount        uint32
	oldReferencedBlocksCount uint32

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
	protoParas *iotago.ProtocolParameters,
	milestoneTimeout time.Duration,
	whiteFlagParentsSolidTimeout time.Duration,
	updateSyncedAtStartup bool) *Tangle {

	t := &Tangle{
		WrappedLogger:                logger.NewWrappedLogger(log),
		daemon:                       daemon,
		shutdownCtx:                  shutdownCtx,
		storage:                      dbStorage,
		syncManager:                  syncManager,
		milestoneManager:             milestoneManager,
		requestQueue:                 requestQueue,
		gossipService:                gossipService,
		messageProcessor:             messageProcessor,
		serverMetrics:                serverMetrics,
		requester:                    requester,
		receiptService:               receiptService,
		protoParas:                   protoParas,
		milestoneTimeout:             milestoneTimeout,
		whiteFlagParentsSolidTimeout: whiteFlagParentsSolidTimeout,
		updateSyncedAtStartup:        updateSyncedAtStartup,

		milestoneTimeoutTicker:           nil,
		futureConeSolidifier:             nil,
		receiveBlockWorkerCount:          2 * runtime.NumCPU(),
		receiveBlockQueueSize:            10000,
		futureConeSolidifierWorkerCount:  1, // must be one, so there are no parallel solidifications of the same cone
		futureConeSolidifierQueueSize:    10000,
		processValidMilestoneWorkerCount: 1, // must be one, so there are no parallel validations
		processValidMilestoneQueueSize:   1000,
		milestoneSolidifierWorkerCount:   2, // must be two, so a new request can abort another, in case it is an older milestone
		milestoneSolidifierQueueSize:     2,
		blockProcessedSyncEvent:          events.NewSyncEvent(),
		blockSolidSyncEvent:              events.NewSyncEvent(),
		milestoneConfirmedSyncEvent:      events.NewSyncEvent(),
		Events: &Events{
			BPSMetricsUpdated:              events.NewEvent(BPSMetricsCaller),
			ReceivedNewBlock:               events.NewEvent(storage.NewBlockCaller),
			ReceivedKnownBlock:             events.NewEvent(storage.BlockCaller),
			ProcessedBlock:                 events.NewEvent(storage.BlockIDCaller),
			BlockSolid:                     events.NewEvent(storage.BlockMetadataCaller),
			BlockReferenced:                events.NewEvent(storage.BlockReferencedCaller),
			ReceivedNewMilestoneBlock:      events.NewEvent(storage.BlockIDCaller),
			LatestMilestoneChanged:         events.NewEvent(storage.MilestoneCaller),
			LatestMilestoneIndexChanged:    events.NewEvent(milestone.IndexCaller),
			MilestoneConfirmed:             events.NewEvent(ConfirmedMilestoneCaller),
			ConfirmedMilestoneChanged:      events.NewEvent(storage.MilestoneCaller),
			ConfirmedMilestoneIndexChanged: events.NewEvent(milestone.IndexCaller),
			NewConfirmedMilestoneMetric:    events.NewEvent(NewConfirmedMilestoneMetricCaller),
			ConfirmationMetricsUpdated:     events.NewEvent(ConfirmationMetricsCaller),
			ReferencedBlocksCountUpdated:   events.NewEvent(ReferencedBlocksCountUpdatedCaller),
			MilestoneSolidificationFailed:  events.NewEvent(milestone.IndexCaller),
			MilestoneTimeout:               events.NewEvent(events.VoidCaller),
			LedgerUpdated:                  events.NewEvent(LedgerUpdatedCaller),
			TreasuryMutated:                events.NewEvent(TreasuryMutationCaller),
			NewReceipt:                     events.NewEvent(ReceiptCaller),
		},
	}
	t.futureConeSolidifier = NewFutureConeSolidifier(t.storage, t.markBlockAsSolid)
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
