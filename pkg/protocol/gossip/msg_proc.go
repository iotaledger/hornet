package gossip

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/workerpool"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/pow"
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

func MessageProcessedCaller(handler interface{}, params ...interface{}) {
	handler.(func(msg *storage.Message, requests Requests, proto *Protocol))(params[0].(*storage.Message), params[1].(Requests), params[2].(*Protocol))
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

// MessageProcessorEvents are the events fired by the MessageProcessor.
type MessageProcessorEvents struct {
	// Fired when a message was fully processed.
	MessageProcessed *events.Event
	// Fired when a message is meant to be broadcasted.
	BroadcastMessage *events.Event
}

// The Options for the MessageProcessor.
type Options struct {
	MinPoWScore       float64
	NetworkID         uint64
	ProtocolVersion   byte
	BelowMaxDepth     milestone.Index
	WorkUnitCacheOpts *profile.CacheOpts
}

// MessageProcessor processes submitted messages in parallel and fires appropriate completion events.
type MessageProcessor struct {
	// used to access the node storage.
	storage *storage.Storage
	// used to determine the sync status of the node.
	syncManager *syncmanager.SyncManager
	// contains requests for needed messages.
	requestQueue RequestQueue
	// used to manage connected peers.
	peeringManager *p2p.Manager
	// shared server metrics instance.
	serverMetrics *metrics.ServerMetrics
	// Deserialization parameters including byte costs
	deSeriParas *iotago.DeSerializationParameters
	// holds the message processor options.
	opts Options

	// events of the message processor.
	Events *MessageProcessorEvents
	// cache that holds processed incomming messages.
	workUnits *objectstorage.ObjectStorage
	// worker pool for incomming messages.
	wp *workerpool.WorkerPool

	// mutex to secure the shutdown flag.
	shutdownMutex syncutils.RWMutex
	// indicates that the message processor was shut down.
	shutdown bool
}

// NewMessageProcessor creates a new processor which parses messages.
func NewMessageProcessor(
	dbStorage *storage.Storage,
	syncManager *syncmanager.SyncManager,
	requestQueue RequestQueue,
	peeringManager *p2p.Manager,
	serverMetrics *metrics.ServerMetrics,
	deSeriParas *iotago.DeSerializationParameters,
	opts *Options) (*MessageProcessor, error) {

	proc := &MessageProcessor{
		storage:        dbStorage,
		syncManager:    syncManager,
		requestQueue:   requestQueue,
		peeringManager: peeringManager,
		serverMetrics:  serverMetrics,
		deSeriParas:    deSeriParas,
		opts:           *opts,
		Events: &MessageProcessorEvents{
			MessageProcessed: events.NewEvent(MessageProcessedCaller),
			BroadcastMessage: events.NewEvent(BroadcastCaller),
		},
	}

	wuCacheOpts := opts.WorkUnitCacheOpts

	cacheTime, err := time.ParseDuration(wuCacheOpts.CacheTime)
	if err != nil {
		return nil, err
	}

	leakDetectionMaxConsumerHoldTime, err := time.ParseDuration(wuCacheOpts.LeakDetectionOptions.MaxConsumerHoldTime)
	if err != nil {
		return nil, err
	}

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

	return proc, nil
}

// Run runs the processor and blocks until the shutdown signal is triggered.
func (proc *MessageProcessor) Run(ctx context.Context) {
	proc.wp.Start()
	<-ctx.Done()
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

// Process submits the given message to the processor for processing.
func (proc *MessageProcessor) Process(p *Protocol, msgType message.Type, data []byte) {
	proc.wp.Submit(p, msgType, data)
}

// Emit triggers MessageProcessed and BroadcastMessage events for the given message.
// All messages passed to this function must be checked with "DeSeriModePerformValidation" before.
// We also check if the parents are solid and not BMD before we broadcast the message, otherwise
// this message would be seen as invalid gossip by other peers.
func (proc *MessageProcessor) Emit(msg *storage.Message) error {

	if msg.ProtocolVersion() != proc.opts.ProtocolVersion {
		return fmt.Errorf("msg has invalid protocol version %d instead of %d", msg.ProtocolVersion(), proc.opts.ProtocolVersion)
	}

	essence := msg.TransactionEssence()
	if essence != nil && essence.NetworkID != proc.opts.NetworkID {
		return fmt.Errorf("transaction contained in msg has invalid network ID %d instead of %d", essence.NetworkID, proc.opts.NetworkID)
	}

	score := pow.Score(msg.Data())
	if score < proc.opts.MinPoWScore {
		return fmt.Errorf("msg has insufficient PoW score %0.2f", score)
	}

	cmi := proc.syncManager.ConfirmedMilestoneIndex()

	checkParentFunc := func(messageID hornet.MessageID) error {
		cachedMsgMeta := proc.storage.CachedMessageMetadataOrNil(messageID) // meta +1
		if cachedMsgMeta == nil {
			// parent not found
			entryPointIndex, exists, err := proc.storage.SolidEntryPointsIndex(messageID)
			if err != nil {
				return err
			}
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
		defer cachedMsgMeta.Release(true) // meta -1

		if !cachedMsgMeta.Metadata().IsSolid() {
			// if the parent is not solid, the message itself can't be solid
			return ErrMessageNotSolid
		}

		// we pass a background context here to not prevent emitting messages at shutdown (COO etc).
		_, ocri, err := dag.ConeRootIndexes(context.Background(), proc.storage, cachedMsgMeta.Retain(), cmi) // meta pass +1
		if err != nil {
			return err
		}

		if (cmi - ocri) > proc.opts.BelowMaxDepth {
			// the parent is below max depth
			return ErrMessageBelowMaxDepth
		}

		return nil
	}

	for _, parentMsgID := range msg.Parents() {
		err := checkParentFunc(parentMsgID)
		if err != nil {
			return err
		}
	}

	proc.Events.MessageProcessed.Trigger(msg, (Requests)(nil), (*Protocol)(nil))
	proc.Events.BroadcastMessage.Trigger(&Broadcast{MsgData: msg.Data()})

	return nil
}

// WorkUnitsSize returns the size of WorkUnits currently cached.
func (proc *MessageProcessor) WorkUnitsSize() int {
	return proc.workUnits.GetSize()
}

// gets a CachedWorkUnit or creates a new one if it not existent.
func (proc *MessageProcessor) workUnitFor(receivedTxBytes []byte) (cachedWorkUnit *CachedWorkUnit, newlyAdded bool) {
	return &CachedWorkUnit{
		proc.workUnits.ComputeIfAbsent(receivedTxBytes, func(_ []byte) objectstorage.StorableObject { // cachedWorkUnit +1
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
		_ = proc.peeringManager.DisconnectPeer(p.PeerID, errors.WithMessage(err, "processMilestoneRequest failed"))
		return
	}

	// peers can request the latest milestone we know
	if msIndex == LatestMilestoneRequestIndex {
		msIndex = proc.syncManager.LatestMilestoneIndex()
	}

	cachedMsgMilestone := proc.storage.MilestoneCachedMessageOrNil(msIndex) // message +1
	if cachedMsgMilestone == nil {
		// can't reply if we don't have the wanted milestone
		return
	}
	defer cachedMsgMilestone.Release(true) // message -1

	requestedData, err := cachedMsgMilestone.Message().Message().Serialize(serializer.DeSeriModeNoValidation, iotago.ZeroRentParas)
	if err != nil {
		// can't reply if serialization fails
		return
	}

	msg, err := NewMessageMsg(requestedData)
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

	cachedMsg := proc.storage.CachedMessageOrNil(hornet.MessageIDFromSlice(data)) // message +1
	if cachedMsg == nil {
		// can't reply if we don't have the requested message
		return
	}
	defer cachedMsg.Release(true) // message -1

	requestedData, err := cachedMsg.Message().Message().Serialize(serializer.DeSeriModeNoValidation, iotago.ZeroRentParas)
	if err != nil {
		// can't reply if serialization fails
		return
	}

	msg, err := NewMessageMsg(requestedData)
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
	workUnit.addReceivedFrom(p)
	proc.processWorkUnit(workUnit, p)
}

// tries to process the WorkUnit by first checking in what state it is.
// if the WorkUnit is invalid (because the underlying message is invalid), the given peer is punished.
// if the WorkUnit is already completed, and the message was requested, this function emits a MessageProcessed event.
// it is safe to call this function for the same WorkUnit multiple times.
func (proc *MessageProcessor) processWorkUnit(wu *WorkUnit, p *Protocol) {

	processRequests := func(wu *WorkUnit, msg *storage.Message, isMilestonePayload bool) Requests {

		var requests Requests

		// mark the message as received
		request := proc.requestQueue.Received(msg.MessageID())
		if request != nil {
			requests = append(requests, request)
		}

		if isMilestonePayload {
			// mark the milestone as received
			msRequest := proc.requestQueue.Received(milestone.Index(msg.Milestone().Index))
			if msRequest != nil {
				requests = append(requests, msRequest)
			}
		}

		wu.requested = requests.HasRequest()
		return requests
	}

	wu.processingLock.Lock()

	switch {
	case wu.Is(Hashing):
		wu.processingLock.Unlock()
		return

	case wu.Is(Invalid):
		wu.processingLock.Unlock()

		proc.serverMetrics.InvalidMessages.Inc()

		// drop the connection to the peer
		_ = proc.peeringManager.DisconnectPeer(p.PeerID, errors.New("peer sent an invalid message"))
		return

	case wu.Is(Hashed):
		wu.processingLock.Unlock()

		// we need to check for requests here again because there is a race condition
		// between processing received messages and enqueuing requests.
		requests := processRequests(wu, wu.msg, wu.msg.IsMilestone())
		if wu.requested {
			wu.requested = true
			proc.Events.MessageProcessed.Trigger(wu.msg, requests, p)
		}

		if proc.storage.ContainsMessage(wu.msg.MessageID()) {
			proc.serverMetrics.KnownMessages.Inc()
			p.Metrics.KnownMessages.Inc()
		}

		return
	}

	wu.UpdateState(Hashing)
	wu.processingLock.Unlock()

	// build HORNET representation of the message
	msg, err := storage.MessageFromBytes(wu.receivedMsgBytes, serializer.DeSeriModePerformValidation, proc.deSeriParas)
	if err != nil {
		wu.UpdateState(Invalid)
		wu.punish(errors.WithMessagef(err, "peer sent an invalid message"))
		return
	}

	// check the network ID of the message
	if msg.ProtocolVersion() != proc.opts.ProtocolVersion {
		wu.UpdateState(Invalid)
		wu.punish(errors.New("peer sent a message with an invalid protocol version"))
		return
	}

	essence := msg.TransactionEssence()
	if essence != nil && essence.NetworkID != proc.opts.NetworkID {
		wu.UpdateState(Invalid)
		wu.punish(errors.New("peer sent a message containing a transaction with an invalid network ID"))
		return
	}

	isMilestonePayload := msg.IsMilestone()

	// mark the message as received
	requests := processRequests(wu, msg, isMilestonePayload)

	// validate PoW score
	if !wu.requested && pow.Score(wu.receivedMsgBytes) < proc.opts.MinPoWScore {
		wu.UpdateState(Invalid)
		wu.punish(errors.New("peer sent a message with insufficient PoW score"))
		return
	}

	// safe to set the msg here, because it is protected by the state "Hashing"
	wu.msg = msg
	wu.UpdateState(Hashed)

	// increase the known message count for all other peers
	wu.increaseKnownTxCount(p)

	// do not process gossip if we are not in sync.
	// we ignore all received messages if we didn't request them and it's not a milestone.
	// otherwise these messages would get evicted from the cache, and it's heavier to load them
	// from the storage than to request them again.
	if !wu.requested && !proc.syncManager.IsNodeAlmostSynced() && !isMilestonePayload {
		return
	}

	proc.Events.MessageProcessed.Trigger(msg, requests, p)
}

func (proc *MessageProcessor) Broadcast(cachedMsgMeta *storage.CachedMetadata) {
	proc.shutdownMutex.RLock()
	defer proc.shutdownMutex.RUnlock()
	defer cachedMsgMeta.Release(true) // meta -1

	if proc.shutdown {
		// do not broadcast if the message processor was shut down
		return
	}

	if !proc.syncManager.IsNodeSyncedWithinBelowMaxDepth() {
		// no need to broadcast messages if the node is not sync within "below max depth"
		return
	}

	// we pass a background context here to not prevent broadcasting messages at shutdown (COO etc).
	_, ocri, err := dag.ConeRootIndexes(context.Background(), proc.storage, cachedMsgMeta.Retain(), proc.syncManager.ConfirmedMilestoneIndex()) // meta pass +1
	if err != nil {
		return
	}

	if (proc.syncManager.LatestMilestoneIndex() - ocri) > proc.opts.BelowMaxDepth {
		// the solid message was below max depth in relation to the latest milestone index, do not broadcast
		return
	}

	cachedMsg := proc.storage.CachedMessageOrNil(cachedMsgMeta.Metadata().MessageID()) // message +1
	if cachedMsg == nil {
		return
	}
	defer cachedMsg.Release(true) // message -1

	cachedWorkUnit, _ := proc.workUnitFor(cachedMsg.Message().Data()) // workUnit +1
	defer cachedWorkUnit.Release(true)                                // workUnit -1
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
