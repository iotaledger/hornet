package gossip

import (
	"container/heap"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/atomic"

	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	ErrUnknownRequestType = errors.New("unknown request type")
)

// RequestType is the type of request.
type RequestType int

const (
	RequestTypeBlockID RequestType = iota
	RequestTypeMilestoneIndex
)

func getRequestMapKey(data interface{}) string {
	switch value := data.(type) {
	case iotago.BlockID:
		return string(value[:])

	case iotago.MilestoneIndex:
		return strconv.Itoa(int(value))

	case *Request:
		return value.MapKey()

	default:
		panic(ErrUnknownRequestType)
	}
}

// RequestQueue implements a queue which contains requests for needed data.
type RequestQueue interface {
	// Next returns the next request to send, pops it from the queue and marks it as pending.
	Next() *Request
	// Peek returns the next request to send without popping it from the queue.
	Peek() *Request
	// Enqueue enqueues the given request if it isn't already queued or pending.
	Enqueue(*Request) (enqueued bool)
	// IsQueued tells whether a given request for the given data is queued.
	IsQueued(data interface{}) bool
	// IsPending tells whether a given request was popped from the queue and is now pending.
	IsPending(data interface{}) bool
	// IsProcessing tells whether a given request was popped from the queue, received and is now processing.
	IsProcessing(data interface{}) bool
	// Received marks a request as received and thereby removes it from the pending set.
	// It is added to the processing set.
	// Returns the origin request which was pending or nil if the data was not requested.
	Received(data interface{}) *Request
	// Processed marks a request as fulfilled and thereby removes it from the processing set.
	// Returns the origin request which was processing or nil if the data was not requested.
	Processed(data interface{}) *Request
	// EnqueuePending enqueues all pending requests back into the queue.
	// It also discards requests in the pending set of which their enqueue time is over the given delta threshold.
	// If discardOlderThan is zero, no requests are discarded.
	EnqueuePending(discardOlderThan time.Duration) (queued int)
	// Size returns the size of currently queued, requested/pending and processing requests.
	Size() (queued int, pending int, processing int)
	// Empty tells whether the queue has no queued and pending requests.
	Empty() bool
	// Requests returns a snapshot of all queued, pending and processing requests in the queue.
	Requests() (queued []*Request, pending []*Request, processing []*Request)
	// AvgLatency returns the average latency of enqueueing and then receiving a request.
	AvgLatency() int64
	// Filter adds the given filter function to the queue. Passing nil resets the current one.
	// Setting a filter automatically clears all queued and pending requests which do not fulfill
	// the filter criteria.
	Filter(f FilterFunc)
}

// FilterFunc is a function which determines whether a request should be enqueued or not.
type FilterFunc func(request *Request) bool

const DefaultLatencyResolution = 100

// NewRequestQueue creates a new RequestQueue where request are prioritized over their milestone index (lower = higher priority).
func NewRequestQueue(latencyResolution ...int32) RequestQueue {
	q := &priorityqueue{
		queue:      &requestHeap{},
		queued:     make(map[string]*Request),
		pending:    make(map[string]*Request),
		processing: make(map[string]*Request),
	}
	if len(latencyResolution) == 0 {
		q.latencyResolution = DefaultLatencyResolution
	}
	heap.Init(q)

	return q
}

// Request is a request for a particular block.
type Request struct {
	// The type of the request.
	RequestType RequestType
	// The BlockID of the block to request.
	BlockID iotago.BlockID
	// The milestone index under which this request is linked.
	MilestoneIndex iotago.MilestoneIndex
	// Tells the request queue to not remove this request if the enqueue time is
	// over the given threshold.
	PreventDiscard bool
	// the time at which this request was first enqueued.
	// do not modify this time
	EnqueueTime time.Time
}

// NewBlockIDRequest creates a new block request for a specific blockID.
func NewBlockIDRequest(blockID iotago.BlockID, msIndex iotago.MilestoneIndex) *Request {
	return &Request{RequestType: RequestTypeBlockID, BlockID: blockID, MilestoneIndex: msIndex}
}

// NewMilestoneIndexRequest creates a new block request for a specific milestone index.
func NewMilestoneIndexRequest(msIndex iotago.MilestoneIndex) *Request {
	return &Request{RequestType: RequestTypeMilestoneIndex, MilestoneIndex: msIndex}
}

func (r *Request) MapKey() string {
	switch r.RequestType {
	case RequestTypeBlockID:
		return string(r.BlockID[:])
	case RequestTypeMilestoneIndex:
		return strconv.Itoa(int(r.MilestoneIndex))
	default:
		panic(ErrUnknownRequestType)
	}
}

type Requests []*Request

// HasRequest returns true if Requests contains a Request.
func (r Requests) HasRequest() bool {
	return len(r) > 0
}

// implements a priority queue where requests with the lowest milestone index are popped first.
type priorityqueue struct {
	// must be first field for 64-bit alignment.
	// otherwise it crashes under 32-bit ARM systems
	// see: https://golang.org/pkg/sync/atomic/#pkg-note-BUG
	avgLatency        atomic.Int64
	queue             *requestHeap
	queued            map[string]*Request
	pending           map[string]*Request
	processing        map[string]*Request
	latencyResolution int64
	latencySum        int64
	latencyEntries    int64
	filter            FilterFunc
	sync.RWMutex
}

func (pq *priorityqueue) Next() (request *Request) {
	pq.Lock()
	defer pq.Unlock()

	// Pop() doesn't gracefully handle empty queues, so we check it ourselves
	if pq.Len() == 0 {
		return nil
	}

	next := heap.Pop(pq)
	if next == nil {
		return nil
	}

	nextRequest, ok := next.(*Request)
	if !ok {
		panic(fmt.Sprintf("invalid type: expected *Request, got %T", next))
	}

	return nextRequest
}

func (pq *priorityqueue) Enqueue(request *Request) bool {
	pq.Lock()
	defer pq.Unlock()

	requestMapKey := request.MapKey()

	if _, queued := pq.queued[requestMapKey]; queued {
		// do not enqueue because it was already queued.
		return false
	}
	if _, pending := pq.pending[requestMapKey]; pending {
		// do not enqueue because it was already pending.
		return false
	}
	if _, processing := pq.processing[requestMapKey]; processing {
		// do not enqueue because it was already processing.
		return false
	}
	if pq.filter != nil && !pq.filter(request) {
		// do not enqueue because it doesn't match the filter.
		return false
	}
	request.EnqueueTime = time.Now()
	heap.Push(pq, request)

	return true
}

func (pq *priorityqueue) IsQueued(data interface{}) bool {
	pq.RLock()
	defer pq.RUnlock()

	_, k := pq.queued[getRequestMapKey(data)]

	return k
}

func (pq *priorityqueue) IsPending(data interface{}) bool {
	pq.RLock()
	defer pq.RUnlock()

	_, k := pq.pending[getRequestMapKey(data)]

	return k
}

func (pq *priorityqueue) IsProcessing(data interface{}) bool {
	pq.RLock()
	defer pq.RUnlock()

	_, k := pq.processing[getRequestMapKey(data)]

	return k
}

func (pq *priorityqueue) Received(data interface{}) *Request {
	pq.Lock()
	defer pq.Unlock()

	requestMapKey := getRequestMapKey(data)

	if req, wasPending := pq.pending[requestMapKey]; wasPending {
		pq.latencySum += time.Since(req.EnqueueTime).Milliseconds()
		pq.latencyEntries++
		if pq.latencyEntries == pq.latencyResolution {
			pq.avgLatency.Store(pq.latencySum / pq.latencyResolution)
			pq.latencySum = 0
			pq.latencyEntries = 0
		}
		delete(pq.pending, requestMapKey)
		if len(pq.pending) == 0 {
			pq.latencySum = 0
			pq.avgLatency.Store(0)
		}

		// add the request to processing
		pq.processing[requestMapKey] = req

		return req
	}

	// check if the request is in the queue (was enqueued again after request)
	if req, wasQueued := pq.queued[requestMapKey]; wasQueued {
		// delete it from queued, it will be cleaned up from the heap with pop
		delete(pq.queued, requestMapKey)

		// add the request to processing
		pq.processing[requestMapKey] = req

		return req
	}

	return nil
}

func (pq *priorityqueue) Processed(data interface{}) *Request {
	pq.Lock()
	defer pq.Unlock()

	requestMapKey := getRequestMapKey(data)
	req, wasProcessing := pq.processing[requestMapKey]
	if wasProcessing {
		delete(pq.processing, requestMapKey)
	}

	return req
}

func (pq *priorityqueue) EnqueuePending(discardOlderThan time.Duration) int {
	pq.Lock()
	defer pq.Unlock()

	enqueued := len(pq.pending)
	s := time.Now()
	for k, v := range pq.pending {
		value := v

		if pq.filter != nil && !pq.filter(value) {
			// discard request from the queue, because it didn't match the filter
			delete(pq.pending, k)
			enqueued--

			continue
		}
		if discardOlderThan == 0 || value.PreventDiscard || s.Sub(value.EnqueueTime) < discardOlderThan {
			// no need to examine the queued set
			// as addition and removal are synced over Push and Pops
			heap.Push(pq, value)

			continue
		}
		// discard request from the queue
		delete(pq.pending, k)
		enqueued--
	}

	return enqueued
}

func (pq *priorityqueue) Size() (int, int, int) {
	pq.RLock()
	defer pq.RUnlock()

	x := len(pq.queued)
	y := len(pq.pending)
	z := len(pq.processing)

	return x, y, z
}

func (pq *priorityqueue) Empty() bool {
	pq.RLock()
	defer pq.RUnlock()

	empty := len(pq.queued) == 0 && len(pq.pending) == 0 && len(pq.processing) == 0

	return empty
}

func (pq *priorityqueue) AvgLatency() int64 {
	return pq.avgLatency.Load()
}

func (pq *priorityqueue) Requests() (queued []*Request, pending []*Request, processing []*Request) {
	pq.RLock()
	defer pq.RUnlock()

	queued = make([]*Request, len(pq.queued))
	var i int
	for _, v := range pq.queued {
		value := v
		queued[i] = value
		i++
	}
	pending = make([]*Request, len(pq.pending))
	var j int
	for _, v := range pq.pending {
		value := v
		pending[j] = value
		j++
	}
	processing = make([]*Request, len(pq.processing))
	var k int
	for _, v := range pq.processing {
		value := v
		processing[k] = value
		k++
	}

	return queued, pending, processing
}

func (pq *priorityqueue) Filter(f FilterFunc) {
	pq.Lock()
	defer pq.Unlock()

	if f != nil {
		filteredQueue := requestHeap{}
		for _, v := range pq.queued {
			value := v
			if !f(value) {
				// discard request from the queue, because it didn't match the filter
				delete(pq.queued, value.MapKey())

				continue
			}
			filteredQueue.Push(value)
		}
		pq.queue = &filteredQueue

		for k, v := range pq.pending {
			value := v
			if !f(value) {
				// discard request from the queue, because it didn't match the filter
				delete(pq.pending, k)
			}
		}
	}
	pq.filter = f
}

func (pq *priorityqueue) Len() int {
	return pq.queue.Len()
}

func (pq *priorityqueue) Less(i, j int) bool {
	return pq.queue.Less(i, j)
}

func (pq *priorityqueue) Swap(i, j int) {
	pq.queue.Swap(i, j)
}

func (pq *priorityqueue) Push(x interface{}) {
	pq.queue.Push(x)

	request, ok := x.(*Request)
	if !ok {
		panic(fmt.Sprintf("invalid type: expected *Request, got %T", x))
	}

	requestMapKey := request.MapKey()

	// mark as queued and remove from pending
	delete(pq.pending, requestMapKey)
	pq.queued[requestMapKey] = request
}

func (pq *priorityqueue) Pop() interface{} {

	for pq.queue.Len() > 0 {
		x := pq.queue.Pop()

		request, ok := x.(*Request)
		if !ok {
			panic(fmt.Sprintf("invalid type: expected *Request, got %T", x))
		}

		requestMapKey := request.MapKey()
		if _, queued := pq.queued[requestMapKey]; !queued {
			// the request is not queued anymore
			// => remove it from the heap and jump to the next entry
			continue
		}

		// mark as pending and remove from queued
		delete(pq.queued, requestMapKey)
		pq.pending[requestMapKey] = request

		return request
	}

	return nil
}

func (pq *priorityqueue) Peek() *Request {
	pq.Lock()
	defer pq.Unlock()

	return pq.queue.Peek()
}

type requestHeap []*Request

func (rh requestHeap) Len() int {
	return len(rh)
}

func (rh requestHeap) Less(i, j int) bool {
	// requests for older milestones (lower number) have priority
	return rh[i].MilestoneIndex <= rh[j].MilestoneIndex
}

func (rh requestHeap) Swap(i, j int) {
	rh[i], rh[j] = rh[j], rh[i]
}

func (rh *requestHeap) Push(x any) {
	// Push uses pointer receivers because it modifies the slice's length,
	// not just its contents.

	request, ok := x.(*Request)
	if !ok {
		panic(fmt.Sprintf("invalid type: expected *Request, got %T", x))
	}

	*rh = append(*rh, request)
}

func (rh *requestHeap) Pop() interface{} {
	// Pop uses pointer receivers because it modifies the slice's length,
	// not just its contents.
	old := *rh
	n := len(old)
	if n == 0 {
		// queue is empty
		return nil
	}
	x := old[n-1]
	old[n-1] = nil // avoid memory leak
	*rh = old[0 : n-1]

	return x
}

func (rh *requestHeap) Peek() *Request {
	if len(*rh) == 0 {
		return nil
	}

	return (*rh)[0]
}
