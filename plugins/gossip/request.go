package gossip

import (
	"time"

	"github.com/gohornet/hornet/pkg/protocol/helpers"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol/rqueue"
	"github.com/gohornet/hornet/pkg/protocol/sting"
	"github.com/gohornet/hornet/pkg/shutdown"
)

var (
	requestQueueEnqueueSignal      = make(chan struct{}, 2)
	enqueuePendingRequestsInterval = 1500 * time.Millisecond
	discardRequestsOlderThan       = 10 * time.Second
)

func runRequestWorkers() {
	daemon.BackgroundWorker("PendingRequestsEnqueuer", func(shutdownSignal <-chan struct{}) {
		enqueueTicker := time.NewTicker(enqueuePendingRequestsInterval)
		for {
			select {
			case <-shutdownSignal:
				return
			case <-enqueueTicker.C:
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
				// drain request queue
				for r := RequestQueue().Next(); r != nil; r = RequestQueue().Next() {
					manager.ForAllConnected(func(p *peer.Peer) bool {
						if !p.Protocol.Supports(sting.FeatureSet) {
							return false
						}
						// we only send a request message if the peer actually has the data
						if !p.HasDataFor(r.MilestoneIndex) {
							return false
						}

						helpers.SendTransactionRequest(p, r.HashBytesEncoded)
						return true
					})
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
func Request(hash trinary.Hash, msIndex milestone.Index, preventDiscard ...bool) bool {
	if tangle.SolidEntryPointsContain(hash) {
		return false
	}

	if tangle.ContainsTransaction(hash) {
		return false
	}

	r := &rqueue.Request{
		Hash:             hash,
		HashBytesEncoded: trinary.MustTrytesToBytes(hash),
		MilestoneIndex:   msIndex,
	}
	if len(preventDiscard) > 0 {
		r.PreventDiscard = preventDiscard[0]
	}
	return enqueueAndSignal(r)
}

// RequestMultiple works like Request but takes multiple transaction hashes.
func RequestMultiple(hashes trinary.Hashes, msIndex milestone.Index) {
	for _, hash := range hashes {
		Request(hash, msIndex, true)
	}
}

// RequestApprovees enqueues requests for the approvees of the given transaction to the request queue, if the
// given transaction is not a solid entry point and neither its approvees are and also not in the database.
func RequestApprovees(cachedTx *tangle.CachedTransaction, msIndex milestone.Index, preventDiscard ...bool) {
	cachedTx.ConsumeTransaction(func(tx *hornet.Transaction, metadata *hornet.TransactionMetadata) {
		txHash := tx.GetHash()

		if tangle.SolidEntryPointsContain(txHash) {
			return
		}

		Request(tx.GetTrunk(), msIndex, preventDiscard...)
		if tx.GetTrunk() != tx.GetBranch() {
			Request(tx.GetBranch(), msIndex, preventDiscard...)
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
	enqueued := Request(tx.GetTrunk(), msIndex, true)
	if tx.GetTrunk() != tx.GetBranch() {
		enqueuedTwo := Request(tx.GetBranch(), msIndex, true)
		if !enqueued && enqueuedTwo {
			enqueued = true
		}
	}

	return enqueued
}
