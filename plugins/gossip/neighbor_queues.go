package gossip

import (
	"bytes"
	"fmt"
	"log"
	"runtime"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/gossip/server"
)

const (
	SEND_MS_REQ_QUEUE_SIZE    = 1000
	SEND_LEGACY_TX_QUEUE_SIZE = 1000
	SEND_TX_QUEUE_SIZE        = 1000
	SEND_TX_REQ_SIZE          = 1000
	SEND_HEARTBEAT_SIZE       = 1000
	BROADCAST_QUEUE_SIZE      = 1000
	TX_TRYTES_SIZE            = 2673
)

type broadcastTransaction struct {
	excludedNeighbors map[string]struct{}
	truncatedTxData   []byte
	txHash            []byte
}

type replyItem struct {
	neighborIdentity string
	recHashBytes     []byte
	neighborRequest  *NeighborRequest
}

type legacyGossipTransaction struct {
	truncatedTxData []byte
	reqHash         []byte
}

type neighborQueue struct {
	protocol                  *protocol
	sendMilestoneRequestQueue chan milestone_index.MilestoneIndex
	legacyTxQueue             chan *legacyGossipTransaction
	txQueue                   chan []byte
	txReqQueue                chan []byte
	heartbeatQueue            chan *Heartbeat
	disconnectChan            chan int
}

func newNeighborQueue(p *protocol) *neighborQueue {
	return &neighborQueue{
		protocol:                  p,
		sendMilestoneRequestQueue: make(chan milestone_index.MilestoneIndex, SEND_MS_REQ_QUEUE_SIZE),
		legacyTxQueue:             make(chan *legacyGossipTransaction, SEND_LEGACY_TX_QUEUE_SIZE),
		txQueue:                   make(chan []byte, SEND_TX_QUEUE_SIZE),
		txReqQueue:                make(chan []byte, SEND_TX_REQ_SIZE),
		heartbeatQueue:            make(chan *Heartbeat, SEND_HEARTBEAT_SIZE),
		disconnectChan:            make(chan int, 1),
	}
}

var (
	neighborQueues      = make(map[string]*neighborQueue)
	neighborQueuesMutex syncutils.RWMutex
	broadcastQueue      = make(chan *broadcastTransaction, BROADCAST_QUEUE_SIZE)
	replyWorkerCount    = runtime.NumCPU()
	replyQueueSize      = 10000
	replyWorkerPool     *workerpool.WorkerPool
)

func DebugPrintQueueStats() {
	gossipLogger.Info("STATS:")
	gossipLogger.Infof("   BroadcastQueue: %d", len(broadcastQueue))
	for _, neighbor := range neighborQueues {
		gossipLogger.Infof("   Neighbor (%s): TxQueue: %d, MilestoneReqQueue: %d", neighbor.protocol.Neighbor.Identity, len(neighbor.legacyTxQueue), len(neighbor.sendMilestoneRequestQueue))
	}
}

func configureBroadcastQueue() {

	replyWorkerPool = workerpool.New(func(task workerpool.Task) {
		processReplies(task.Param(0).(*replyItem))
		task.Return(nil)
	}, workerpool.WorkerCount(replyWorkerCount), workerpool.QueueSize(replyQueueSize))
}

func runBroadcastQueue() {
	gossipLogger.Info("Starting Broadcast Queue Dispatcher ...")

	neighborQueuesMutex.RLock()
	for _, neighborQueue := range neighborQueues {
		startNeighborSendQueue(neighborQueue.protocol.Neighbor, neighborQueue)
	}
	neighborQueuesMutex.RUnlock()

	daemon.BackgroundWorker("ReplyProcessor", func(shutdownSignal <-chan struct{}) {
		gossipLogger.Info("Starting ReplyProcessor ... done")
		replyWorkerPool.Start()
		<-shutdownSignal
		gossipLogger.Info("Stopping ReplyProcessor ...")
		replyWorkerPool.StopAndWait()
		gossipLogger.Info("Stopping ReplyProcessor ... done")
	}, shutdown.ShutdownPriorityReplyProcessor)

	daemon.BackgroundWorker("Gossip Broadcast Queue Dispatcher", func(shutdownSignal <-chan struct{}) {
		gossipLogger.Info("Starting Broadcast Queue Dispatcher ... done")

		for {
			select {
			case <-shutdownSignal:
				gossipLogger.Info("Stopping Broadcast Queue Dispatcher ...")
				gossipLogger.Info("Stopping Broadcast Queue Dispatcher ... done")
				return

			case btx := <-broadcastQueue:
				neighborQueuesMutex.RLock()

				if len(btx.excludedNeighbors) == len(neighborQueues) {
					neighborQueuesMutex.RUnlock()
					break
				}

				// Not all neighbors excluded => broadcast
				for _, neighborQueue := range neighborQueues {
					// only send the transaction to non excluded neighbors
					if _, excluded := btx.excludedNeighbors[neighborQueue.protocol.Neighbor.Identity]; !excluded {

						if neighborQueue.protocol.SupportsSTING() {
							select {
							case neighborQueue.txQueue <- btx.truncatedTxData:
							default:
								neighborQueue.protocol.Neighbor.Metrics.IncrDroppedSendPacketsCount()
								server.SharedServerMetrics.IncrDroppedSendPacketsCount()
							}
							continue
						}

						ourReqHash, _, _ := RequestQueue.GetNext()
						if ourReqHash == nil {
							// We are sync, nothing to request => take the hash of the broadcast Tx to signal the neighbor that we are synced
							ourReqHash = btx.txHash
						}

						msg := &legacyGossipTransaction{truncatedTxData: btx.truncatedTxData, reqHash: ourReqHash}
						select {
						case neighborQueue.legacyTxQueue <- msg:
						default:
							neighborQueue.protocol.Neighbor.Metrics.IncrDroppedSendPacketsCount()
							server.SharedServerMetrics.IncrDroppedSendPacketsCount()
						}
					}
				}
				neighborQueuesMutex.RUnlock()
			}
		}
	}, shutdown.ShutdownPriorityBroadcastQueue)
}

func BroadcastTransaction(excludedNeighbors map[string]struct{}, truncatedTxData []byte, txHash []byte) {
	// At broadcast, we already know the data, but we need to request new tx.
	// If we don't have any request, we signal the neighbor that we are synced, by sending the same reqHash like the hash of the data
	broadcastQueue <- &broadcastTransaction{excludedNeighbors: excludedNeighbors, truncatedTxData: truncatedTxData, txHash: txHash}
}

// Reply to the neighbor
func (neighbor *Neighbor) Reply(recHashBytes []byte, neighborReq *NeighborRequest) {
	// At reply, we check if the neighbor requested something (recHashBytes != reqHashBytes)
	//	- If yes, we send the requested data from our database, and we request new tx
	//	- If not, or if we don't have the data, we send the latest milestone, and we request new tx
	//
	// If we don't have any request, we signal the neighbor that we are synced, by sending the same reqHash like the hash of the data
	// 	- If the neighbor was also synced, we stop the gossip by not replying

	replyWorkerPool.Submit(&replyItem{neighborIdentity: neighbor.Identity, recHashBytes: recHashBytes, neighborRequest: neighborReq})
}

// Requests the latest milestone from the neigbor
func (neighbor *Neighbor) RequestLatestMilestone() {
	if !neighbor.Protocol.SupportsSTING() {
		return
	}

	neighborQueuesMutex.RLock()
	if neighborQueue, exists := neighborQueues[neighbor.Identity]; exists {
		select {
		case neighborQueue.sendMilestoneRequestQueue <- 0:
		default:
			neighborQueue.protocol.Neighbor.Metrics.IncrDroppedSendPacketsCount()
			server.SharedServerMetrics.IncrDroppedSendPacketsCount()
		}

	}
	neighborQueuesMutex.RUnlock()
}

// Sends a heartbeat message to the given neighbor
func (neighbor *Neighbor) SendHeartbeat() {
	if !neighbor.Protocol.SupportsSTING() {
		return
	}

	neighborQueuesMutex.RLock()
	if neighborQueue, exists := neighborQueues[neighbor.Identity]; exists {
		snapshotInfo := tangle.GetSnapshotInfo()
		if snapshotInfo != nil {
			msg := &Heartbeat{SolidMilestoneIndex: tangle.GetSolidMilestoneIndex(), PrunedMilestoneIndex: snapshotInfo.PruningIndex}
			select {
			case neighborQueue.heartbeatQueue <- msg:
			default:
				neighborQueue.protocol.Neighbor.Metrics.IncrDroppedSendPacketsCount()
				server.SharedServerMetrics.IncrDroppedSendPacketsCount()
			}
		}
	}
	neighborQueuesMutex.RUnlock()
}

// Sends a transaction request to the given neighbor if we have something in our queue
func (neighbor *Neighbor) SendTransactionRequest() {
	if !neighbor.Protocol.SupportsSTING() {
		return
	}

	lastHb := neighbor.Protocol.Neighbor.LatestHeartbeat
	if lastHb == nil {
		return
	}

	// only send a request if the neighbor should have the transaction given its pruned milestone index
	ourReqHash, _, _ := RequestQueue.GetNextInRange(lastHb.PrunedMilestoneIndex+1, lastHb.SolidMilestoneIndex)
	if ourReqHash == nil {
		// We have nothing to request from the neighbor
		return
	}

	neighborQueuesMutex.RLock()
	if neighborQueue, exists := neighborQueues[neighbor.Identity]; exists {
		select {
		case neighborQueue.txReqQueue <- ourReqHash:
		default:
			neighborQueue.protocol.Neighbor.Metrics.IncrDroppedSendPacketsCount()
			server.SharedServerMetrics.IncrDroppedSendPacketsCount()
		}

	}
	neighborQueuesMutex.RUnlock()
}

// Sends a milestone request message to the given neighbor
func (neighbor *Neighbor) SendMilestoneRequest(msIndex milestone_index.MilestoneIndex) {
	if !neighbor.Protocol.SupportsSTING() {
		return
	}

	neighborQueuesMutex.RLock()
	if neighborQueue, exists := neighborQueues[neighbor.Identity]; exists {
		select {
		case neighborQueue.sendMilestoneRequestQueue <- msIndex:
		default:
			neighborQueue.protocol.Neighbor.Metrics.IncrDroppedSendPacketsCount()
			server.SharedServerMetrics.IncrDroppedSendPacketsCount()
		}

	}
	neighborQueuesMutex.RUnlock()
}

func processReplies(reply *replyItem) {
	neighborQueuesMutex.RLock()
	defer neighborQueuesMutex.RUnlock()

	neighborQueue, exists := neighborQueues[reply.neighborIdentity]
	if !exists {
		return
	}

	if reply.neighborRequest.isTransactionRequest {
		reqHash, err := trinary.BytesToTrytes(reply.neighborRequest.reqHashBytes, 81)
		if err != nil {
			return
		}
		cachedTx := tangle.GetCachedTransaction(reqHash) // tx +1
		defer cachedTx.Release()                         // tx -1
		if !cachedTx.Exists() {
			return
		}
		select {
		case neighborQueue.txQueue <- cachedTx.GetTransaction().GetRawBytes():
		default:
			neighborQueue.protocol.Neighbor.Metrics.IncrDroppedSendPacketsCount()
			server.SharedServerMetrics.IncrDroppedSendPacketsCount()
		}
		return
	}

	if reply.neighborRequest.isLegacyTransactionRequest {
		// If recHashBytes == reqHashBytes, the neighbor is synced (SolidMilestone = LatestMilestone)
		neighborSynced := bytes.Equal(reply.recHashBytes, reply.neighborRequest.reqHashBytes)

		ourReqHash, _, _ := RequestQueue.GetNext()

		// Neighbor is sync and we are sync => no need to reply
		if neighborSynced && (ourReqHash == nil) {
			return
		}

		var err error
		var cachedTxToSend *tangle.CachedTransaction

		if !neighborSynced {
			reqHash, err := trinary.BytesToTrytes(reply.neighborRequest.reqHashBytes, 81)
			if err != nil {
				return
			}

			cachedTx := tangle.GetCachedTransaction(reqHash) // tx +1
			if !cachedTx.Exists() {
				cachedTx.Release() // cachedTx -1
			} else {
				cachedTxToSend = cachedTx
			}
		}

		if cachedTxToSend == nil {
			if ourReqHash == nil {
				// We don't have the tx, and we have nothing to request => no need to reply
				return
			}

			// If we don't have the tx the neighbor requests, send the genesis tx, since it can be compressed
			// This reduces the outgoing traffic if we are not sync

			cachedGenesisTx := tangle.GetCachedTransaction(consts.NullHashTrytes) // tx +1

			if !cachedGenesisTx.Exists() {
				log.Panicf("Genesis tx not found. cachedObject: %p", cachedGenesisTx.CachedObject)
			}

			cachedTxToSend = cachedGenesisTx
		}

		if ourReqHash == nil {
			// We are synced => notify the neighbor
			ourReqHash, err = trinary.TrytesToBytes(cachedTxToSend.GetTransaction().GetHash())
			if err != nil {
				cachedTxToSend.Release() // tx -1
				return
			}
		}

		msg := &legacyGossipTransaction{truncatedTxData: cachedTxToSend.GetTransaction().GetRawBytes(), reqHash: ourReqHash}
		cachedTxToSend.Release() // tx -1

		select {
		case neighborQueue.legacyTxQueue <- msg:
		default:
			neighborQueue.protocol.Neighbor.Metrics.IncrDroppedSendPacketsCount()
			server.SharedServerMetrics.IncrDroppedSendPacketsCount()
		}
		return
	}

	if reply.neighborRequest.isMilestoneRequest {
		if reply.neighborRequest.reqMilestoneIndex == 0 {
			// Milestone Index 0 == Request latest milestone
			reply.neighborRequest.reqMilestoneIndex = tangle.GetLatestMilestoneIndex()
		}

		cachedReqMs := tangle.GetMilestoneOrNil(reply.neighborRequest.reqMilestoneIndex) // bundle +1
		if cachedReqMs == nil {
			// We don't have the requested milestone => no need to reply
			return
		}

		cachedTxs := cachedReqMs.GetBundle().GetTransactions() // txs +1
		for _, cachedTxToSend := range cachedTxs {
			select {
			case neighborQueue.txQueue <- cachedTxToSend.GetTransaction().GetRawBytes():
			default:
				neighborQueue.protocol.Neighbor.Metrics.IncrDroppedSendPacketsCount()
				server.SharedServerMetrics.IncrDroppedSendPacketsCount()
			}
		}
		cachedTxs.Release()   // txs -1
		cachedReqMs.Release() // bundle -1
		return
	}
}

func startNeighborSendQueue(neighbor *Neighbor, neighborQueue *neighborQueue) {
	gossipLogger.Infof("Starting Gossip Send Queue Dispatcher (%s) ...", neighbor.Identity)

	neighbor.RequestLatestMilestone()
	neighbor.SendHeartbeat()

	daemon.BackgroundWorker(fmt.Sprintf("Gossip Send Queue (%s)", neighbor.Identity), func(shutdownSignal <-chan struct{}) {
		for {
			select {
			case <-shutdownSignal:
				gossipLogger.Infof("Stopping Gossip Send Queue Dispatcher (%s) ...", neighbor.Identity)
				gossipLogger.Infof("Stopping Gossip Send Queue Dispatcher (%s) ... done", neighbor.Identity)
				return

			case <-neighborQueue.disconnectChan:
				return

			case legacyTx := <-neighborQueue.legacyTxQueue:
				sendLegacyTransaction(neighborQueue.protocol, legacyTx.truncatedTxData, legacyTx.reqHash)

			case txBytes := <-neighborQueue.txQueue:
				sendTransaction(neighborQueue.protocol, txBytes)

			case txReqHash := <-neighborQueue.txReqQueue:
				sendTransactionRequest(neighborQueue.protocol, txReqHash)

			case hb := <-neighborQueue.heartbeatQueue:
				sendHeartbeat(neighborQueue.protocol, hb)

			case reqMilestoneIndex := <-neighborQueue.sendMilestoneRequestQueue:
				sendMilestoneRequest(neighborQueue.protocol, reqMilestoneIndex)
			}
		}
	}, shutdown.ShutdownPriorityNeighborSendQueue)
}
