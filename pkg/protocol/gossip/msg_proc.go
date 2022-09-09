package gossip

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/objectstorage"
	"github.com/iotaledger/hive.go/core/protocol/message"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hive.go/core/workerpool"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/p2p"
	"github.com/iotaledger/hornet/v2/pkg/profile"
	"github.com/iotaledger/hornet/v2/pkg/protocol"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/builder"
	"github.com/iotaledger/iota.go/v3/pow"
)

const (
	WorkerQueueSize = 50000
	WorkerCount     = 64
)

var (
	ErrBlockNotSolid      = errors.New("block is not solid")
	ErrBlockBelowMaxDepth = errors.New("block is below max depth")
)

func BlockProcessedCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(block *storage.Block, requests Requests, proto *Protocol))(params[0].(*storage.Block), params[1].(Requests), params[2].(*Protocol))
}

// Broadcast defines a data which should be broadcasted.
type Broadcast struct {
	// The data to broadcast.
	Data []byte
	// The IDs of the peers to exclude from broadcasting.
	ExcludePeers map[peer.ID]struct{}
}

func BroadcastCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(b *Broadcast))(params[0].(*Broadcast))
}

// MessageProcessorEvents are the events fired by the MessageProcessor.
type MessageProcessorEvents struct {
	// Fired when a block was fully processed.
	BlockProcessed *events.Event
	// Fired when a block is meant to be broadcasted.
	BroadcastBlock *events.Event
}

// The Options for the MessageProcessor.
type Options struct {
	WorkUnitCacheOpts *profile.CacheOpts
}

// MessageProcessor processes submitted messages in parallel and fires appropriate completion events.
type MessageProcessor struct {
	// used to access the node storage.
	storage *storage.Storage
	// used to determine the sync status of the node.
	syncManager *syncmanager.SyncManager
	// contains requests for needed blocks.
	requestQueue RequestQueue
	// used to manage connected peers.
	peeringManager *p2p.Manager
	// shared server metrics instance.
	serverMetrics *metrics.ServerMetrics
	// protocol manager
	protocolManager *protocol.Manager
	// holds the message processor options.
	opts Options

	// events of the block processor.
	Events *MessageProcessorEvents
	// cache that holds processed incoming messages.
	workUnits *objectstorage.ObjectStorage
	// worker pool for incoming messages.
	wp *workerpool.WorkerPool

	// mutex to secure the shutdown flag.
	shutdownMutex syncutils.RWMutex
	// indicates that the message processor was shut down.
	shutdown bool
}

// NewMessageProcessor creates a new processor which processes messages.
func NewMessageProcessor(
	dbStorage *storage.Storage,
	syncManager *syncmanager.SyncManager,
	requestQueue RequestQueue,
	peeringManager *p2p.Manager,
	serverMetrics *metrics.ServerMetrics,
	protocolManager *protocol.Manager,
	opts *Options) (*MessageProcessor, error) {

	proc := &MessageProcessor{
		storage:         dbStorage,
		syncManager:     syncManager,
		requestQueue:    requestQueue,
		peeringManager:  peeringManager,
		serverMetrics:   serverMetrics,
		protocolManager: protocolManager,
		opts:            *opts,
		Events: &MessageProcessorEvents{
			BlockProcessed: events.NewEvent(BlockProcessedCaller),
			BroadcastBlock: events.NewEvent(BroadcastCaller),
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
		defer task.Return(nil)

		p, ok := task.Param(0).(*Protocol)
		if !ok {
			panic(fmt.Sprintf("invalid type: expected *Protocol, got %T", task.Param(0)))
		}

		data, ok := task.Param(2).([]byte)
		if !ok {
			panic(fmt.Sprintf("invalid type: expected []byte, got %T", task.Param(2)))
		}

		msgType, ok := task.Param(1).(message.Type)
		if !ok {
			panic(fmt.Sprintf("invalid type: expected message.Type, got %T", task.Param(1)))
		}

		switch msgType {
		case MessageTypeBlock:
			proc.processBlockData(p, data)
		case MessageTypeBlockRequest:
			proc.processBlockRequest(p, data)
		case MessageTypeMilestoneRequest:
			proc.processMilestoneRequest(p, data)
		}
	}, workerpool.WorkerCount(WorkerCount), workerpool.QueueSize(WorkerQueueSize))

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

// Emit triggers BlockProcessed and BroadcastBlock events for the given block.
// All blocks passed to this function must be checked with "DeSeriModePerformValidation" before.
// We also check if the parents are solid and not BMD before we broadcast the block, otherwise
// this block would be seen as invalid gossip by other peers.
func (proc *MessageProcessor) Emit(block *storage.Block) error {

	if block.ProtocolVersion() != proc.protocolManager.Current().Version {
		return fmt.Errorf("block has invalid protocol version %d instead of %d", block.ProtocolVersion(), proc.protocolManager.Current().Version)
	}

	switch block.Block().Payload.(type) {

	case *iotago.Milestone:
		// enforce milestone block nonce == 0
		if block.Block().Nonce != 0 {
			return errors.New("milestone block nonce must be zero")
		}

	default:
		// validate PoW score
		if proc.protocolManager.Current().MinPoWScore != 0 {
			score := pow.Score(block.Data())
			if score < float64(proc.protocolManager.Current().MinPoWScore) {
				return fmt.Errorf("block has insufficient PoW score %0.2f", score)
			}
		}
	}

	cmi := proc.syncManager.ConfirmedMilestoneIndex()

	checkParentFunc := func(blockID iotago.BlockID) error {
		cachedBlockMeta := proc.storage.CachedBlockMetadataOrNil(blockID) // meta +1
		if cachedBlockMeta == nil {
			// parent not found
			entryPointIndex, exists, err := proc.storage.SolidEntryPointsIndex(blockID)
			if err != nil {
				return err
			}
			if !exists {
				return ErrBlockNotSolid
			}

			if (cmi - entryPointIndex) > syncmanager.MilestoneIndexDelta(proc.protocolManager.Current().BelowMaxDepth) {
				// the parent is below max depth
				return ErrBlockBelowMaxDepth
			}

			// block is a SEP and not below max depth
			return nil
		}
		defer cachedBlockMeta.Release(true) // meta -1

		if !cachedBlockMeta.Metadata().IsSolid() {
			// if the parent is not solid, the block itself can't be solid
			return ErrBlockNotSolid
		}

		// we pass a background context here to not prevent emitting blocks at shutdown (COO etc).
		_, ocri, err := dag.ConeRootIndexes(context.Background(), proc.storage, cachedBlockMeta.Retain(), cmi) // meta pass +1
		if err != nil {
			return err
		}

		if (cmi - ocri) > syncmanager.MilestoneIndexDelta(proc.protocolManager.Current().BelowMaxDepth) {
			// the parent is below max depth
			return ErrBlockBelowMaxDepth
		}

		return nil
	}

	for _, parentBlockID := range block.Parents() {
		err := checkParentFunc(parentBlockID)
		if err != nil {
			return err
		}
	}

	proc.Events.BlockProcessed.Trigger(block, (Requests)(nil), (*Protocol)(nil))
	proc.Events.BroadcastBlock.Trigger(&Broadcast{Data: block.Data()})

	return nil
}

// WorkUnitsSize returns the size of WorkUnits currently cached.
func (proc *MessageProcessor) WorkUnitsSize() int {
	return proc.workUnits.GetSize()
}

// gets a CachedWorkUnit or creates a new one if it not existent.
func (proc *MessageProcessor) workUnitFor(receivedBlockBytes []byte) (cachedWorkUnit *CachedWorkUnit, newlyAdded bool) {
	return &CachedWorkUnit{
		proc.workUnits.ComputeIfAbsent(receivedBlockBytes, func(_ []byte) objectstorage.StorableObject { // cachedWorkUnit +1
			newlyAdded = true

			return newWorkUnit(receivedBlockBytes, proc)
		}),
	}, newlyAdded
}

// processes the given milestone request by parsing it and then replying to the peer with it.
func (proc *MessageProcessor) processMilestoneRequest(p *Protocol, data []byte) {
	msIndex, err := extractRequestedMilestoneIndex(data)
	if err != nil {
		proc.serverMetrics.InvalidRequests.Inc()

		// drop the connection to the peer
		_ = proc.peeringManager.DisconnectPeer(p.PeerID, errors.WithMessage(err, "processMilestoneRequest failed"))

		return
	}

	// peers can request the latest milestone we know
	if msIndex == latestMilestoneRequestIndex {
		msIndex = proc.syncManager.LatestMilestoneIndex()
	}

	cachedMilestone := proc.storage.CachedMilestoneByIndexOrNil(msIndex) // milestone +1
	if cachedMilestone == nil {
		// can't reply if we don't have the wanted milestone
		return
	}
	defer cachedMilestone.Release(true) // milestone -1

	milestoneBlock, err := constructMilestoneBlock(proc.protocolManager.Current(), cachedMilestone.Retain()) // milestone +1
	if err != nil {
		// can't reply if creating milestone block fails
		return
	}

	requestedData, err := milestoneBlock.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		// can't reply if serialization fails
		return
	}

	msg, err := newBlockMessage(requestedData)
	if err != nil {
		// can't reply if serialization fails
		return
	}

	p.Enqueue(msg)
}

func constructMilestoneBlock(protoParams *iotago.ProtocolParameters, cachedMilestone *storage.CachedMilestone) (*iotago.Block, error) {
	defer cachedMilestone.Release(true) // milestone -1

	// we don't need to do proof of work for milestone blocks because milestones have Nonce = 0.
	// TODO: this is enforced by TIP-???
	return builder.NewBlockBuilder().
		ProtocolVersion(protoParams.Version).
		Payload(cachedMilestone.Milestone().Milestone()).
		Parents(cachedMilestone.Milestone().Milestone().Parents).
		Build()
}

// processes the given block request by parsing it and then replying to the peer with it.
func (proc *MessageProcessor) processBlockRequest(p *Protocol, data []byte) {
	if len(data) != iotago.BlockIDLength {
		return
	}
	blockID := iotago.BlockID{}
	copy(blockID[:], data)
	cachedBlock := proc.storage.CachedBlockOrNil(blockID) // block +1
	if cachedBlock == nil {
		// can't reply if we don't have the requested block
		return
	}
	defer cachedBlock.Release(true) // block -1

	msg, err := newBlockMessage(cachedBlock.Block().Data())
	if err != nil {
		// can't reply if serialization fails
		return
	}

	p.Enqueue(msg)
}

// gets or creates a new WorkUnit for the given block data and then processes the WorkUnit.
func (proc *MessageProcessor) processBlockData(p *Protocol, data []byte) {
	cachedWorkUnit, newlyAdded := proc.workUnitFor(data) // workUnit +1

	// force release if not newly added, so the cache time is only active the first time the block is received.
	defer cachedWorkUnit.Release(!newlyAdded) // workUnit -1

	workUnit := cachedWorkUnit.WorkUnit()
	workUnit.addReceivedFrom(p)
	proc.processWorkUnit(workUnit, p)
}

// tries to process the WorkUnit by first checking in what state it is.
// if the WorkUnit is invalid (because the underlying block is invalid), the given peer is punished.
// if the WorkUnit is already completed, and the block was requested, this function emits a BlockProcessed event.
// it is safe to call this function for the same WorkUnit multiple times.
func (proc *MessageProcessor) processWorkUnit(wu *WorkUnit, p *Protocol) {

	processRequests := func(wu *WorkUnit, block *storage.Block, isMilestonePayload bool) Requests {

		requests := Requests{}

		// mark the block as received
		request := proc.requestQueue.Received(block.BlockID())
		if request != nil {
			requests = append(requests, request)
		}

		if isMilestonePayload {
			// mark the milestone as received
			msRequest := proc.requestQueue.Received(block.Milestone().Index)
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
		_ = proc.peeringManager.DisconnectPeer(p.PeerID, errors.New("peer sent an invalid block"))

		return

	case wu.Is(Hashed):
		wu.processingLock.Unlock()

		// we need to check for requests here again because there is a race condition
		// between processing received blocks and enqueuing requests.
		requests := processRequests(wu, wu.block, wu.block.IsMilestone())
		if wu.requested {
			proc.Events.BlockProcessed.Trigger(wu.block, requests, p)
		}

		if proc.storage.ContainsBlock(wu.block.BlockID()) {
			proc.serverMetrics.KnownBlocks.Inc()
			p.Metrics.KnownBlocks.Inc()
		}

		return
	}

	wu.UpdateState(Hashing)
	wu.processingLock.Unlock()

	// build HORNET representation of the block
	block, err := storage.BlockFromBytes(wu.receivedBytes, serializer.DeSeriModePerformValidation, proc.protocolManager.Current())
	if err != nil {
		wu.UpdateState(Invalid)
		wu.punish(errors.WithMessagef(err, "peer sent an invalid block"))

		return
	}

	// check the network ID of the block
	if block.ProtocolVersion() != proc.protocolManager.Current().Version {
		wu.UpdateState(Invalid)
		wu.punish(errors.New("peer sent a block with an invalid protocol version"))

		return
	}

	isMilestonePayload := block.IsMilestone()

	// mark the block as received
	requests := processRequests(wu, block, isMilestonePayload)

	if !isMilestonePayload {
		// validate PoW score
		targetScore := proc.protocolManager.Current().MinPoWScore

		if !wu.requested && targetScore != 0 && pow.Score(wu.receivedBytes) < float64(targetScore) {
			wu.UpdateState(Invalid)
			wu.punish(errors.New("peer sent a block with insufficient PoW score"))

			return
		}
	} else {
		// enforce milestone block nonce == 0
		if block.Block().Nonce != 0 {
			wu.punish(errors.New("milestone block nonce must be zero"))
		}

		// TODO: refactor data flow
	}

	// safe to set the block here, because it is protected by the state "Hashing"
	wu.block = block
	wu.UpdateState(Hashed)

	// increase the known block count for all other peers
	wu.increaseKnownTxCount(p)

	// do not process gossip if we are not in sync.
	// we ignore all received blocks if we didn't request them and it's not a milestone.
	// otherwise these blocks would get evicted from the cache, and it's heavier to load them
	// from the storage than to request them again.
	if !wu.requested && !proc.syncManager.IsNodeAlmostSynced() && !isMilestonePayload {
		return
	}

	proc.Events.BlockProcessed.Trigger(block, requests, p)
}

func (proc *MessageProcessor) Broadcast(cachedBlockMeta *storage.CachedMetadata) {
	proc.shutdownMutex.RLock()
	defer proc.shutdownMutex.RUnlock()
	defer cachedBlockMeta.Release(true) // meta -1

	if proc.shutdown {
		// do not broadcast if the block processor was shut down
		return
	}

	syncState := proc.syncManager.SyncState()

	if !syncState.NodeSyncedWithinBelowMaxDepth {
		// no need to broadcast blocks if the node is not sync within "below max depth"
		return
	}

	// we pass a background context here to not prevent broadcasting blocks at shutdown (COO etc).
	_, ocri, err := dag.ConeRootIndexes(context.Background(), proc.storage, cachedBlockMeta.Retain(), syncState.ConfirmedMilestoneIndex) // meta pass +1
	if err != nil {
		return
	}

	if (syncState.LatestMilestoneIndex - ocri) > syncmanager.MilestoneIndexDelta(proc.protocolManager.Current().BelowMaxDepth) {
		// the solid block was below max depth in relation to the latest milestone index, do not broadcast
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
		// no need to broadcast if the block was requested
		return
	}

	// if the workunit was already evicted, it may happen that
	// we send the block back to peers which already sent us the same block.
	// we should never access the "block", because it may not be set in this context.

	// broadcast the block to all peers that didn't sent it to us yet
	proc.Events.BroadcastBlock.Trigger(wu.broadcast())
}
