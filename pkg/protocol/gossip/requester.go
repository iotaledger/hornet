package gossip

import (
	"time"

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

// applies the given RequesterOption.
func (ro *RequesterOptions) apply(opts ...RequesterOption) {
	for _, opt := range opts {
		opt(ro)
	}
}

// RequestBackPressureFunc is a function which tells the Requester
// to stop requesting more data.
type RequestBackPressureFunc func() bool

var defaultRequesterOpts = []RequesterOption{
	WithRequesterDiscardRequestsOlderThan(10 * time.Second),
	WithRequesterPendingRequestReEnqueueInterval(5 * time.Second),
}

// RequesterOption is a function which sets an option on a RequesterOptions instance.
type RequesterOption func(options *RequesterOptions)

// WithRequesterDiscardRequestsOlderThan sets the threshold for the max age of requests.
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

// Requester handles requesting packets.
type Requester struct {
	storage *storage.Storage
	service *Service
	rQueue  RequestQueue
	opts    *RequesterOptions

	running     bool
	backPFuncs  []RequestBackPressureFunc
	drainSignal chan struct{}
}

// NewRequester creates a new Requester.
func NewRequester(
	dbStorage *storage.Storage,
	service *Service,
	rQueue RequestQueue,
	opts ...RequesterOption) *Requester {

	reqOpts := &RequesterOptions{}
	reqOpts.apply(defaultRequesterOpts...)
	reqOpts.apply(opts...)

	return &Requester{
		storage:     dbStorage,
		service:     service,
		rQueue:      rQueue,
		opts:        reqOpts,
		drainSignal: make(chan struct{}, 2),
	}
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
			for request := r.rQueue.Next(); request != nil; request = r.rQueue.Next() {

				sendRequest := func(request *Request, proto *Protocol) {
					switch request.RequestType {
					case RequestTypeMessageID:
						proto.SendMessageRequest(request.MessageID)
					case RequestTypeMilestoneIndex:
						proto.SendMilestoneRequest(request.MilestoneIndex)
					default:
						panic(ErrUnknownRequestType)
					}
				}

				requested := false
				r.service.ForEach(func(proto *Protocol) bool {
					// we only send a request message if the peer actually has the data
					// (r.MilestoneIndex > PrunedMilestoneIndex && r.MilestoneIndex <= SolidMilestoneIndex)
					if !proto.HasDataForMilestone(request.MilestoneIndex) {
						return true
					}

					sendRequest(request, proto)
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

						sendRequest(request, proto)
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
// This function can be called multiple times to add additional RequestBackPressureFunc.
// Calling this function after any Requester worker has been started results in a panic.
func (r *Requester) AddBackPressureFunc(pressureFunc RequestBackPressureFunc) {
	if r.running {
		panic("back pressure functions can only be added before the requester is started")
	}
	r.backPFuncs = append(r.backPFuncs, pressureFunc)
}

// Request enqueues a request to the request queue for the given message if it isn't a solid entry point
// and is not contained in the database already.
func (r *Requester) Request(data interface{}, msIndex milestone.Index, preventDiscard ...bool) bool {

	var request *Request

	switch value := data.(type) {
	case hornet.MessageID:
		messageID := value
		if r.storage.SolidEntryPointsContain(messageID) {
			return false
		}
		if r.storage.ContainsMessage(messageID) {
			return false
		}
		request = NewMessageIDRequest(messageID, msIndex)

	case milestone.Index:
		msIndex := value
		if r.storage.ContainsMilestone(msIndex) {
			return false
		}
		request = NewMilestoneIndexRequest(msIndex)

	default:
		panic(ErrUnknownRequestType)
	}

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
		messageID := metadata.MessageID()

		if r.storage.SolidEntryPointsContain(messageID) {
			return
		}

		for _, parent := range metadata.Parents() {
			r.Request(parent, msIndex, preventDiscard...)
		}
	})
}

// RequestMilestoneParents enqueues requests for the parents of the given milestone to the request queue,
// if the parents are not solid entry points and not already in the database.
func (r *Requester) RequestMilestoneParents(cachedMilestone *storage.CachedMilestone) bool {
	defer cachedMilestone.Release(true) // message -1

	msIndex := cachedMilestone.Milestone().Index

	cachedMilestoneMsgMeta := r.storage.CachedMessageMetadataOrNil(cachedMilestone.Milestone().MessageID) // meta +1
	if cachedMilestoneMsgMeta == nil {
		panic("milestone metadata doesn't exist")
	}
	defer cachedMilestoneMsgMeta.Release(true) // meta -1

	txMeta := cachedMilestoneMsgMeta.Metadata()

	enqueued := false
	for _, parent := range txMeta.Parents() {
		if r.Request(parent, msIndex, true) {
			enqueued = true
		}
	}

	return enqueued
}
