package gossip

import (
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/batchhasher"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/math"

	"github.com/gohornet/hornet/packages/compressed"
	"github.com/gohornet/hornet/packages/metrics"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/queue"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
)

var (
	packetProcessorWorkerCount = batchhasher.CURLP81.GetBatchSize() * batchhasher.CURLP81.GetWorkerCount()
	packetProcessorWorkerPool  = (&async.WorkerPool{}).Tune(packetProcessorWorkerCount)

	RequestQueue *queue.RequestQueue

	ErrTxExpired = errors.New("tx too old")
)

func configurePacketProcessor() {

	RequestQueue = queue.NewRequestQueue()
	configureIncomingStorage()

	gossipLogger.Infof("Configuring packetProcessorWorkerPool with %d workers", packetProcessorWorkerCount)
}

func runPacketProcessor() {
	gossipLogger.Info("Starting PacketProcessor ...")

	daemon.BackgroundWorker("PacketProcessor", func(shutdownSignal <-chan struct{}) {
		gossipLogger.Info("Starting PacketProcessor ... done")
		<-shutdownSignal
		gossipLogger.Info("Stopping PacketProcessor ...")
		packetProcessorWorkerPool.Shutdown()
		gossipLogger.Info("Stopping PacketProcessor ... done")
	}, shutdown.ShutdownPriorityPacketProcessor)
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

	hashTrits := batchhasher.CURLP81.Hash(txTrits)
	tx.Hash = trinary.MustTritsToTrytes(hashTrits)

	if tx.Value != 0 {
		// Additional checks
		if txTrits[consts.AddressTrinaryOffset+consts.AddressTrinarySize-1] != 0 {
			// The last trit is always zero because of KERL/keccak
			return consts.ErrInvalidAddress
		}

		if uint64(math.Abs(tx.Value)) > compressed.TOTAL_SUPPLY {
			return consts.ErrInsufficientBalance
		}
	}

	if !transaction.HasValidNonce(tx, ownMWM) {
		return consts.ErrInvalidTransactionHash
	}

	txBytesTruncated := compressed.TruncateTx(trinary.MustTritsToBytes(txTrits))
	hornetTx := hornet.NewTransaction(tx, txBytesTruncated)

	if timeValid, _ := checkTimestamp(hornetTx); !timeValid {
		return ErrTxExpired
	}

	Events.ReceivedTransaction.Trigger(hornetTx, false, milestone_index.MilestoneIndex(0), (*metrics.NeighborMetrics)(nil))
	BroadcastTransaction(make(map[string]struct{}), txBytesTruncated, trinary.MustTritsToBytes(hashTrits))

	return nil
}

func ProcessReceivedMilestoneRequest(protocol *protocol, data []byte) {
	metrics.SharedServerMetrics.IncrReceivedMilestoneRequestsCount()
	protocol.Neighbor.Metrics.IncrReceivedMilestoneRequestsCount()

	// the raw message contains the index of a milestone they want
	reqMilestoneIndex, err := ExtractRequestedMilestoneIndex(data)
	if err != nil {
		metrics.SharedServerMetrics.IncrInvalidRequestsCount()
		protocol.Neighbor.Metrics.IncrInvalidRequestsCount()
		return
	}

	protocol.Neighbor.Reply(nil, &NeighborRequest{
		p:                  protocol,
		reqMilestoneIndex:  reqMilestoneIndex,
		isMilestoneRequest: true,
	})
}

func ProcessReceivedLegacyTransactionGossipData(protocol *protocol, data []byte) {
	// increment txs count
	metrics.SharedServerMetrics.IncrAllTransactionsCount()
	protocol.Neighbor.Metrics.IncrAllTransactionsCount()

	var pending *PendingNeighborRequests

	// The raw message contains a TX received from the neighbor and the hash of a TX they want
	// copy requested tx hash
	txDataLen := len(data) - GOSSIP_REQUESTED_TX_HASH_BYTES_LENGTH
	reqHashBytes := ExtractRequestedTxHash(data)

	txData := data[:txDataLen]
	cachedRequest := GetCachedPendingNeighborRequest(txData) // neighborReq +1
	pending = cachedRequest.GetRequest()

	pending.AddLegacyTxRequest(protocol, reqHashBytes)
	pending.process(protocol.Neighbor)

	cachedRequest.Release() // neighborReq -1
}

func ProcessReceivedTransactionGossipData(protocol *protocol, txData []byte) {
	// increment txs count
	metrics.SharedServerMetrics.IncrAllTransactionsCount()
	protocol.Neighbor.Metrics.IncrAllTransactionsCount()

	cachedRequest := GetCachedPendingNeighborRequest(txData) // neighborReq +1
	pending := cachedRequest.GetRequest()

	pending.BlockFeedback(protocol)
	pending.process(protocol.Neighbor)

	cachedRequest.Release() // neighborReq -1

	protocol.Neighbor.SendTransactionRequest()
}

func ProcessReceivedTransactionRequestData(protocol *protocol, reqHash []byte) {
	metrics.SharedServerMetrics.IncrReceivedTransactionRequestsCount()
	protocol.Neighbor.Metrics.IncrReceivedTransactionRequestsCount()

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
