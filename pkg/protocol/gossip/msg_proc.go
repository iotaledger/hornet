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
	"github.com/iotaledger/iota.go/v3/builder"
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
	handler.(func(msg *storage.Block, requests Requests, proto *Protocol))(params[0].(*storage.Block), params[1].(Requests), params[2].(*Protocol))
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

// MessageProcessorEvents are the events fired by the BlockProcessor.
type MessageProcessorEvents struct {
	// Fired when a message was fully processed.
	BlockProcessed *events.Event
	// Fired when a message is meant to be broadcasted.
	BroadcastMessage *events.Event
}

// The Options for the BlockProcessor.
type Options struct {
	WorkUnitCacheOpts *profile.CacheOpts
}

// BlockProcessor processes submitted messages in parallel and fires appropriate completion events.
type BlockProcessor struct {
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
	// protocol parameters including byte costs
	protoParas *iotago.ProtocolParameters
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
	protoParas *iotago.ProtocolParameters,
	opts *Options) (*BlockProcessor, error) {

	proc := &BlockProcessor{
		storage:        dbStorage,
		syncManager:    syncManager,
		requestQueue:   requestQueue,
		peeringManager: peeringManager,
		serverMetrics:  serverMetrics,
		protoParas:     protoParas,
		opts:           *opts,
		Events: &MessageProcessorEvents{
			BlockProcessed:   events.NewEvent(MessageProcessedCaller),
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
		case MessageTypeBlock:
			proc.processMessage(p, data)
		case MessageTypeBlockRequest:
			proc.processMessageRequest(p, data)
		case MessageTypeMilestoneRequest:
			proc.processMilestoneRequest(p, data)
		}

		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(WorkerQueueSize))

	return proc, nil
}

// Run runs the processor and blocks until the shutdown signal is triggered.
func (proc *BlockProcessor) Run(ctx context.Context) {
	proc.wp.Start()
	<-ctx.Done()
	proc.Shutdown()
}

// Shutdown signals the internal worker pool and object storage
// to shut down and sets the shutdown flag.
func (proc *BlockProcessor) Shutdown() {
	proc.shutdownMutex.Lock()
	defer proc.shutdownMutex.Unlock()

	proc.shutdown = true
	proc.wp.StopAndWait()
	proc.workUnits.Shutdown()
}

// Process submits the given message to the processor for processing.
func (proc *BlockProcessor) Process(p *Protocol, msgType message.Type, data []byte) {
	proc.wp.Submit(p, msgType, data)
}

// Emit triggers BlockProcessed and BroadcastMessage events for the given message.
// All messages passed to this function must be checked with "DeSeriModePerformValidation" before.
// We also check if the parents are solid and not BMD before we broadcast the message, otherwise
// this message would be seen as invalid gossip by other peers.
func (proc *BlockProcessor) Emit(msg *storage.Block) error {

	if msg.ProtocolVersion() != proc.protoParas.Version {
		return fmt.Errorf("msg has invalid protocol version %d instead of %d", msg.ProtocolVersion(), proc.protoParas.Version)
	}

	switch msg.Block().Payload.(type) {

	case *iotago.Milestone:
		// enforce milestone msg nonce == 0
		if msg.Block().Nonce != 0 {
			return errors.New("milestone msg nonce must be zero")
		}

	default:
		// validate PoW score
		score := pow.Score(msg.Data())
		if score < proc.protoParas.MinPoWScore {
			return fmt.Errorf("msg has insufficient PoW score %0.2f", score)
		}
	}

	cmi := proc.syncManager.ConfirmedMilestoneIndex()

	checkParentFunc := func(blockID hornet.BlockID) error {
		cachedBlockMeta := proc.storage.CachedBlockMetadataOrNil(blockID) // meta +1
		if cachedBlockMeta == nil {
			// parent not found
			entryPointIndex, exists, err := proc.storage.SolidEntryPointsIndex(blockID)
			if err != nil {
				return err
			}
			if !exists {
				return ErrMessageNotSolid
			}

			if (cmi - entryPointIndex) > milestone.Index(proc.protoParas.BelowMaxDepth) {
				// the parent is below max depth
				return ErrMessageBelowMaxDepth
			}

			// message is a SEP and not below max depth
			return nil
		}
		defer cachedBlockMeta.Release(true) // meta -1

		if !cachedBlockMeta.Metadata().IsSolid() {
			// if the parent is not solid, the message itself can't be solid
			return ErrMessageNotSolid
		}

		// we pass a background context here to not prevent emitting messages at shutdown (COO etc).
		_, ocri, err := dag.ConeRootIndexes(context.Background(), proc.storage, cachedBlockMeta.Retain(), cmi) // meta pass +1
		if err != nil {
			return err
		}

		if (cmi - ocri) > milestone.Index(proc.protoParas.BelowMaxDepth) {
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

	proc.Events.BlockProcessed.Trigger(msg, (Requests)(nil), (*Protocol)(nil))
	proc.Events.BroadcastMessage.Trigger(&Broadcast{MsgData: msg.Data()})

	return nil
}

// WorkUnitsSize returns the size of WorkUnits currently cached.
func (proc *BlockProcessor) WorkUnitsSize() int {
	return proc.workUnits.GetSize()
}

// gets a CachedWorkUnit or creates a new one if it not existent.
func (proc *BlockProcessor) workUnitFor(receivedTxBytes []byte) (cachedWorkUnit *CachedWorkUnit, newlyAdded bool) {
	return &CachedWorkUnit{
		proc.workUnits.ComputeIfAbsent(receivedTxBytes, func(_ []byte) objectstorage.StorableObject { // cachedWorkUnit +1
			newlyAdded = true
			return newWorkUnit(receivedTxBytes, proc)
		}),
	}, newlyAdded
}

// processes the given milestone request by parsing it and then replying to the peer with it.
func (proc *BlockProcessor) processMilestoneRequest(p *Protocol, data []byte) {
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

	cachedMilestone := proc.storage.CachedMilestoneByIndexOrNil(msIndex) // milestone +1
	if cachedMilestone == nil {
		// can't reply if we don't have the wanted milestone
		return
	}
	defer cachedMilestone.Release(true) // milestone -1

	milestoneMsg, err := constructMilestoneMessage(proc.protoParas, cachedMilestone.Retain()) // milestone +1
	if err != nil {
		// can't reply if creating milestone message fails
		return
	}

	requestedData, err := milestoneMsg.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		// can't reply if serialization fails
		return
	}

	msg, err := NewBlockMessage(requestedData)
	if err != nil {
		// can't reply if serialization fails
		return
	}

	p.Enqueue(msg)
}

// TODO: this is a workaround, we need to create a different channel for milestone payloads in STING instead.
func constructMilestoneMessage(protoParas *iotago.ProtocolParameters, cachedMilestone *storage.CachedMilestone) (*iotago.Block, error) {
	defer cachedMilestone.Release(true) // milestone -1

	// we don't need to do proof of work for milestone messages because milestones have Nonce = 0.
	// TODO: this is enforced by TIP-???
	return builder.NewBlockBuilder(protoParas.Version).
		Payload(cachedMilestone.Milestone().Milestone()).
		ParentsBlockIDs(cachedMilestone.Milestone().Milestone().Parents).
		Build()
}

// processes the given message request by parsing it and then replying to the peer with it.
func (proc *BlockProcessor) processMessageRequest(p *Protocol, data []byte) {
	if len(data) != iotago.BlockIDLength {
		return
	}

	cachedBlock := proc.storage.CachedBlockOrNil(hornet.BlockIDFromSlice(data)) // block +1
	if cachedBlock == nil {
		// can't reply if we don't have the requested message
		return
	}
	defer cachedBlock.Release(true) // block -1

	requestedData, err := cachedBlock.Block().Block().Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		// can't reply if serialization fails
		return
	}

	msg, err := NewBlockMessage(requestedData)
	if err != nil {
		// can't reply if serialization fails
		return
	}

	p.Enqueue(msg)
}

// gets or creates a new WorkUnit for the given message and then processes the WorkUnit.
func (proc *BlockProcessor) processMessage(p *Protocol, data []byte) {
	cachedWorkUnit, newlyAdded := proc.workUnitFor(data) // workUnit +1

	// force release if not newly added, so the cache time is only active the first time the message is received.
	defer cachedWorkUnit.Release(!newlyAdded) // workUnit -1

	workUnit := cachedWorkUnit.WorkUnit()
	workUnit.addReceivedFrom(p)
	proc.processWorkUnit(workUnit, p)
}

// tries to process the WorkUnit by first checking in what state it is.
// if the WorkUnit is invalid (because the underlying message is invalid), the given peer is punished.
// if the WorkUnit is already completed, and the message was requested, this function emits a BlockProcessed event.
// it is safe to call this function for the same WorkUnit multiple times.
func (proc *BlockProcessor) processWorkUnit(wu *WorkUnit, p *Protocol) {

	processRequests := func(wu *WorkUnit, msg *storage.Block, isMilestonePayload bool) Requests {

		var requests Requests

		// mark the message as received
		request := proc.requestQueue.Received(msg.BlockID())
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

		proc.serverMetrics.InvalidBlocks.Inc()

		// drop the connection to the peer
		_ = proc.peeringManager.DisconnectPeer(p.PeerID, errors.New("peer sent an invalid message"))
		return

	case wu.Is(Hashed):
		wu.processingLock.Unlock()

		// we need to check for requests here again because there is a race condition
		// between processing received messages and enqueuing requests.
		requests := processRequests(wu, wu.msg, wu.msg.IsMilestone())
		if wu.requested {
			proc.Events.BlockProcessed.Trigger(wu.msg, requests, p)
		}

		if proc.storage.ContainsBlock(wu.msg.BlockID()) {
			proc.serverMetrics.KnownBlocks.Inc()
			p.Metrics.KnownBlocks.Inc()
		}

		return
	}

	wu.UpdateState(Hashing)
	wu.processingLock.Unlock()

	// build HORNET representation of the message
	msg, err := storage.BlockFromBytes(wu.receivedMsgBytes, serializer.DeSeriModePerformValidation, proc.protoParas)
	if err != nil {
		wu.UpdateState(Invalid)
		wu.punish(errors.WithMessagef(err, "peer sent an invalid message"))
		return
	}

	// check the network ID of the message
	if msg.ProtocolVersion() != proc.protoParas.Version {
		wu.UpdateState(Invalid)
		wu.punish(errors.New("peer sent a message with an invalid protocol version"))
		return
	}

	isMilestonePayload := msg.IsMilestone()

	// mark the message as received
	requests := processRequests(wu, msg, isMilestonePayload)

	if !isMilestonePayload {
		// validate PoW score
		if !wu.requested && pow.Score(wu.receivedMsgBytes) < proc.protoParas.MinPoWScore {
			wu.UpdateState(Invalid)
			wu.punish(errors.New("peer sent a msg with insufficient PoW score"))
			return
		}
	} else {
		// enforce milestone msg nonce == 0
		if msg.Block().Nonce != 0 {
			wu.punish(errors.New("milestone msg nonce must be zero"))
		}

		// TODO: refactor data flow
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

	proc.Events.BlockProcessed.Trigger(msg, requests, p)
}

func (proc *BlockProcessor) Broadcast(cachedBlockMeta *storage.CachedMetadata) {
	proc.shutdownMutex.RLock()
	defer proc.shutdownMutex.RUnlock()
	defer cachedBlockMeta.Release(true) // meta -1

	if proc.shutdown {
		// do not broadcast if the message processor was shut down
		return
	}

	if !proc.syncManager.IsNodeSyncedWithinBelowMaxDepth() {
		// no need to broadcast messages if the node is not sync within "below max depth"
		return
	}

	// we pass a background context here to not prevent broadcasting messages at shutdown (COO etc).
	_, ocri, err := dag.ConeRootIndexes(context.Background(), proc.storage, cachedBlockMeta.Retain(), proc.syncManager.ConfirmedMilestoneIndex()) // meta pass +1
	if err != nil {
		return
	}

	if (proc.syncManager.LatestMilestoneIndex() - ocri) > milestone.Index(proc.protoParas.BelowMaxDepth) {
		// the solid message was below max depth in relation to the latest milestone index, do not broadcast
		return
	}

	cachedBlock := proc.storage.CachedBlockOrNil(cachedBlockMeta.Metadata().BlockID()) // block +1
	if cachedBlock == nil {
		return
	}
	defer cachedBlock.Release(true) // block -1

	cachedWorkUnit, _ := proc.workUnitFor(cachedBlock.Block().Data()) // workUnit +1
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
