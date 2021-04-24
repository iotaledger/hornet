package gossip

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/workerpool"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/iotaledger/iota.go/v2/pow"
)

const (
	WorkerQueueSize = 50000
)

var (
	workerCount             = 64
	ErrInvalidTimestamp     = errors.New("invalid timestamp")
	ErrMessageNotSolid      = errors.New("msg is not solid")
	ErrMessageBelowMaxDepth = errors.New("msg is below max depth")
)

// New creates a new processor which parses messages.
func NewMessageProcessor(storage *storage.Storage, requestQueue RequestQueue, peeringService *p2p.Manager, serverMetrics *metrics.ServerMetrics, opts *Options) *MessageProcessor {
	proc := &MessageProcessor{
		storage: storage,
		Events: MessageProcessorEvents{
			MessageProcessed: events.NewEvent(MessageProcessedCaller),
			BroadcastMessage: events.NewEvent(BroadcastCaller),
		},
		ps:            peeringService,
		requestQueue:  requestQueue,
		serverMetrics: serverMetrics,
		opts:          *opts,
	}

	wuCacheOpts := opts.WorkUnitCacheOpts
	cacheTime, _ := time.ParseDuration(wuCacheOpts.CacheTime)
	leakDetectionMaxConsumerHoldTime, _ := time.ParseDuration(wuCacheOpts.LeakDetectionOptions.MaxConsumerHoldTime)

	proc.workUnits = objectstorage.New(
		nil,
		// defines the factory function for WorkUnits.
		func(key []byte, data []byte) (objectstorage.StorableObject, error) {
			return newWorkUnit(key, proc), nil
		},
		objectstorage.CacheTime(cacheTime),
		objectstorage.PersistenceEnabled(false),
		objectstorage.KeysOnly(true),
		objectstorage.StoreOnCreation(false),
		objectstorage.ReleaseExecutorWorkerCount(wuCacheOpts.ReleaseExecutorWorkerCount),
		objectstorage.LeakDetectionEnabled(wuCacheOpts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: wuCacheOpts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   leakDetectionMaxConsumerHoldTime,
			}),
	)

	proc.wp = workerpool.New(func(task workerpool.Task) {
		p := task.Param(0).(*Protocol)
		data := task.Param(2).([]byte)

		switch task.Param(1).(message.Type) {
		case MessageTypeMessage:
			proc.processMessage(p, data)
		case MessageTypeMessageRequest:
			proc.processMessageRequest(p, data)
		case MessageTypeMilestoneRequest:
			proc.processMilestoneRequest(p, data)
		}

		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(WorkerQueueSize))

	return proc
}

func MessageProcessedCaller(handler interface{}, params ...interface{}) {
	handler.(func(msg *storage.Message, request *Request, proto *Protocol))(params[0].(*storage.Message), params[1].(*Request), params[2].(*Protocol))
}

// Broadcast defines a message which should be broadcasted.
type Broadcast struct {
	// The message data to broadcast.
	MsgData []byte
	// The IDs of the peers to exclude from broadcasting.
	ExcludePeers map[peer.ID]struct{}
}

func BroadcastCaller(handler interface{}, params ...interface{}) {
	handler.(func(b *Broadcast))(params[0].(*Broadcast))
}

// MessageProcessorEventsEvents are the events fired by the MessageProcessor.
type MessageProcessorEvents struct {
	// Fired when a message was fully processed.
	MessageProcessed *events.Event
	// Fired when a message is meant to be broadcasted.
	BroadcastMessage *events.Event
}

// MessageProcessor processes submitted messages in parallel and fires appropriate completion events.
type MessageProcessor struct {
	storage       *storage.Storage
	Events        MessageProcessorEvents
	ps            *p2p.Manager
	wp            *workerpool.WorkerPool
	requestQueue  RequestQueue
	workUnits     *objectstorage.ObjectStorage
	serverMetrics *metrics.ServerMetrics
	opts          Options
	shutdownMutex syncutils.RWMutex
	shutdown      bool
}

// The Options for the MessageProcessor.
type Options struct {
	MinPoWScore       float64
	NetworkID         uint64
	BelowMaxDepth     milestone.Index
	WorkUnitCacheOpts *profile.CacheOpts
}

// Run runs the processor and blocks until the shutdown signal is triggered.
func (proc *MessageProcessor) Run(shutdownSignal <-chan struct{}) {
	proc.wp.Start()
	<-shutdownSignal
	proc.Shutdown()
}

// Shutdown signals the internal worker pool and object storage
// to shut down and sets the shutdown flag.
func (proc *MessageProcessor) Shutdown() {
	proc.shutdownMutex.Lock()
	defer proc.shutdownMutex.Unlock()

	proc.shutdown = true
	proc.wp.StopAndWait()
	proc.workUnits.Shutdown()
}

// FreeMemory copies the content of the internal maps to newly created maps.
// This is neccessary, otherwise the GC is not able to free the memory used by the old maps.
// "delete" doesn't shrink the maximum memory used by the map, since it only marks the entry as deleted.
func (proc *MessageProcessor) FreeMemory() {
	proc.workUnits.FreeMemory()
}

// Process submits the given message to the processor for processing.
func (proc *MessageProcessor) Process(p *Protocol, msgType message.Type, data []byte) {
	proc.wp.Submit(p, msgType, data)
}

// Emit triggers MessageProcessed and BroadcastMessage events for the given message.
// All messages passed to this function must be checked with "DeSeriModePerformValidation" before.
// We also check if the parents are solid and not BMD before we broadcast the message, otherwise
// this message would be seen as invalid gossip by other peers.
func (proc *MessageProcessor) Emit(msg *storage.Message) error {

	if msg.GetNetworkID() != proc.opts.NetworkID {
		return fmt.Errorf("msg has invalid network ID %d instead of %d", msg.GetNetworkID(), proc.opts.NetworkID)
	}

	score := pow.Score(msg.GetData())
	if score < proc.opts.MinPoWScore {
		return fmt.Errorf("msg has insufficient PoW score %0.2f", score)
	}

	cmi := proc.storage.GetConfirmedMilestoneIndex()

	checkParentFunc := func(parentMsgID iotago.MessageID) error {
		messageID := hornet.MessageIDFromArray(parentMsgID)
		cachedMsgMeta := proc.storage.GetCachedMessageMetadataOrNil(messageID) // meta +1
		if cachedMsgMeta == nil {
			// parent not found
			entryPointIndex, exists := proc.storage.SolidEntryPointsIndex(messageID)
			if !exists {
				return ErrMessageNotSolid
			}

			if (cmi - entryPointIndex) > proc.opts.BelowMaxDepth {
				// the parent is below max depth
				return ErrMessageBelowMaxDepth
			}

			// message is a SEP and not below max depth
			return nil
		}
		defer cachedMsgMeta.Release(true)

		if !cachedMsgMeta.GetMetadata().IsSolid() {
			// if the parent is not solid, the message itself can't be solid
			return ErrMessageNotSolid
		}

		_, ocri := dag.GetConeRootIndexes(proc.storage, cachedMsgMeta.Retain(), cmi) // meta +
		if (cmi - ocri) > proc.opts.BelowMaxDepth {
			// the parent is below max depth
			return ErrMessageBelowMaxDepth
		}

		return nil
	}

	for _, parent := range msg.GetMessage().Parents {
		err := checkParentFunc(parent)
		if err != nil {
			return err
		}
	}

	proc.Events.MessageProcessed.Trigger(msg, (*Request)(nil), (*Protocol)(nil))
	proc.Events.BroadcastMessage.Trigger(&Broadcast{MsgData: msg.GetData()})

	return nil
}

// WorkUnitSize returns the size of WorkUnits currently cached.
func (proc *MessageProcessor) WorkUnitsSize() int {
	return proc.workUnits.GetSize()
}

// gets a CachedWorkUnit or creates a new one if it not existent.
func (proc *MessageProcessor) workUnitFor(receivedTxBytes []byte) (cachedWorkUnit *CachedWorkUnit, newlyAdded bool) {
	return &CachedWorkUnit{
		proc.workUnits.ComputeIfAbsent(receivedTxBytes, func(key []byte) objectstorage.StorableObject { // cachedWorkUnit +1
			newlyAdded = true
			return newWorkUnit(receivedTxBytes, proc)
		}),
	}, newlyAdded
}

// processes the given milestone request by parsing it and then replying to the peer with it.
func (proc *MessageProcessor) processMilestoneRequest(p *Protocol, data []byte) {
	msIndex, err := ExtractRequestedMilestoneIndex(data)
	if err != nil {
		proc.serverMetrics.InvalidRequests.Inc()

		// drop the connection to the peer
		_ = proc.ps.DisconnectPeer(p.PeerID, errors.WithMessage(err, "processMilestoneRequest failed"))
		return
	}

	// peers can request the latest milestone we know
	if msIndex == LatestMilestoneRequestIndex {
		msIndex = proc.storage.GetLatestMilestoneIndex()
	}

	cachedMessage := proc.storage.GetMilestoneCachedMessageOrNil(msIndex) // message +1
	if cachedMessage == nil {
		// can't reply if we don't have the wanted milestone
		return
	}
	defer cachedMessage.Release(true) // message -1

	cachedRequestedData, err := cachedMessage.GetMessage().GetMessage().Serialize(iotago.DeSeriModeNoValidation)
	if err != nil {
		// can't reply if serialization fails
		return
	}

	msg, err := NewMessageMsg(cachedRequestedData)
	if err != nil {
		// can't reply if serialization fails
		return
	}

	p.Enqueue(msg)
}

// processes the given message request by parsing it and then replying to the peer with it.
func (proc *MessageProcessor) processMessageRequest(p *Protocol, data []byte) {
	if len(data) != iotago.MessageIDLength {
		return
	}

	cachedMessage := proc.storage.GetCachedMessageOrNil(hornet.MessageIDFromSlice(data)) // message +1
	if cachedMessage == nil {
		// can't reply if we don't have the requested message
		return
	}
	defer cachedMessage.Release(true) // message -1

	cachedRequestedData, err := cachedMessage.GetMessage().GetMessage().Serialize(iotago.DeSeriModeNoValidation)
	if err != nil {
		// can't reply if serialization fails
		return
	}

	msg, err := NewMessageMsg(cachedRequestedData)
	if err != nil {
		// can't reply if serialization fails
		return
	}

	p.Enqueue(msg)
}

// gets or creates a new WorkUnit for the given message and then processes the WorkUnit.
func (proc *MessageProcessor) processMessage(p *Protocol, data []byte) {
	cachedWorkUnit, newlyAdded := proc.workUnitFor(data) // workUnit +1

	// force release if not newly added, so the cache time is only active the first time the message is received.
	defer cachedWorkUnit.Release(!newlyAdded) // workUnit -1

	workUnit := cachedWorkUnit.WorkUnit()
	workUnit.addReceivedFrom(p, nil)
	proc.processWorkUnit(workUnit, p)
}

// tries to process the WorkUnit by first checking in what state it is.
// if the WorkUnit is invalid (because the underlying message is invalid), the given peer is punished.
// if the WorkUnit is already completed, and the message was requested, this function emits a MessageProcessed event.
// it is safe to call this function for the same WorkUnit multiple times.
func (proc *MessageProcessor) processWorkUnit(wu *WorkUnit, p *Protocol) {
	wu.processingLock.Lock()

	switch {
	case wu.Is(Hashing):
		wu.processingLock.Unlock()
		return

	case wu.Is(Invalid):
		wu.processingLock.Unlock()

		proc.serverMetrics.InvalidMessages.Inc()

		// drop the connection to the peer
		_ = proc.ps.DisconnectPeer(p.PeerID, errors.New("peer sent an invalid message"))
		return

	case wu.Is(Hashed):
		wu.processingLock.Unlock()

		// we need to check for requests here again because there is a race condition
		// between processing received messages and enqueuing requests.
		if request := proc.requestQueue.Received(wu.msg.GetMessageID()); request != nil {
			wu.requested = true
			proc.Events.MessageProcessed.Trigger(wu.msg, request, p)
		}

		// the message should be in the cache already by high chance, because the state is hashed.
		// there is no need to create disc pressure by doing a storage lookup just for these stats.
		if proc.storage.ContainsMessage(wu.msg.GetMessageID(), objectstorage.WithReadSkipStorage(true)) {
			proc.serverMetrics.KnownMessages.Inc()
			p.Metrics.KnownMessages.Inc()
		}

		return
	}

	wu.UpdateState(Hashing)
	wu.processingLock.Unlock()

	// build HORNET representation of the message
	msg, err := storage.MessageFromBytes(wu.receivedMsgBytes, iotago.DeSeriModePerformValidation)
	if err != nil {
		wu.UpdateState(Invalid)
		wu.punish(errors.WithMessagef(err, "peer sent an invalid message"))
		return
	}

	// check the network ID of the message
	if msg.GetNetworkID() != proc.opts.NetworkID {
		wu.UpdateState(Invalid)
		wu.punish(errors.New("peer sent a message with an invalid network ID"))
		return
	}

	// mark the message as received
	request := proc.requestQueue.Received(msg.GetMessageID())

	// validate PoW score
	if request == nil && pow.Score(wu.receivedMsgBytes) < proc.opts.MinPoWScore {
		wu.UpdateState(Invalid)
		wu.punish(errors.New("peer sent a message with insufficient PoW score"))
		return
	}

	// safe to set the msg here, because it is protected by the state "Hashing"
	wu.msg = msg
	wu.requested = request != nil

	wu.UpdateState(Hashed)

	// increase the known message count for all other peers
	wu.increaseKnownTxCount(p)

	// do not process gossip if we are not in sync.
	// we ignore all received messages if we didn't request them and it's not a milestone.
	// otherwise these messages would get evicted from the cache, and it's heavier to load them
	// from the storage than to request them again.
	if request == nil && !proc.storage.IsNodeAlmostSynced() && !msg.IsMilestone() {
		return
	}

	proc.Events.MessageProcessed.Trigger(msg, request, p)
}

func (proc *MessageProcessor) Broadcast(cachedMsgMeta *storage.CachedMetadata) {
	proc.shutdownMutex.RLock()
	defer proc.shutdownMutex.RUnlock()
	defer cachedMsgMeta.Release(true)

	if proc.shutdown {
		// do not broadcast if the message processor was shut down
		return
	}

	if !proc.storage.IsNodeSyncedWithinBelowMaxDepth() {
		// no need to broadcast messages if the node is not sync within "below max depth"
		return
	}

	_, ocri := dag.GetConeRootIndexes(proc.storage, cachedMsgMeta.Retain(), proc.storage.GetConfirmedMilestoneIndex())
	if (proc.storage.GetLatestMilestoneIndex() - ocri) > proc.opts.BelowMaxDepth {
		// the solid message was below max depth in relation to the latest milestone index, do not broadcast
		return
	}

	cachedMsg := proc.storage.GetCachedMessageOrNil(cachedMsgMeta.GetMetadata().GetMessageID())
	if cachedMsg == nil {
		return
	}
	defer cachedMsg.Release(true)

	cachedWorkUnit, _ := proc.workUnitFor(cachedMsg.GetMessage().GetData()) // workUnit +1
	defer cachedWorkUnit.Release(true)                                      // workUnit -1
	wu := cachedWorkUnit.WorkUnit()

	if wu.requested {
		// no need to broadcast if the message was requested
		return
	}

	// if the workunit was already evicted, it may happen that
	// we send the message back to peers which already sent us the same message.
	// we should never access the "msg", because it may not be set in this context.

	// broadcast the message to all peers that didn't sent it to us yet
	proc.Events.BroadcastMessage.Trigger(wu.broadcast())
}
