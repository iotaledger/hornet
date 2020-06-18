package gossip

import (
	"bytes"
	"time"

	"github.com/iotaledger/hive.go/daemon"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol/helpers"
	"github.com/gohornet/hornet/pkg/protocol/rqueue"
	"github.com/gohornet/hornet/pkg/protocol/sting"
	"github.com/gohornet/hornet/pkg/shutdown"
)

var (
	requestQueueEnqueueSignal      = make(chan struct{}, 2)
	enqueuePendingRequestsInterval = 1500 * time.Millisecond
	discardRequestsOlderThan       = 10 * time.Second
	requestBackpressureSignals     [](func() bool)
)

func AddRequestBackpressureSignal(reqFunc func() bool) {
	requestBackpressureSignals = append(requestBackpressureSignals, reqFunc)
}

func runRequestWorkers() {
	daemon.BackgroundWorker("PendingRequestsEnqueuer", func(shutdownSignal <-chan struct{}) {
		enqueueTicker := time.NewTicker(enqueuePendingRequestsInterval)

	requestQueueEnqueueLoop:
		for {
			select {
			case <-shutdownSignal:
				return
			case <-enqueueTicker.C:
				for _, reqBackpressureSignal := range requestBackpressureSignals {
					if reqBackpressureSignal() {
						// skip enqueueing of the pending requests if a backpressure signal is set to true to reduce pressure
						continue requestQueueEnqueueLoop
					}
				}

				newlyEnqueued := requestQueue.EnqueuePending(discardRequestsOlderThan)
				if newlyEnqueued > 0 {
					select {
					case requestQueueEnqueueSignal <- struct{}{}:
					default:
					}
				}
			}
		}
	}, shutdown.PriorityRequestsProcessor)

	daemon.BackgroundWorker("STINGRequester", func(shutdownSignal <-chan struct{}) {
		for {
			select {
			case <-shutdownSignal:
				return
			case <-requestQueueEnqueueSignal:

				// abort if no peer is connected or no peer supports sting
				if !manager.AnySTINGPeerConnected() {
					continue
				}

				RequestQueue().Lock()

				// drain request queue
				for r := RequestQueue().PeekWithoutLocking(); r != nil; r = RequestQueue().PeekWithoutLocking() {
					sent := false
					manager.ForAllConnected(func(p *peer.Peer) bool {
						if !p.Protocol.Supports(sting.FeatureSet) {
							return false
						}
						// we only send a request message if the peer actually has the data
						if !p.HasDataFor(r.MilestoneIndex) {
							return false
						}

						helpers.SendTransactionRequest(p, r.Hash)
						sent = true
						RequestQueue().NextWithoutLocking()
						return true
					})

					// couldn't send a request to any peer, abort
					if !sent {
						break
					}
				}

				RequestQueue().Unlock()
			}
		}
	}, shutdown.PriorityRequestsProcessor)
}

// adds the request to the request queue and signals the request to drain it.
func enqueueAndSignal(r *rqueue.Request) bool {
	if !RequestQueue().Enqueue(r) {
		return false
	}

	// signal requester
	select {
	case requestQueueEnqueueSignal <- struct{}{}:
	default:
		// if the signal queue is full, there's no need to block until it becomes empty
		// as the requester will drain everything present in the queue
	}
	return true
}

// Request enqueues a request to the request queue for the given transaction if it isn't a solid entry point
// and is not contained in the database already.
func Request(hash hornet.Hash, msIndex milestone.Index, preventDiscard ...bool) bool {
	if tangle.SolidEntryPointsContain(hash) {
		return false
	}

	if tangle.ContainsTransaction(hash) {
		return false
	}

	r := &rqueue.Request{
		Hash:           hash,
		MilestoneIndex: msIndex,
	}
	if len(preventDiscard) > 0 {
		r.PreventDiscard = preventDiscard[0]
	}
	return enqueueAndSignal(r)
}

// RequestMultiple works like Request but takes multiple transaction hashes.
func RequestMultiple(hashes hornet.Hashes, msIndex milestone.Index, preventDiscard ...bool) {
	for _, hash := range hashes {
		Request(hash, msIndex, preventDiscard...)
	}
}

// RequestApprovees enqueues requests for the approvees of the given transaction to the request queue, if the
// given transaction is not a solid entry point and neither its approvees are and also not in the database.
func RequestApprovees(cachedTx *tangle.CachedTransaction, msIndex milestone.Index, preventDiscard ...bool) {
	cachedTx.ConsumeTransaction(func(tx *hornet.Transaction, metadata *hornet.TransactionMetadata) {
		txHash := tx.GetTxHash()

		if tangle.SolidEntryPointsContain(txHash) {
			return
		}

		Request(tx.GetTrunkHash(), msIndex, preventDiscard...)
		if !bytes.Equal(tx.GetTrunkHash(), tx.GetBranchHash()) {
			Request(tx.GetBranchHash(), msIndex, preventDiscard...)
		}
	})
}

// RequestMilestoneApprovees enqueues requests for the approvees of the given milestone bundle to the request queue,
// if the approvees are not solid entry points and not already in the database.
func RequestMilestoneApprovees(cachedMsBndl *tangle.CachedBundle) bool {
	defer cachedMsBndl.Release() // bundle -1

	cachedHeadTx := cachedMsBndl.GetBundle().GetHead() // tx +1
	defer cachedHeadTx.Release()                       // tx -1

	msIndex := cachedMsBndl.GetBundle().GetMilestoneIndex()

	tx := cachedHeadTx.GetTransaction()
	enqueued := Request(tx.GetTrunkHash(), msIndex, true)
	if !bytes.Equal(tx.GetTrunkHash(), tx.GetBranchHash()) {
		enqueuedTwo := Request(tx.GetBranchHash(), msIndex, true)
		if !enqueued && enqueuedTwo {
			enqueued = true
		}
	}

	return enqueued
}

// MemoizedRequestMissingMilestoneApprovees returns a function which traverses the approvees
// of a given milestone and requests each missing approvee. As a special property, invocations
// of the yielded function share the same 'already traversed' set to circumvent requesting
// the same approvees multiple times.
func MemoizedRequestMissingMilestoneApprovees(preventDiscard ...bool) func(ms milestone.Index) {
	traversed := map[string]struct{}{}
	return func(ms milestone.Index) {
		cachedMsBundle := tangle.GetMilestoneOrNil(ms) // bundle +1
		if cachedMsBundle == nil {
			log.Panicf("milestone %d wasn't found", ms)
		}

		msBundleTailHash := cachedMsBundle.GetBundle().GetTailHash()
		cachedMsBundle.Release(true) // bundle -1

		dag.TraverseApprovees(msBundleTailHash,
			// predicate
			func(cachedTx *tangle.CachedTransaction) bool { // tx +1
				defer cachedTx.Release(true) // tx -1
				_, previouslyTraversed := traversed[string(cachedTx.GetTransaction().GetTxHash())]
				return !cachedTx.GetMetadata().IsSolid() && !previouslyTraversed
			},
			// consumer
			func(cachedTx *tangle.CachedTransaction) { // tx +1
				defer cachedTx.Release(true) // tx -1
				traversed[string(cachedTx.GetTransaction().GetTxHash())] = struct{}{}
			},
			// called on missing approvees
			func(approveeHash hornet.Hash) {
				Request(approveeHash, ms, preventDiscard...)
			}, true)
	}
}
