package gossip

import (
	"runtime"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/gossip/server"
)

var (
	stingRequestsWorkerCount = runtime.NumCPU()
	stingRequestsQueueSize   = 10000
	stingRequestsWorkerPool  *workerpool.WorkerPool
)

func configureSTINGRequestsProcessor() {

	stingRequestsWorkerPool = workerpool.New(func(task workerpool.Task) {
		sendSTINGRequest(task.Param(0).(trinary.Hash), task.Param(1).(milestone_index.MilestoneIndex))
		task.Return(nil)
	}, workerpool.WorkerCount(stingRequestsWorkerCount), workerpool.QueueSize(stingRequestsQueueSize))
}

func runSTINGRequestsProcessor() {

	daemon.BackgroundWorker("STINGRequestsProcessor", func(shutdownSignal <-chan struct{}) {
		gossipLogger.Info("Starting STINGRequestsProcessor ... done")
		stingRequestsWorkerPool.Start()
		<-shutdownSignal
		gossipLogger.Info("Stopping STINGRequestsProcessor ...")
		RequestQueue.Stop()
		stingRequestsWorkerPool.StopAndWait()
		gossipLogger.Info("Stopping STINGRequestsProcessor ... done")
	}, shutdown.ShutdownPriorityRequestsProcessor)
}

func sendSTINGRequest(txHash trinary.Hash, msIndex milestone_index.MilestoneIndex) {

	// send a STING request to all neighbors who supports the STING protocol
	neighborQueuesMutex.RLock()

	// since the iteration order while iterating maps is random, we can simply do this:
	for _, neighborQueue := range neighborQueues {

		if !neighborQueue.protocol.SupportsSTING() {
			continue
		}

		lastHb := neighborQueue.protocol.Neighbor.LatestHeartbeat
		if lastHb == nil {
			continue
		}

		// only send a request if the neighbor should have the transaction given its pruned milestone index
		if lastHb.PrunedMilestoneIndex >= msIndex || lastHb.SolidMilestoneIndex < msIndex {
			continue
		}

		txBytes := trinary.MustTrytesToBytes(txHash)[:49]
		RequestQueue.Add(txHash, msIndex, true)
		server.SharedServerMetrics.IncrSentTransactionRequestCount()
		select {
		case neighborQueue.txReqQueue <- txBytes:
		default:
			neighborQueue.protocol.Neighbor.Metrics.IncrDroppedSendPacketsCount()
			server.SharedServerMetrics.IncrDroppedSendPacketsCount()
		}

		// sent the same request to only one neighbor
		break
	}

	neighborQueuesMutex.RUnlock()
}

// RequestMulti adds multiple request to the queue at once
func RequestMulti(hashes []trinary.Hash, reqMilestoneIndex milestone_index.MilestoneIndex) {
	added := RequestQueue.AddMulti(hashes, reqMilestoneIndex, false)
	for x, txHash := range hashes {
		if added[x] {
			stingRequestsWorkerPool.TrySubmit(txHash, reqMilestoneIndex)
		}
	}
}

// Request adds a request to the queue
func Request(hashes []trinary.Hash, reqMilestoneIndex milestone_index.MilestoneIndex) {

	for _, txHash := range hashes {
		if tangle.SolidEntryPointsContain(txHash) {
			// Ignore solid entry points (snapshot milestone included)
			return
		}
		if tangle.ContainsTransaction(txHash) {
			// Do not request tx that we already know
			continue
		}

		if RequestQueue.Add(txHash, reqMilestoneIndex, false) {
			stingRequestsWorkerPool.TrySubmit(txHash, reqMilestoneIndex)
		}
	}
}

// RequestApproveesAndRemove add the approvees of a tx to the queue and removes the tx from the queue
func RequestApprovees(tx *tangle.CachedTransaction) {

	tx.RegisterConsumer() //+1
	defer tx.Release()    //-1

	txHash := tx.GetTransaction().GetHash()

	if tangle.SolidEntryPointsContain(txHash) {
		// Ignore solid entry points (snapshot milestone included)
		return
	}

	contains, reqMilestoneIndex := RequestQueue.Contains(txHash)
	if contains {
		// Tx was requested => request trunk and branch tx

		approveeHashes := []trinary.Hash{tx.GetTransaction().GetTrunk()}
		if tx.GetTransaction().GetTrunk() != tx.GetTransaction().GetBranch() {
			approveeHashes = append(approveeHashes, tx.GetTransaction().GetBranch())
		}

		approvesToAdd := trinary.Hashes{}
		for _, approveeHash := range approveeHashes {
			if tangle.SolidEntryPointsContain(approveeHash) {
				// Ignore solid entry points (snapshot milestone included)
				continue
			}
			if tangle.ContainsTransaction(approveeHash) {
				// Do not request tx that we already know
				continue
			}
			approvesToAdd = append(approvesToAdd, approveeHash)
		}

		reqsAdded := RequestQueue.AddMulti(approvesToAdd, reqMilestoneIndex, false)
		for i, added := range reqsAdded {
			if added {
				stingRequestsWorkerPool.TrySubmit(approvesToAdd[i], reqMilestoneIndex)
			}
		}
	}
}

// RequestMilestone requests trunk and branch of a milestone if they are missing
// ToDo: add it to the requestsWorkerPool
func RequestMilestone(milestone *tangle.Bundle) bool {
	var requested bool

	milestoneHeadTx := milestone.GetHead() //+1
	defer milestoneHeadTx.Release()        //-1

	reqMilestoneIndex := milestone.GetMilestoneIndex()

	approveeHashes := []trinary.Hash{milestoneHeadTx.GetTransaction().GetTrunk()}
	if milestoneHeadTx.GetTransaction().GetTrunk() != milestoneHeadTx.GetTransaction().GetBranch() {
		approveeHashes = append(approveeHashes, milestoneHeadTx.GetTransaction().GetBranch())
	}

	for _, approveeHash := range approveeHashes {
		if tangle.SolidEntryPointsContain(approveeHash) {
			// Ignore solid entry points (snapshot milestone included)
			continue
		}
		if tangle.ContainsTransaction(approveeHash) {
			// Do not request tx that we already know
			continue
		}

		// Tx is unknown, request it!
		if RequestQueue.Add(approveeHash, reqMilestoneIndex, false) {
			requested = true
			stingRequestsWorkerPool.TrySubmit(approveeHash, reqMilestoneIndex)
		}
	}

	return requested
}
