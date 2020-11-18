package gossip

import (
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
)

// RequesterOptions are options around a Requester.
type RequesterOptions struct {
	// Defines the re-queue interval for pending requests.
	PendingRequestReEnqueueInterval time.Duration
	// Defines the max age for requests.
	DiscardRequestsOlderThan time.Duration
}

// RequestBackPressureFunc is a function which tells the Requester
// to stop requesting more data.
type RequestBackPressureFunc func() bool

var defaultRequesterOpts = []RequesterOption{
	WithRequesterDiscardRequestsOlderThan(10 * time.Second),
	WithRequesterPendingRequestReEnqueueInterval(1500 * time.Millisecond),
}

// RequesterOption is a function which sets an option on a RequesterOptions instance.
type RequesterOption func(options *RequesterOptions)

// WithRequesterPendingRequestReEnqueueInterval sets the threshold for the max age of requests.
func WithRequesterDiscardRequestsOlderThan(dur time.Duration) RequesterOption {
	return func(options *RequesterOptions) {
		options.DiscardRequestsOlderThan = dur
	}
}

// WithRequesterPendingRequestReEnqueueInterval sets the re-enqueue interval for pending requests.
func WithRequesterPendingRequestReEnqueueInterval(dur time.Duration) RequesterOption {
	return func(options *RequesterOptions) {
		options.PendingRequestReEnqueueInterval = dur
	}
}

// NewRequester creates a new Requester.
func NewRequester(service *Service, rQueue RequestQueue, storage *storage.Storage, opts ...RequesterOption) *Requester {
	reqOpts := RequesterOptions{}
	for _, opt := range defaultRequesterOpts {
		opt(&reqOpts)
	}
	for _, opt := range opts {
		opt(&reqOpts)
	}
	return &Requester{
		service:     service,
		rQueue:      rQueue,
		storage:     storage,
		opts:        reqOpts,
		drainSignal: make(chan struct{}, 2),
	}
}

// Requester handles requesting packets.
type Requester struct {
	running     bool
	service     *Service
	rQueue      RequestQueue
	storage     *storage.Storage
	opts        RequesterOptions
	backPFuncs  []RequestBackPressureFunc
	drainSignal chan struct{}
}

// RunRequestQueueDrainer runs the RequestQueue drainer.
func (r *Requester) RunRequestQueueDrainer(shutdownSignal <-chan struct{}) {
	r.running = true
	for {
		select {
		case <-shutdownSignal:
			return
		case <-r.drainSignal:

			// drain request queue
			for request := r.rQueue.Next(); r != nil; request = r.rQueue.Next() {

				requested := false
				r.service.ForEach(func(proto *Protocol) bool {
					// we only send a request message if the peer actually has the data
					// (r.MilestoneIndex > PrunedMilestoneIndex && r.MilestoneIndex <= SolidMilestoneIndex)
					if !proto.HasDataForMilestone(request.MilestoneIndex) {
						return true
					}

					proto.SendMessageRequest(request.MessageID)
					requested = true
					return false
				})

				if !requested {
					// we have no neighbor that has the data for sure,
					// so we ask all neighbors that could have the data
					// (r.MilestoneIndex > PrunedMilestoneIndex && r.MilestoneIndex <= LatestMilestoneIndex)
					r.service.ForEach(func(proto *Protocol) bool {
						// we only send a request message if the peer could have the data
						if !proto.CouldHaveDataForMilestone(request.MilestoneIndex) {
							return true
						}

						proto.SendMessageRequest(request.MessageID)
						return true
					})
				}
			}
		}
	}
}

// RunPendingRequestEnqueuer runs the loop to periodically re-request pending requests from the RequestQueue.
func (r *Requester) RunPendingRequestEnqueuer(shutdownSignal <-chan struct{}) {
	r.running = true
	reEnqueueTicker := time.NewTicker(r.opts.PendingRequestReEnqueueInterval)
reEnqueueLoop:
	for {
		select {
		case <-shutdownSignal:
			return
		case <-reEnqueueTicker.C:

			// check whether we should hold off requesting more data
			// if the node is currently under a lot of load
			if r.checkBackPressureFunctions() {
				continue reEnqueueLoop
			}

			// always fire the signal if something is in the queue, otherwise the sting request is not kicking in
			if queued := r.rQueue.EnqueuePending(r.opts.DiscardRequestsOlderThan); queued > 0 {
				select {
				case r.drainSignal <- struct{}{}:
				default:
				}
			}
		}
	}
}

// adds the request to the request queue and signals the request drainer to drain it.
func (r *Requester) enqueueAndSignal(request *Request) bool {
	if !r.rQueue.Enqueue(request) {
		return false
	}

	// signal requester
	select {
	case r.drainSignal <- struct{}{}:
	default:
		// if the signal queue is full, there's no need to block until it becomes empty
		// as the requester will drain everything present in the queue
	}
	return true
}

// checks whether any back pressure function is signaling congestion.
func (r *Requester) checkBackPressureFunctions() bool {
	for _, f := range r.backPFuncs {
		if f() {
			return true
		}
	}
	return false
}

// AddBackPressureFunc adds a RequestBackPressureFunc.
// This function can bbe called multiple times to add additional RequestBackPressureFunc.
// Calling this function after any Requester worker has been started results in a panic.
func (r *Requester) AddBackPressureFunc(pressureFunc RequestBackPressureFunc) {
	if r.running {
		panic("back pressure functions can only be added before the requester is started")
	}
	r.backPFuncs = append(r.backPFuncs, pressureFunc)
}

// Request enqueues a request to the request queue for the given message if it isn't a solid entry point
// and is not contained in the database already.
func (r *Requester) Request(messageID *hornet.MessageID, msIndex milestone.Index, preventDiscard ...bool) bool {
	if r.storage.SolidEntryPointsContain(messageID) {
		return false
	}

	if r.storage.ContainsMessage(messageID) {
		return false
	}

	request := &Request{MessageID: messageID, MilestoneIndex: msIndex}
	if len(preventDiscard) > 0 {
		request.PreventDiscard = preventDiscard[0]
	}

	return r.enqueueAndSignal(request)
}

// RequestMultiple works like Request but takes multiple message IDs.
func (r *Requester) RequestMultiple(messageIDs hornet.MessageIDs, msIndex milestone.Index, preventDiscard ...bool) int {
	requested := 0
	for _, messageID := range messageIDs {
		if r.Request(messageID, msIndex, preventDiscard...) {
			requested++
		}
	}
	return requested
}

// RequestParents enqueues requests for the parents of the given message to the request queue, if the
// given message is not a solid entry point and neither its parents are and also not in the database.
func (r *Requester) RequestParents(cachedMsg *storage.CachedMessage, msIndex milestone.Index, preventDiscard ...bool) {
	cachedMsg.ConsumeMetadata(func(metadata *storage.MessageMetadata) {
		messageID := metadata.GetMessageID()

		if r.storage.SolidEntryPointsContain(messageID) {
			return
		}

		r.Request(metadata.GetParent1MessageID(), msIndex, preventDiscard...)
		if *metadata.GetParent1MessageID() != *metadata.GetParent2MessageID() {
			r.Request(metadata.GetParent2MessageID(), msIndex, preventDiscard...)
		}
	})
}

// RequestMilestoneParents enqueues requests for the parents of the given milestone to the request queue,
// if the parents are not solid entry points and not already in the database.
func (r *Requester) RequestMilestoneParents(cachedMilestone *storage.CachedMilestone) bool {
	defer cachedMilestone.Release(true) // message -1

	msIndex := cachedMilestone.GetMilestone().Index

	cachedMilestoneMsgMeta := r.storage.GetCachedMessageMetadataOrNil(cachedMilestone.GetMilestone().MessageID) // meta +1
	if cachedMilestoneMsgMeta == nil {
		panic("milestone metadata doesn't exist")
	}
	defer cachedMilestoneMsgMeta.Release(true) // meta -1

	txMeta := cachedMilestoneMsgMeta.GetMetadata()
	enqueued := r.Request(txMeta.GetParent1MessageID(), msIndex, true)
	if *txMeta.GetParent1MessageID() != *txMeta.GetParent2MessageID() {
		enqueuedTwo := r.Request(txMeta.GetParent2MessageID(), msIndex, true)
		if !enqueued && enqueuedTwo {
			enqueued = true
		}
	}

	return enqueued
}

// of a given milestone and requests each missing parent. As a special property, invocations
// MemoizedRequestMissingMilestoneParents returns a function which traverses the parents
// of the yielded function share the same 'already traversed' set to circumvent requesting
// the same parents multiple times.
func (r *Requester) MemoizedRequestMissingMilestoneParents(preventDiscard ...bool) func(ms milestone.Index) {
	traversed := map[string]struct{}{}
	return func(ms milestone.Index) {

		cachedMs := r.storage.GetCachedMilestoneOrNil(ms) // milestone +1
		if cachedMs == nil {
			panic(fmt.Sprintf("milestone %d wasn't found", ms))
		}

		milestoneMessageID := cachedMs.GetMilestone().MessageID
		cachedMs.Release(true) // message -1

		_ = dag.TraverseParents(r.storage, milestoneMessageID,
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
				r.Request(parentMessageID, ms, preventDiscard...)
				return nil
			},
			// called on solid entry points
			// Ignore solid entry points (snapshot milestone included)
			nil,
			false, nil)
	}
}
