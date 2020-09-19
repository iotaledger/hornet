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

				// drain request queue
				for r := RequestQueue().Next(); r != nil; r = RequestQueue().Next() {
					requested := false
					manager.ForAllConnected(func(p *peer.Peer) bool {
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

	if tangle.ContainsMessage(hash) {
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

// RequestParents enqueues requests for the parents of the given transaction to the request queue, if the
// given transaction is not a solid entry point and neither its parents are and also not in the database.
func RequestParents(cachedMsg *tangle.CachedMessage, msIndex milestone.Index, preventDiscard ...bool) {
	cachedMsg.ConsumeMetadata(func(metadata *hornet.MessageMetadata) {
		txHash := metadata.GetMessageID()

		if tangle.SolidEntryPointsContain(txHash) {
			return
		}

		Request(metadata.GetParent1MessageID(), msIndex, preventDiscard...)
		if !bytes.Equal(metadata.GetParent1MessageID(), metadata.GetParent2MessageID()) {
			Request(metadata.GetParent2MessageID(), msIndex, preventDiscard...)
		}
	})
}

// RequestMilestoneParents enqueues requests for the parents of the given milestone bundle to the request queue,
// if the parents are not solid entry points and not already in the database.
func RequestMilestoneParents(cachedMilestone *tangle.CachedMilestone) bool {
	defer cachedMilestone.Release(true) // message -1

	msIndex := cachedMilestone.GetMilestone().Index

	cachedMilestoneMsgMeta := tangle.GetCachedMessageMetadataOrNil(cachedMilestone.GetMilestone().MessageID) // meta +1
	if cachedMilestoneMsgMeta == nil {
		panic("milestone metadata doesn't exist")
	}
	defer cachedMilestoneMsgMeta.Release(true) // meta -1

	txMeta := cachedMilestoneMsgMeta.GetMetadata()
	enqueued := Request(txMeta.GetParent1MessageID(), msIndex, true)
	if !bytes.Equal(txMeta.GetParent1MessageID(), txMeta.GetParent2MessageID()) {
		enqueuedTwo := Request(txMeta.GetParent2MessageID(), msIndex, true)
		if !enqueued && enqueuedTwo {
			enqueued = true
		}
	}

	return enqueued
}

// MemoizedRequestMissingMilestoneParents returns a function which traverses the parents
// of a given milestone and requests each missing parent. As a special property, invocations
// of the yielded function share the same 'already traversed' set to circumvent requesting
// the same parents multiple times.
func MemoizedRequestMissingMilestoneParents(preventDiscard ...bool) func(ms milestone.Index) {
	traversed := map[string]struct{}{}
	return func(ms milestone.Index) {

		cachedMs := tangle.GetCachedMilestoneOrNil(ms) // milestone +1
		if cachedMs == nil {
			log.Panicf("milestone %d wasn't found", ms)
		}

		msHash := cachedMs.GetMilestone().MessageID
		cachedMs.Release(true) // message -1

		dag.TraverseParents(msHash,
			// traversal stops if no more messages pass the given condition
			// Caution: condition func is not in DFS order
			func(cachedMsgMeta *tangle.CachedMetadata) (bool, error) { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1
				_, previouslyTraversed := traversed[string(cachedMsgMeta.GetMetadata().GetMessageID())]
				return !cachedMsgMeta.GetMetadata().IsSolid() && !previouslyTraversed, nil
			},
			// consumer
			func(cachedMsgMeta *tangle.CachedMetadata) error { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1
				traversed[string(cachedMsgMeta.GetMetadata().GetMessageID())] = struct{}{}
				return nil
			},
			// called on missing parents
			func(parentHash hornet.Hash) error {
				Request(parentHash, ms, preventDiscard...)
				return nil
			},
			// called on solid entry points
			// Ignore solid entry points (snapshot milestone included)
			nil,
			false, nil)
	}
}
