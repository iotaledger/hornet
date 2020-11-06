package gossip

import (
	"errors"
	"time"

	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/iota.go/pow"
	"github.com/libp2p/go-libp2p-core/peer"

	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
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
func NewMessageProcessor(tangle *tangle.Tangle, requestQueue RequestQueue, peeringService *p2p.Manager, opts *Options) *MessageProcessor {
	proc := &MessageProcessor{
		tangle:       tangle,
		ps:           peeringService,
		requestQueue: requestQueue,
		Events: MessageProcessorEvents{
			MessageProcessed: events.NewEvent(MessageProcessedCaller),
			BroadcastMessage: events.NewEvent(BroadcastCaller),
		},
		opts: *opts,
	}
	wuCacheOpts := opts.WorkUnitCacheOpts
	proc.workUnits = objectstorage.New(
		nil,
		workUnitFactory,
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
	handler.(func(msg *tangle.Message, request *Request, proto *Protocol))(params[0].(*tangle.Message), params[1].(*Request), params[2].(*Protocol))
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
	tangle       *tangle.Tangle
	Events       MessageProcessorEvents
	ps           *p2p.Manager
	wp           *workerpool.WorkerPool
	requestQueue RequestQueue
	workUnits    *objectstorage.ObjectStorage
	opts         Options
}

// The Options for the MessageProcessor.
type Options struct {
	MinPoWScore       float64
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
func (proc *MessageProcessor) Emit(msg *tangle.Message) error {

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
			return newWorkUnit(receivedTxBytes)
		}),
	}
}

// processes the given milestone request by parsing it and then replying to the peer with it.
func (proc *MessageProcessor) processMilestoneRequest(p *Protocol, data []byte) {
	msIndex, err := ExtractRequestedMilestoneIndex(data)
	if err != nil {
		metrics.SharedServerMetrics.InvalidRequests.Inc()

		// drop the connection to the peer
		_ = proc.ps.DisconnectPeer(p.PeerID)
		return
	}

	// peers can request the latest milestone we know
	if msIndex == LatestMilestoneRequestIndex {
		msIndex = proc.tangle.GetLatestMilestoneIndex()
	}

	cachedMessage := proc.tangle.GetMilestoneCachedMessageOrNil(msIndex) // message +1
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

	cachedMessage := proc.tangle.GetCachedMessageOrNil(hornet.MessageIDFromBytes(data)) // message +1
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

		metrics.SharedServerMetrics.InvalidMessages.Inc()

		// drop the connection to the peer
		_ = proc.ps.DisconnectPeer(p.PeerID)

		return
	case wu.Is(Hashed):
		wu.processingLock.Unlock()

		// emit an event to say that a message was fully processed
		if request := proc.requestQueue.Received(wu.msg.GetMessageID()); request != nil {
			proc.Events.MessageProcessed.Trigger(wu.msg, request, p)
			return
		}

		if proc.tangle.ContainsMessage(wu.msg.GetMessageID()) {
			metrics.SharedServerMetrics.KnownMessages.Inc()
			p.Metrics.KnownMessages.Inc()
			return
		}

		return
	}

	wu.UpdateState(Hashing)
	wu.processingLock.Unlock()

	// build HORNET representation of the message
	msg, err := tangle.MessageFromBytes(wu.receivedMsgBytes, iotago.DeSeriModePerformValidation)
	if err != nil {
		wu.UpdateState(Invalid)
		wu.punish(proc.ps)
		return
	}

	// mark the message as received
	request := proc.requestQueue.Received(msg.GetMessageID())

	// validate PoW score
	if request == nil && pow.Score(wu.receivedMsgBytes) < proc.opts.MinPoWScore {
		wu.UpdateState(Invalid)
		wu.punish(proc.ps)
		return
	}

	wu.dataLock.Lock()
	wu.msg = msg
	wu.dataLock.Unlock()

	if _, isInvalidMilestoneTx := invalidMilestoneHashes[string(wu.receivedTxHash)]; isInvalidMilestoneTx {
		// do not accept the invalid milestone transactions
		wu.UpdateState(Invalid)
		wu.punish()
		return
	}

	wu.UpdateState(Hashed)

	// check the existence of the message before broadcasting it
	containsTx := proc.tangle.ContainsMessage(msg.GetMessageID())

	proc.Events.MessageProcessed.Trigger(msg, request, p)

	// increase the known message count for all other peers
	wu.increaseKnownTxCount(p)

	// ToDo: broadcast on solidification
	// broadcast the message if it wasn't requested and not known yet
	if request == nil && !containsTx {
		proc.Events.BroadcastMessage.Trigger(wu.broadcast())
	}
}
