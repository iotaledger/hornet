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

				// always fire the signal if something is in the queue, otherwise the sting request is not kicking in
				queued := requestQueue.EnqueuePending(discardRequestsOlderThan)
				if queued > 0 {
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

				// drain request queue
				for r := RequestQueue().Next(); r != nil; r = RequestQueue().Next() {
					requested := false
					manager.ForAllConnected(func(p *peer.Peer) bool {
						if !p.Protocol.Supports(sting.FeatureSet) {
							return true
						}
						// we only send a request message if the peer actually has the data
						// (r.MilestoneIndex > PrunedMilestoneIndex && r.MilestoneIndex <= SolidMilestoneIndex)
						if !p.HasDataFor(r.MilestoneIndex) {
							return true
						}

						helpers.SendTransactionRequest(p, r.Hash)
						requested = true
						return false
					})

					if !requested {
						// We have no neighbor that has the data for sure,
						// so we ask all neighbors that could have the data
						// (r.MilestoneIndex > PrunedMilestoneIndex && r.MilestoneIndex <= LatestMilestoneIndex)
						manager.ForAllConnected(func(p *peer.Peer) bool {
							if !p.Protocol.Supports(sting.FeatureSet) {
								return true
							}

							// we only send a request message if the peer could have the data
							if !p.CouldHaveDataFor(r.MilestoneIndex) {
								return true
							}

							helpers.SendTransactionRequest(p, r.Hash)
							return true
						})
					}
				}
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
func RequestMultiple(hashes hornet.Hashes, msIndex milestone.Index, preventDiscard ...bool) int {
	requested := 0
	for _, hash := range hashes {
		if Request(hash, msIndex, preventDiscard...) {
			requested++
		}
	}
	return requested
}

// RequestApprovees enqueues requests for the approvees of the given transaction to the request queue, if the
// given transaction is not a solid entry point and neither its approvees are and also not in the database.
func RequestApprovees(cachedTx *tangle.CachedTransaction, msIndex milestone.Index, preventDiscard ...bool) {
	cachedTx.ConsumeMetadata(func(metadata *hornet.TransactionMetadata) {
		txHash := metadata.GetTxHash()

		if tangle.SolidEntryPointsContain(txHash) {
			return
		}

		Request(metadata.GetTrunkHash(), msIndex, preventDiscard...)
		if !bytes.Equal(metadata.GetTrunkHash(), metadata.GetBranchHash()) {
			Request(metadata.GetBranchHash(), msIndex, preventDiscard...)
		}
	})
}

// RequestMilestoneApprovees enqueues requests for the approvees of the given milestone bundle to the request queue,
// if the approvees are not solid entry points and not already in the database.
func RequestMilestoneApprovees(cachedMsBndl *tangle.CachedBundle) bool {
	defer cachedMsBndl.Release() // bundle -1

	cachedHeadTxMeta := cachedMsBndl.GetBundle().GetHeadMetadata() // meta +1
	defer cachedHeadTxMeta.Release()                               // meta -1

	msIndex := cachedMsBndl.GetBundle().GetMilestoneIndex()

	txMeta := cachedHeadTxMeta.GetMetadata()
	enqueued := Request(txMeta.GetTrunkHash(), msIndex, true)
	if !bytes.Equal(txMeta.GetTrunkHash(), txMeta.GetBranchHash()) {
		enqueuedTwo := Request(txMeta.GetBranchHash(), msIndex, true)
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

		cachedMs := tangle.GetCachedMilestoneOrNil(ms) // milestone +1
		if cachedMs == nil {
			log.Panicf("milestone %d wasn't found", ms)
		}

		msHash := cachedMs.GetMilestone().Hash
		cachedMs.Release(true) // bundle -1

		dag.TraverseApprovees(msHash,
			// traversal stops if no more transactions pass the given condition
			// Caution: condition func is not in DFS order
			func(cachedTxMeta *tangle.CachedMetadata) (bool, error) { // meta +1
				defer cachedTxMeta.Release(true) // meta -1
				_, previouslyTraversed := traversed[string(cachedTxMeta.GetMetadata().GetTxHash())]
				return !cachedTxMeta.GetMetadata().IsSolid() && !previouslyTraversed, nil
			},
			// consumer
			func(cachedTxMeta *tangle.CachedMetadata) error { // meta +1
				defer cachedTxMeta.Release(true) // meta -1
				traversed[string(cachedTxMeta.GetMetadata().GetTxHash())] = struct{}{}
				return nil
			},
			// called on missing approvees
			func(approveeHash hornet.Hash) error {
				Request(approveeHash, ms, preventDiscard...)
				return nil
			},
			// called on solid entry points
			// Ignore solid entry points (snapshot milestone included)
			nil,
			false, false, nil)
	}
}
