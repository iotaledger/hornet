package processor

import (
	"errors"
	"time"

	"github.com/iotaledger/hive.go/batchhasher"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/math"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/compressed"
	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/peering"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/gohornet/hornet/pkg/protocol/bqueue"
	"github.com/gohornet/hornet/pkg/protocol/message"
	"github.com/gohornet/hornet/pkg/protocol/rqueue"
	"github.com/gohornet/hornet/pkg/protocol/sting"
)

const (
	WorkerQueueSize = 50000
)

var (
	workerCount         = batchhasher.CURLP81.GetBatchSize() * batchhasher.CURLP81.GetWorkerCount()
	ErrInvalidTimestamp = errors.New("invalid timestamp")
)

// New creates a new processor which parses messages.
func New(requestQueue rqueue.Queue, peerManager *peering.Manager, opts *Options) *Processor {
	proc := &Processor{
		pm:           peerManager,
		requestQueue: requestQueue,
		Events: Events{
			TransactionProcessed: events.NewEvent(TransactionProcessedCaller),
			BroadcastTransaction: events.NewEvent(BroadcastCaller),
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
		p := task.Param(0).(*peer.Peer)
		data := task.Param(2).([]byte)

		switch task.Param(1).(message.Type) {
		case sting.MessageTypeTransaction:
			proc.processTransaction(p, data)
		case sting.MessageTypeTransactionRequest:
			proc.processTransactionRequest(p, data)
		case sting.MessageTypeMilestoneRequest:
			proc.processMilestoneRequest(p, data)
		}

		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(WorkerQueueSize))

	return proc
}

func TransactionProcessedCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx *hornet.Transaction, request *rqueue.Request, p *peer.Peer))(params[0].(*hornet.Transaction), params[1].(*rqueue.Request), params[2].(*peer.Peer))
}

func BroadcastCaller(handler interface{}, params ...interface{}) {
	handler.(func(b *bqueue.Broadcast))(params[0].(*bqueue.Broadcast))
}

// Events are the events fired by the Processor.
type Events struct {
	// Fired when a transaction was fully processed.
	TransactionProcessed *events.Event
	// Fired when a transaction is meant to be broadcasted.
	BroadcastTransaction *events.Event
}

// Processor processes submitted messages in parallel and fires appropriate completion events.
type Processor struct {
	Events       Events
	pm           *peering.Manager
	wp           *workerpool.WorkerPool
	requestQueue rqueue.Queue
	workUnits    *objectstorage.ObjectStorage
	opts         Options
}

// The Options for the Processor.
type Options struct {
	ValidMWM          uint64
	WorkUnitCacheOpts profile.CacheOpts
}

// Run runs the processor and blocks until the shutdown signal is triggered.
func (proc *Processor) Run(shutdownSignal <-chan struct{}) {
	proc.wp.Start()
	<-shutdownSignal
	proc.wp.StopAndWait()
}

// Process submits the given message to the processor for processing.
func (proc *Processor) Process(p *peer.Peer, msgType message.Type, data []byte) {
	proc.wp.Submit(p, msgType, data)
}

// ValidateTransactionTrytesAndEmit validates the given transaction trytes which were not received via gossip but
// through some other mechanism. This function does not run within the Processor's worker pool.
// Emits a TransactionProcessed and BroadcastTransaction event if the transaction was processed.
func (proc *Processor) ValidateTransactionTrytesAndEmit(txTrytes trinary.Trytes) error {
	if !guards.IsTransactionTrytes(txTrytes) {
		return consts.ErrInvalidTransactionTrytes
	}

	txTrits, err := trinary.TrytesToTrits(txTrytes)
	if err != nil {
		return err
	}

	tx, err := transaction.ParseTransaction(txTrits, true)
	if err != nil {
		return err
	}

	hashTrits := batchhasher.CURLP81.Hash(txTrits)
	tx.Hash = trinary.MustTritsToTrytes(hashTrits)

	if tx.Value != 0 {
		// last trit must be zero because of KERL
		if txTrits[consts.AddressTrinaryOffset+consts.AddressTrinarySize-1] != 0 {
			return consts.ErrInvalidAddress
		}

		if math.AbsInt64(tx.Value) > consts.TotalSupply {
			return consts.ErrInsufficientBalance
		}
	}

	if !transaction.HasValidNonce(tx, config.NodeConfig.GetUint64(config.CfgCoordinatorMWM)) {
		return consts.ErrInvalidTransactionHash
	}

	return proc.CompressAndEmit(tx, txTrits)
}

// CompressAndEmit compresses the given transaction and emits TransactionProcessed and BroadcastTransaction events.
// This function does not run within the Processor's worker pool.
func (proc *Processor) CompressAndEmit(tx *transaction.Transaction, txTrits trinary.Trits) error {
	txBytesTruncated := compressed.TruncateTx(trinary.MustTritsToBytes(txTrits))
	hornetTx := hornet.NewTransactionFromTx(tx, txBytesTruncated)

	if timeValid, _ := proc.ValidateTimestamp(hornetTx); !timeValid {
		return ErrInvalidTimestamp
	}

	proc.Events.TransactionProcessed.Trigger(hornetTx, (*rqueue.Request)(nil), (*peer.Peer)(nil))
	proc.Events.BroadcastTransaction.Trigger(&bqueue.Broadcast{
		TxData:          txBytesTruncated,
		RequestedTxHash: hornetTx.GetTxHash(),
	})
	return nil
}

// WorkUnitSize returns the size of WorkUnits currently cached.
func (proc *Processor) WorkUnitsSize() int {
	return proc.workUnits.GetSize()
}

// gets a CachedWorkUnit or creates a new one if it not existent.
func (proc *Processor) workUnitFor(receivedTxBytes []byte) *CachedWorkUnit {
	return &CachedWorkUnit{
		proc.workUnits.ComputeIfAbsent(receivedTxBytes, func(key []byte) objectstorage.StorableObject { // cachedWorkUnit +1
			cachedWorkUnit, _, _ := workUnitFactory(receivedTxBytes)
			return cachedWorkUnit
		}),
	}
}

// processes the given milestone request by parsing it and then replying to the peer with it.
func (proc *Processor) processMilestoneRequest(p *peer.Peer, data []byte) {
	msIndex, err := sting.ExtractRequestedMilestoneIndex(data)
	if err != nil {
		metrics.SharedServerMetrics.InvalidRequests.Inc()

		// drop the connection to the peer
		proc.pm.Remove(p.ID)
		return
	}

	// peers can request the latest milestone we know
	if msIndex == sting.LatestMilestoneRequestIndex {
		msIndex = tangle.GetLatestMilestoneIndex()
	}

	cachedReqMs := tangle.GetMilestoneOrNil(msIndex) // bundle +1
	if cachedReqMs == nil {
		// can't reply if we don't have the wanted milestone
		return
	}

	cachedTxs := cachedReqMs.GetBundle().GetTransactions() // txs +1
	for _, cachedTxToSend := range cachedTxs {
		transactionMsg, _ := sting.NewTransactionMessage(cachedTxToSend.GetTransaction().RawBytes)
		p.EnqueueForSending(transactionMsg)
	}
	cachedTxs.Release(true)   // txs -1
	cachedReqMs.Release(true) // bundle -1
}

// processes the given transaction request by parsing it and then replying to the peer with it.
func (proc *Processor) processTransactionRequest(p *peer.Peer, data []byte) {
	if len(data) != 49 {
		return
	}

	cachedTx := tangle.GetCachedTransactionOrNil(hornet.Hash(data)) // tx +1
	if cachedTx == nil {
		// can't reply if we don't have the requested transaction
		return
	}
	defer cachedTx.Release()

	transactionMsg, _ := sting.NewTransactionMessage(cachedTx.GetTransaction().RawBytes)
	p.EnqueueForSending(transactionMsg)
}

// gets or creates a new WorkUnit for the given transaction and then processes the WorkUnit.
func (proc *Processor) processTransaction(p *peer.Peer, data []byte) {
	cachedWorkUnit := proc.workUnitFor(data) // workUnit +1
	defer cachedWorkUnit.Release()           // workUnit -1
	workUnit := cachedWorkUnit.WorkUnit()
	workUnit.addReceivedFrom(p, nil)
	proc.processWorkUnit(workUnit, p)
}

// tries to process the WorkUnit by first checking in what state it is.
// if the WorkUnit is invalid (because the underlying transaction is invalid), the given peer is punished.
// if the WorkUnit is already completed, and the transaction was requested, this function emits a TransactionProcessed event.
// it is safe to call this function for the same WorkUnit multiple times.
func (proc *Processor) processWorkUnit(wu *WorkUnit, p *peer.Peer) {
	wu.processingLock.Lock()

	switch {
	case wu.Is(Hashing):
		wu.processingLock.Unlock()
		return
	case wu.Is(Invalid):
		wu.processingLock.Unlock()

		metrics.SharedServerMetrics.InvalidTransactions.Inc()

		// drop the connection to the peer
		proc.pm.Remove(p.ID)

		return
	case wu.Is(Hashed):
		wu.processingLock.Unlock()

		// emit an event to say that a transaction was fully processed
		if request := proc.requestQueue.Received(wu.tx.GetTxHash()); request != nil {
			proc.Events.TransactionProcessed.Trigger(wu.tx, request, p)
			wu.wasStale = false
			return
		}

		if wu.wasStale {
			metrics.SharedServerMetrics.StaleTransactions.Inc()
			p.Metrics.StaleTransactions.Inc()
			return
		}

		if tangle.ContainsTransaction(wu.tx.GetTxHash()) {
			metrics.SharedServerMetrics.KnownTransactions.Inc()
			p.Metrics.KnownTransactions.Inc()
			return
		}

		return
	}

	wu.UpdateState(Hashing)
	wu.processingLock.Unlock()

	// this blocks until the transaction was also hashed
	tx, err := compressed.TransactionFromCompressedBytes(wu.receivedTxBytes)
	if err != nil {
		wu.UpdateState(Invalid)
		wu.punish()
		return
	}

	// build Hornet representation of the transaction
	hornetTx := hornet.NewTransactionFromTx(tx, wu.receivedTxBytes)

	// mark the transaction as received
	request := proc.requestQueue.Received(hornetTx.GetTxHash())

	// validate minimum weight magnitude requirement
	if request == nil && !transaction.HasValidNonce(tx, proc.opts.ValidMWM) {
		wu.UpdateState(Invalid)
		wu.punish()
		return
	}

	timestampValid, broadcast := proc.ValidateTimestamp(hornetTx)

	wu.dataLock.Lock()
	wu.receivedTxHash = hornetTx.GetTxHash()
	wu.tx = hornetTx
	wu.dataLock.Unlock()

	wu.UpdateState(Hashed)

	// mark the WorkUnit as containing a stale transaction but
	if request == nil && !timestampValid {
		wu.wasStale = true
		wu.stale()
		return
	}

	// check the existence of the transaction before broadcasting it
	containsTx := tangle.ContainsTransaction(hornetTx.GetTxHash())

	proc.Events.TransactionProcessed.Trigger(hornetTx, request, p)

	// increase the known transaction count for all other peers
	wu.increaseKnownTxCount(p)

	// broadcast the transaction if it wasn't requested and the timestamp is
	// within what we consider a sensible delta from now
	if request == nil && broadcast && !containsTx {
		proc.Events.BroadcastTransaction.Trigger(wu.broadcast())
	}
}

// checks whether the given transaction's timestamp is valid.
// the timestamp is automatically valid if the transaction is a solid entry point.
// the timestamp should be in the range of +/- 10 minutes to current time.
func (proc *Processor) ValidateTimestamp(hornetTx *hornet.Transaction) (valid, broadcast bool) {
	snapshotTimestamp := tangle.GetSnapshotInfo().Timestamp
	txTimestamp := hornetTx.GetTimestamp()

	pastTime := time.Now().Add(-10 * time.Minute).Unix()
	futureTime := time.Now().Add(10 * time.Minute).Unix()

	// we need to accept all txs since the snapshot timestamp for synchronization
	if txTimestamp >= snapshotTimestamp && txTimestamp < futureTime {
		return true, txTimestamp >= pastTime
	}

	// ignore invalid timestamps for solid entry points
	return tangle.SolidEntryPointsContain(hornetTx.GetTxHash()), false
}
