package gossip

import (
	"time"

	daemon "github.com/iotaledger/hive.go/daemon/ordered"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/packages/compressed"
	"github.com/gohornet/hornet/packages/curl"
	"github.com/gohornet/hornet/packages/datastructure"
	"github.com/gohornet/hornet/packages/integerutil"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/queue"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/packages/syncutils"
	"github.com/gohornet/hornet/packages/typeutils"
	"github.com/gohornet/hornet/packages/workerpool"
	"github.com/gohornet/hornet/plugins/gossip/server"
)

const (
	TRANSACTION_FILTER_SIZE            = 5000
	PACKET_PROCESSOR_WORKER_QUEUE_SIZE = 50000
)

var (
	packetProcessorWorkerCount = curl.CURLP81.GetBatchSize() * curl.CURLP81.GetWorkerCount()
	packetProcessorWorkerPool  *workerpool.WorkerPool

	RequestQueue  *queue.RequestQueue
	incomingCache = datastructure.NewLRUCache(TRANSACTION_FILTER_SIZE)

	ErrTxExpired = errors.New("tx too old")
)

func configurePacketProcessor() {
	RequestQueue = queue.NewRequestQueue()

	gossipLogger.Infof("Configuring packetProcessorWorkerPool with %d workers", packetProcessorWorkerCount)
	packetProcessorWorkerPool = workerpool.New(func(task workerpool.Task) {

		switch task.Param(2).(ProtocolMsgType) {

		case PROTOCOL_MSG_TYPE_LEGACY_TX_GOSSIP:
			ProcessReceivedLegacyTransactionGossipData(task.Param(0).(*protocol), task.Param(1).([]byte))

		case PROTOCOL_MSG_TYPE_TX_GOSSIP:
			ProcessReceivedTransactionGossipData(task.Param(0).(*protocol), task.Param(1).([]byte))

		case PROTOCOL_MSG_TYPE_TX_REQ_GOSSIP:
			ProcessReceivedTransactionRequestData(task.Param(0).(*protocol), task.Param(1).([]byte))

		case PROTOCOL_MSG_TYPE_MS_REQUEST:
			ProcessReceivedMilestoneRequest(task.Param(0).(*protocol), task.Param(1).([]byte))
		}

		task.Return(nil)
	}, workerpool.WorkerCount(packetProcessorWorkerCount), workerpool.QueueSize(PACKET_PROCESSOR_WORKER_QUEUE_SIZE))
}

func runPacketProcessor() {
	gossipLogger.Info("Starting PacketProcessor ...")

	daemon.BackgroundWorker("PacketProcessor", func(shutdownSignal <-chan struct{}) {
		gossipLogger.Info("Starting PacketProcessor ... done")
		packetProcessorWorkerPool.Start()
		<-shutdownSignal
		packetProcessorWorkerPool.StopAndWait()
		gossipLogger.Info("Stopping PacketProcessor ... done")
	}, shutdown.ShutdownPriorityPacketProcessor)
}

type NeighborRequest struct {
	p                          *protocol
	reqHashBytes               []byte
	reqMilestoneIndex          milestone_index.MilestoneIndex
	isMilestoneRequest         bool
	isTransactionRequest       bool
	isLegacyTransactionRequest bool
	hasNoRequest               bool
}

func (n *NeighborRequest) punish() {
	n.p.Neighbor.Metrics.IncrInvalidTransactionsCount()
}

func (n *NeighborRequest) notify(recHashBytes []byte) {
	n.p.Neighbor.Reply(recHashBytes, n)
}

type PendingNeighborRequests struct {
	startProcessingLock syncutils.Mutex

	// data
	dataLock     syncutils.RWMutex
	recTxBytes   []byte
	recHashBytes []byte
	recHash      trinary.Hash
	hornetTx     *hornet.Transaction

	// status
	statusLock syncutils.RWMutex
	invalid    bool
	hashing    bool

	// requests
	requestsLock syncutils.RWMutex
	requests     []*NeighborRequest
}

func (p *PendingNeighborRequests) AddLegacyTxRequest(neighbor *protocol, reqHashBytes []byte) {
	p.requestsLock.Lock()
	defer p.requestsLock.Unlock()

	p.requests = append(p.requests, &NeighborRequest{
		p:                          neighbor,
		reqHashBytes:               reqHashBytes,
		isLegacyTransactionRequest: true,
	})
}

func (p *PendingNeighborRequests) BlockFeedback(neighbor *protocol) {
	p.requestsLock.Lock()
	defer p.requestsLock.Unlock()

	p.requests = append(p.requests, &NeighborRequest{
		p:            neighbor,
		hasNoRequest: true,
	})
}

func (p *PendingNeighborRequests) IsHashing() bool {
	p.statusLock.RLock()
	defer p.statusLock.RUnlock()
	return p.hashing
}

func (p *PendingNeighborRequests) IsHashed() bool {
	p.statusLock.RLock()
	defer p.statusLock.RUnlock()
	return len(p.recHashBytes) > 0
}

func (p *PendingNeighborRequests) IsInvalid() bool {
	p.statusLock.RLock()
	defer p.statusLock.RUnlock()
	return p.invalid
}

func (p *PendingNeighborRequests) GetTxHash() trinary.Hash {
	p.dataLock.RLock()
	defer p.dataLock.RUnlock()
	return p.recHash
}

func (p *PendingNeighborRequests) GetTxHashBytes() []byte {
	p.dataLock.RLock()
	defer p.dataLock.RUnlock()
	return p.recHashBytes
}

func (p *PendingNeighborRequests) process() {
	p.startProcessingLock.Lock()

	if p.IsHashing() {
		p.startProcessingLock.Unlock()
		return
	} else if p.IsInvalid() {
		p.startProcessingLock.Unlock()
		p.punish()
		return
	} else if p.IsHashed() {
		p.startProcessingLock.Unlock()

		// Mark the pending request as received because we received the requested Tx Hash
		requested := RequestQueue.MarkReceived(p.hornetTx.Tx.Hash)

		if requested {
			// Tx is requested => ignore that it was marked as stale before
			p.hornetTx.SetRequested(requested)
			Events.ReceivedTransaction.Trigger(p.hornetTx)
		}

		p.notify()
		return
	}

	p.statusLock.Lock()
	p.hashing = true
	p.statusLock.Unlock()
	p.startProcessingLock.Unlock()

	tx, err := compressed.TransactionFromCompressedBytes(p.recTxBytes)
	if err != nil {
		return
	}

	if !transaction.HasValidNonce(tx, ownMWM) {
		// PoW is invalid => punish neighbor
		p.statusLock.Lock()
		p.invalid = true
		p.statusLock.Unlock()

		// Do not answer
		p.punish()
		return
	}

	// Mark the pending request as received because we received the requested Tx Hash
	requested := RequestQueue.MarkReceived(tx.Hash)

	// POW valid => Process the message
	hornetTx := hornet.NewTransactionFromGossip(tx, p.recTxBytes, requested)

	// received tx was not requested and has an invalid timestamp (maybe before snapshot?)
	// => do not store in our database
	// => we need to reply to answer the neighbors request
	timeValid, broadcast := checkTimestamp(hornetTx)
	stale := !requested && !timeValid
	recHashBytes := trinary.MustTrytesToBytes(tx.Hash)[:49]

	p.dataLock.Lock()
	p.recHash = tx.Hash
	p.recHashBytes = recHashBytes
	p.hornetTx = hornetTx
	p.dataLock.Unlock()

	p.statusLock.Lock()
	p.hashing = false
	p.statusLock.Unlock()

	if !stale {
		// Ignore stale transactions until they are requested
		Events.ReceivedTransaction.Trigger(hornetTx)

		if !requested && broadcast {
			p.broadcast()
		}
	} else if len(p.requests) == 1 {
		p.requests[0].p.Neighbor.Metrics.IncrInvalidTransactionsCount()
	}

	p.notify()
}

func BroadcastTransactionFromAPI(txTrytes trinary.Trytes) error {

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

	hashTrits := curl.CURLP81.Hash(txTrits)
	tx.Hash = trinary.MustTritsToTrytes(hashTrits)

	if tx.Value != 0 {
		// Additional checks
		if txTrits[consts.AddressTrinaryOffset+consts.AddressTrinarySize-1] != 0 {
			// The last trit is always zero because of KERL/keccak
			return consts.ErrInvalidAddress
		}

		if uint64(integerutil.Abs(tx.Value)) > compressed.TOTAL_SUPPLY {
			return consts.ErrInsufficientBalance
		}
	}

	if !transaction.HasValidNonce(tx, ownMWM) {
		return consts.ErrInvalidTransactionHash
	}

	txBytesTruncated := compressed.TruncateTx(trinary.TritsToBytes(txTrits))
	hornetTx := hornet.NewTransactionFromAPI(tx, txBytesTruncated)

	if timeValid, _ := checkTimestamp(hornetTx); !timeValid {
		return ErrTxExpired
	}

	Events.ReceivedTransaction.Trigger(hornetTx)
	BroadcastTransaction(make(map[string]struct{}), txBytesTruncated, trinary.TritsToBytes(hashTrits))

	return nil
}

func (p *PendingNeighborRequests) notify() {
	p.requestsLock.Lock()

	for _, n := range p.requests {
		n.notify(p.recHashBytes)
	}

	p.requests = make([]*NeighborRequest, 0)
	p.requestsLock.Unlock()
}

func (p *PendingNeighborRequests) punish() {
	p.requestsLock.Lock()

	for _, n := range p.requests {
		// Tx is known as invalid => punish neighbor
		server.SharedServerMetrics.IncrInvalidTransactionsCount()
		n.p.Neighbor.Metrics.IncrInvalidTransactionsCount()
		n.punish()
	}

	p.requests = make([]*NeighborRequest, 0)
	p.requestsLock.Unlock()
}

func (p *PendingNeighborRequests) broadcast() {
	p.requestsLock.RLock()

	excludedNeighbors := make(map[string]struct{})
	for _, neighbor := range p.requests {
		excludedNeighbors[neighbor.p.Neighbor.Identity] = struct{}{}
	}
	p.requestsLock.RUnlock()

	BroadcastTransaction(excludedNeighbors, p.recTxBytes, p.GetTxHashBytes())
}

func pendingRequestFor(recTxBytes []byte) (result *PendingNeighborRequests) {

	cacheKey := typeutils.BytesToString(recTxBytes)

	if cacheResult := incomingCache.ComputeIfAbsent(cacheKey, func() interface{} {
		return &PendingNeighborRequests{
			recTxBytes: recTxBytes,
			requests:   make([]*NeighborRequest, 0),
		}
	}); !typeutils.IsInterfaceNil(cacheResult) {
		result = cacheResult.(*PendingNeighborRequests)
	}
	return
}

var genesisTruncatedBytes = make([]byte, NON_SIG_TX_PART_BYTES_LENGTH)

func ProcessReceivedMilestoneRequest(protocol *protocol, data []byte) {
	server.SharedServerMetrics.IncrReceivedMilestoneRequestsCount()
	protocol.Neighbor.Metrics.IncrReceivedMilestoneRequestsCount()

	// the raw message contains the index of a milestone they want
	reqMilestoneIndex, err := ExtractRequestedMilestoneIndex(data)
	if err != nil {
		// TODO: increase invalid milestone request counter
		protocol.Neighbor.Metrics.IncrInvalidTransactionsCount()
		return
	}

	// TODO: add metrics
	protocol.Neighbor.Reply(nil, &NeighborRequest{
		p:                  protocol,
		reqMilestoneIndex:  reqMilestoneIndex,
		isMilestoneRequest: true,
	})
}

func ProcessReceivedLegacyTransactionGossipData(protocol *protocol, data []byte) {
	// increment txs count
	server.SharedServerMetrics.IncrAllTransactionsCount()
	protocol.Neighbor.Metrics.IncrAllTransactionsCount()

	var pending *PendingNeighborRequests

	// The raw message contains a TX received from the neighbor and the hash of a TX they want
	// copy requested tx hash
	txDataLen := len(data) - GOSSIP_REQUESTED_TX_HASH_BYTES_LENGTH
	reqHashBytes := ExtractRequestedTxHash(data)

	txData := data[:txDataLen]
	pending = pendingRequestFor(txData)
	pending.AddLegacyTxRequest(protocol, reqHashBytes)
	pending.process()
}

func ProcessReceivedTransactionGossipData(protocol *protocol, txData []byte) {
	// increment txs count
	// TODO: maybe separate metrics
	server.SharedServerMetrics.IncrAllTransactionsCount()
	protocol.Neighbor.Metrics.IncrAllTransactionsCount()

	var pending *PendingNeighborRequests
	pending = pendingRequestFor(txData)
	pending.BlockFeedback(protocol)
	pending.process()

	protocol.Neighbor.SendTransactionRequest()
}

func ProcessReceivedTransactionRequestData(protocol *protocol, reqHash []byte) {
	server.SharedServerMetrics.IncrReceivedTransactionRequestCount()
	protocol.Neighbor.Reply(nil, &NeighborRequest{
		p:                    protocol,
		reqHashBytes:         reqHash,
		isTransactionRequest: true,
	})
}

func checkTimestamp(hornetTx *hornet.Transaction) (valid, broadcast bool) {
	// Timestamp should be in the range of +/- 10 minutes to current time
	// or Transaction was a solid entry point

	snapshotTimestamp := tangle.GetSnapshotInfo().Timestamp
	txTimestamp := hornetTx.GetTimestamp()

	pastTime := time.Now().Add(-10 * time.Minute).Unix()
	futureTime := time.Now().Add(10 * time.Minute).Unix()

	if (txTimestamp >= snapshotTimestamp) && (txTimestamp < futureTime) {
		// We need to accept all tx since snapshot timestamp for warp sync
		return true, (txTimestamp >= pastTime)
	}

	// ignore invalid timestamps for solid entry points
	return tangle.SolidEntryPointsContain(hornetTx.GetHash()), false
}
