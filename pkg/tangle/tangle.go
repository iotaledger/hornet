package tangle

import (
	"context"
	"runtime"
	"sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/utils"
)

// milestoneParentsRequestFunc requests the parents of a given milestone.
// It returns True if the parents are enqeued to the request queue.
type milestoneParentsRequestFunc func(cachedMilestone *storage.CachedMilestone) bool

// requestMultipleFunc enqueues multiple request to the request queue for the given message ids
// if they aren't solid entry points and are not contained in the database already.
type requestMultipleFunc func(messageIDs hornet.MessageIDs, msIndex milestone.Index, preventDiscard ...bool) int

// requestParentsFunc enqueues requests for the parents of the given message to the request queue, if the
// given message is not a solid entry point and neither its parents are and also not in the database.
type requestParentsFunc func(cachedMsg *storage.CachedMessage, msIndex milestone.Index, preventDiscard ...bool)

type Tangle struct {
	log              *logger.Logger
	storage          *storage.Storage
	requestQueue     gossip.RequestQueue
	service          *gossip.Service
	messageProcessor *gossip.MessageProcessor
	serverMetrics    *metrics.ServerMetrics
	requester        *gossip.Requester
	shutdownCtx      context.Context
	daemon           daemon.Daemon

	receiveMsgWorkerCount int
	receiveMsgQueueSize   int
	receiveMsgWorkerPool  *workerpool.WorkerPool

	lastIncomingMsgCnt    uint32
	lastIncomingNewMsgCnt uint32
	lastOutgoingMsgCnt    uint32

	lastIncomingMPS uint32
	lastNewMPS      uint32
	lastOutgoingMPS uint32

	updateSyncedAtStartup bool

	startWaitGroup sync.WaitGroup

	messageProcessedSyncEvent   *utils.SyncEvent
	messageSolidSyncEvent       *utils.SyncEvent
	milestoneConfirmedSyncEvent *utils.SyncEvent

	milestoneSolidifierWorkerPool *workerpool.WorkerPool

	signalChanMilestoneStopSolidification     chan struct{}
	signalChanMilestoneStopSolidificationLock syncutils.Mutex

	solidifierMilestoneIndex     milestone.Index
	solidifierMilestoneIndexLock syncutils.RWMutex

	solidifierLock syncutils.RWMutex

	oldNewMsgCount        uint32
	oldReferencedMsgCount uint32

	// Index of the first milestone that was sync after node start
	firstSyncedMilestone milestone.Index

	Events *pluginEvents
}

/*

// the default options applied to the Manager.
var defaultManagerOptions = []ManagerOption{
	WithManagerReconnectInterval(30*time.Second, 1*time.Second),
}

// ManagerOptions define options for a Manager.
type ManagerOptions struct {
	// The logger to use to log events.
	Logger *logger.Logger
	// The static reconnect interval.
	ReconnectInterval time.Duration
	// The randomized jitter applied to the reconnect interval.
	ReconnectIntervalJitter time.Duration
}

// ManagerOption is a function setting a ManagerOptions option.
type ManagerOption func(opts *ManagerOptions)

// WithManagerLogger enables logging within the Manager.
func WithManagerLogger(logger *logger.Logger) ManagerOption {
	return func(opts *ManagerOptions) {
		opts.Logger = logger
	}
}

// WithManagerReconnectInterval defines the re-connect interval for peers
// to which the Manager wants to keep a connection open to.
func WithManagerReconnectInterval(interval time.Duration, jitter time.Duration) ManagerOption {
	return func(opts *ManagerOptions) {
		opts.ReconnectInterval = interval
		opts.ReconnectIntervalJitter = jitter
	}
}

// applies the given ManagerOption.
func (mo *ManagerOptions) apply(opts ...ManagerOption) {
	for _, opt := range opts {
		opt(mo)
	}
}

*/

func New(log *logger.Logger, s *storage.Storage, requestQueue gossip.RequestQueue, service *gossip.Service, messageProcessor *gossip.MessageProcessor,
	serverMetrics *metrics.ServerMetrics, shutdownCtx context.Context,
	requester *gossip.Requester, daemon daemon.Daemon, updateSyncedAtStartup bool) *Tangle {
	return &Tangle{
		log:                         log,
		storage:                     s,
		requestQueue:                requestQueue,
		service:                     service,
		messageProcessor:            messageProcessor,
		serverMetrics:               serverMetrics,
		shutdownCtx:                 shutdownCtx,
		requester:                   requester,
		daemon:                      daemon,
		receiveMsgWorkerCount:       2 * runtime.NumCPU(),
		receiveMsgQueueSize:         10000,
		messageProcessedSyncEvent:   utils.NewSyncEvent(),
		messageSolidSyncEvent:       utils.NewSyncEvent(),
		milestoneConfirmedSyncEvent: utils.NewSyncEvent(),
		Events: &pluginEvents{
			MPSMetricsUpdated:             events.NewEvent(MPSMetricsCaller),
			ReceivedNewMessage:            events.NewEvent(storage.NewMessageCaller),
			ReceivedKnownMessage:          events.NewEvent(storage.MessageCaller),
			ProcessedMessage:              events.NewEvent(storage.MessageIDCaller),
			MessageSolid:                  events.NewEvent(storage.MessageMetadataCaller),
			MessageReferenced:             events.NewEvent(storage.MessageReferencedCaller),
			ReceivedNewMilestone:          events.NewEvent(storage.MilestoneCaller),
			LatestMilestoneChanged:        events.NewEvent(storage.MilestoneCaller),
			LatestMilestoneIndexChanged:   events.NewEvent(milestone.IndexCaller),
			MilestoneConfirmed:            events.NewEvent(ConfirmedMilestoneCaller),
			SolidMilestoneChanged:         events.NewEvent(storage.MilestoneCaller),
			SolidMilestoneIndexChanged:    events.NewEvent(milestone.IndexCaller),
			SnapshotMilestoneIndexChanged: events.NewEvent(milestone.IndexCaller),
			PruningMilestoneIndexChanged:  events.NewEvent(milestone.IndexCaller),
			NewConfirmedMilestoneMetric:   events.NewEvent(NewConfirmedMilestoneMetricCaller),
			MilestoneSolidificationFailed: events.NewEvent(milestone.IndexCaller),
			NewUTXOOutput:                 events.NewEvent(UTXOOutputCaller),
			NewUTXOSpent:                  events.NewEvent(UTXOSpentCaller),
		},
	}
}

// SetUpdateSyncedAtStartup sets the flag if the isNodeSynced status should be updated at startup
func (t *Tangle) SetUpdateSyncedAtStartup(updateSyncedAtStartup bool) {
	t.updateSyncedAtStartup = updateSyncedAtStartup
}
