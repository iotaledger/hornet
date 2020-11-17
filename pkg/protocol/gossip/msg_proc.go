package gossip

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/workerpool"

	iotago "github.com/iotaledger/iota.go"
	"github.com/iotaledger/iota.go/pow"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/profile"
)

const (
	WorkerQueueSize = 50000
)

var (
	workerCount         = 64
	ErrInvalidTimestamp = errors.New("invalid timestamp")
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
	proc.workUnits = objectstorage.New(
		nil,
		// defines the factory function for WorkUnits.
		func(key []byte, data []byte) (objectstorage.StorableObject, error) {
			return newWorkUnit(key, serverMetrics), nil
		},
		objectstorage.CacheTime(time.Duration(wuCacheOpts.CacheTimeMs)),
		objectstorage.PersistenceEnabled(false),
		objectstorage.KeysOnly(true),
		objectstorage.StoreOnCreation(false),
		objectstorage.LeakDetectionEnabled(wuCacheOpts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: wuCacheOpts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(wuCacheOpts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
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
}

// The Options for the MessageProcessor.
type Options struct {
	MinPoWScore       float64
	NetworkID         uint64
	WorkUnitCacheOpts *profile.CacheOpts
}

// Run runs the processor and blocks until the shutdown signal is triggered.
func (proc *MessageProcessor) Run(shutdownSignal <-chan struct{}) {
	proc.wp.Start()
	<-shutdownSignal
	proc.wp.StopAndWait()
}

// Process submits the given message to the processor for processing.
func (proc *MessageProcessor) Process(p *Protocol, msgType message.Type, data []byte) {
	proc.wp.Submit(p, msgType, data)
}

// Emit triggers MessageProcessed and BroadcastMessage events for the given message.
func (proc *MessageProcessor) Emit(msg *storage.Message) error {

	if msg.GetNetworkID() != proc.opts.NetworkID {
		return fmt.Errorf("msg has invalid network ID %d instead of %d", msg.GetNetworkID(), proc.opts.NetworkID)
	}

	score, err := msg.GetMessage().POW()
	if err != nil {
		return err
	}

	if score < proc.opts.MinPoWScore {
		return fmt.Errorf("msg has insufficient PoW score %0.2f", score)
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
func (proc *MessageProcessor) workUnitFor(receivedTxBytes []byte) *CachedWorkUnit {
	return &CachedWorkUnit{
		proc.workUnits.ComputeIfAbsent(receivedTxBytes, func(key []byte) objectstorage.StorableObject { // cachedWorkUnit +1
			return newWorkUnit(receivedTxBytes, proc.serverMetrics)
		}),
	}
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
	if len(data) != 32 {
		return
	}

	cachedMessage := proc.storage.GetCachedMessageOrNil(hornet.MessageIDFromBytes(data)) // message +1
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
	cachedWorkUnit := proc.workUnitFor(data) // workUnit +1
	defer cachedWorkUnit.Release()           // workUnit -1
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

		// emit an event to say that a message was fully processed
		if request := proc.requestQueue.Received(wu.msg.GetMessageID()); request != nil {
			proc.Events.MessageProcessed.Trigger(wu.msg, request, p)
			return
		}

		if proc.storage.ContainsMessage(wu.msg.GetMessageID()) {
			proc.serverMetrics.KnownMessages.Inc()
			p.Metrics.KnownMessages.Inc()
			return
		}

		return
	}

	wu.UpdateState(Hashing)
	wu.processingLock.Unlock()

	// build HORNET representation of the message
	msg, err := storage.MessageFromBytes(wu.receivedMsgBytes, iotago.DeSeriModePerformValidation)
	if err != nil {
		wu.UpdateState(Invalid)
		wu.punish(proc.ps, errors.WithMessagef(err, "peer sent an invalid message"))
		return
	}

	// check the network ID of the message
	if msg.GetNetworkID() != proc.opts.NetworkID {
		wu.UpdateState(Invalid)
		wu.punish(proc.ps, errors.New("peer sent a message with an invalid network ID"))
		return
	}

	// mark the message as received
	request := proc.requestQueue.Received(msg.GetMessageID())

	// validate PoW score
	if request == nil && pow.Score(wu.receivedMsgBytes) < proc.opts.MinPoWScore {
		wu.UpdateState(Invalid)
		wu.punish(proc.ps, errors.New("peer sent a message with insufficient PoW score"))
		return
	}

	wu.dataLock.Lock()
	wu.msg = msg
	wu.dataLock.Unlock()

	wu.UpdateState(Hashed)

	// check the existence of the message before broadcasting it
	containsTx := proc.storage.ContainsMessage(msg.GetMessageID())

	proc.Events.MessageProcessed.Trigger(msg, request, p)

	// increase the known message count for all other peers
	wu.increaseKnownTxCount(p)

	// ToDo: broadcast on solidification
	// broadcast the message if it wasn't requested and not known yet
	if request == nil && !containsTx {
		proc.Events.BroadcastMessage.Trigger(wu.broadcast())
	}
}
