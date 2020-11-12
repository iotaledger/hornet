package gossip

import (
	"time"

	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/shutdown"
)

var (
	requestQueueEnqueueSignal      = make(chan struct{}, 2)
	enqueuePendingRequestsInterval = 1500 * time.Millisecond
	discardRequestsOlderThan       = 10 * time.Second
	requestBackpressureSignals     []func() bool
)

func AddRequestBackpressureSignal(reqFunc func() bool) {
	requestBackpressureSignals = append(requestBackpressureSignals, reqFunc)
}

func runRequestWorkers() {
	CorePlugin.Daemon().BackgroundWorker("PendingRequestsEnqueuer", func(shutdownSignal <-chan struct{}) {
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
				queued := deps.RequestQueue.EnqueuePending(discardRequestsOlderThan)
				if queued > 0 {
					select {
					case requestQueueEnqueueSignal <- struct{}{}:
					default:
					}
				}
			}
		}
	}, shutdown.PriorityRequestsProcessor)

	CorePlugin.Daemon().BackgroundWorker("STINGRequester", func(shutdownSignal <-chan struct{}) {
		rQueue := deps.RequestQueue
		for {
			select {
			case <-shutdownSignal:
				return
			case <-requestQueueEnqueueSignal:

				// drain request queue
				for r := rQueue.Next(); r != nil; r = rQueue.Next() {
					requested := false
					deps.Service.ForEach(func(proto *gossip.Protocol) bool {
						// we only send a request message if the peer actually has the data
						// (r.MilestoneIndex > PrunedMilestoneIndex && r.MilestoneIndex <= SolidMilestoneIndex)
						if !proto.HasDataForMilestone(r.MilestoneIndex) {
							return true
						}

						proto.SendMessageRequest(r.MessageID)
						requested = true
						return false
					})

					if !requested {
						// we have no neighbor that has the data for sure,
						// so we ask all neighbors that could have the data
						// (r.MilestoneIndex > PrunedMilestoneIndex && r.MilestoneIndex <= LatestMilestoneIndex)
						deps.Service.ForEach(func(proto *gossip.Protocol) bool {
							// we only send a request message if the peer could have the data
							if !proto.CouldHaveDataForMilestone(r.MilestoneIndex) {
								return true
							}

							proto.SendMessageRequest(r.MessageID)
							return true
						})
					}
				}
			}
		}
	}, shutdown.PriorityRequestsProcessor)
}

// adds the request to the request queue and signals the request to drain it.
func enqueueAndSignal(r *gossip.Request) bool {
	if !deps.RequestQueue.Enqueue(r) {
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

// Request enqueues a request to the request queue for the given message if it isn't a solid entry point
// and is not contained in the database already.
func Request(messageID *hornet.MessageID, msIndex milestone.Index, preventDiscard ...bool) bool {
	if deps.Storage.SolidEntryPointsContain(messageID) {
		return false
	}

	if deps.Storage.ContainsMessage(messageID) {
		return false
	}

	r := &gossip.Request{
		MessageID:      messageID,
		MilestoneIndex: msIndex,
	}
	if len(preventDiscard) > 0 {
		r.PreventDiscard = preventDiscard[0]
	}
	return enqueueAndSignal(r)
}

// RequestMultiple works like Request but takes multiple message IDs.
func RequestMultiple(messageIDs hornet.MessageIDs, msIndex milestone.Index, preventDiscard ...bool) int {
	requested := 0
	for _, messageID := range messageIDs {
		if Request(messageID, msIndex, preventDiscard...) {
			requested++
		}
	}
	return requested
}

// RequestParents enqueues requests for the parents of the given message to the request queue, if the
// given message is not a solid entry point and neither its parents are and also not in the database.
func RequestParents(cachedMsg *storage.CachedMessage, msIndex milestone.Index, preventDiscard ...bool) {
	cachedMsg.ConsumeMetadata(func(metadata *storage.MessageMetadata) {
		messageID := metadata.GetMessageID()

		if deps.Storage.SolidEntryPointsContain(messageID) {
			return
		}

		Request(metadata.GetParent1MessageID(), msIndex, preventDiscard...)
		if *metadata.GetParent1MessageID() != *metadata.GetParent2MessageID() {
			Request(metadata.GetParent2MessageID(), msIndex, preventDiscard...)
		}
	})
}

// RequestMilestoneParents enqueues requests for the parents of the given milestone to the request queue,
// if the parents are not solid entry points and not already in the database.
func RequestMilestoneParents(cachedMilestone *storage.CachedMilestone) bool {
	defer cachedMilestone.Release(true) // message -1

	msIndex := cachedMilestone.GetMilestone().Index

	cachedMilestoneMsgMeta := deps.Storage.GetCachedMessageMetadataOrNil(cachedMilestone.GetMilestone().MessageID) // meta +1
	if cachedMilestoneMsgMeta == nil {
		panic("milestone metadata doesn't exist")
	}
	defer cachedMilestoneMsgMeta.Release(true) // meta -1

	txMeta := cachedMilestoneMsgMeta.GetMetadata()
	enqueued := Request(txMeta.GetParent1MessageID(), msIndex, true)
	if *txMeta.GetParent1MessageID() != *txMeta.GetParent2MessageID() {
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

		cachedMs := deps.Storage.GetCachedMilestoneOrNil(ms) // milestone +1
		if cachedMs == nil {
			log.Panicf("milestone %d wasn't found", ms)
		}

		milestoneMessageID := cachedMs.GetMilestone().MessageID
		cachedMs.Release(true) // message -1

		dag.TraverseParents(deps.Storage, milestoneMessageID,
			// traversal stops if no more messages pass the given condition
			// Caution: condition func is not in DFS order
			func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1
				_, previouslyTraversed := traversed[cachedMsgMeta.GetMetadata().GetMessageID().MapKey()]
				return !cachedMsgMeta.GetMetadata().IsSolid() && !previouslyTraversed, nil
			},
			// consumer
			func(cachedMsgMeta *storage.CachedMetadata) error { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1
				traversed[cachedMsgMeta.GetMetadata().GetMessageID().MapKey()] = struct{}{}
				return nil
			},
			// called on missing parents
			func(parentMessageID *hornet.MessageID) error {
				Request(parentMessageID, ms, preventDiscard...)
				return nil
			},
			// called on solid entry points
			// Ignore solid entry points (snapshot milestone included)
			nil,
			false, nil)
	}
}
