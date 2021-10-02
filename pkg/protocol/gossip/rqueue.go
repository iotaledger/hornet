package gossip

import (
	"container/heap"
	"strconv"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

var (
	ErrUnknownRequestType = errors.New("unknown request type")
)

// RequestType is the type of request.
type RequestType int

const (
	RequestTypeMessageID RequestType = iota
	RequestTypeMilestoneIndex
)

func getRequestMapKey(data interface{}) string {
	switch value := data.(type) {
	case hornet.MessageID:
		return value.ToMapKey()

	case milestone.Index:
		return value.String()

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
type FilterFunc func(r *Request) bool

const DefaultLatencyResolution = 100

// NewRequestQueue creates a new RequestQueue where request are prioritized over their milestone index (lower = higher priority).
func NewRequestQueue(latencyResolution ...int32) RequestQueue {
	q := &priorityqueue{
		queue:      make([]*Request, 0),
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

// Request is a request for a particular message.
type Request struct {
	// The type of the request.
	RequestType RequestType
	// The MessageID of the message to request.
	MessageID hornet.MessageID
	// The milestone index under which this request is linked.
	MilestoneIndex milestone.Index
	// internal to the priority queue
	index int
	// Tells the request queue to not remove this request if the enqueue time is
	// over the given threshold.
	PreventDiscard bool
	// the time at which this request was first enqueued.
	// do not modify this time
	EnqueueTime time.Time
}

// NewMessageIDRequest creates a new message request for a specific messageID.
func NewMessageIDRequest(messageID hornet.MessageID, msIndex milestone.Index) *Request {
	return &Request{RequestType: RequestTypeMessageID, MessageID: messageID, MilestoneIndex: msIndex}
}

// NewMilestoneIndexRequest creates a new message request for a specific milestone index
func NewMilestoneIndexRequest(msIndex milestone.Index) *Request {
	return &Request{RequestType: RequestTypeMilestoneIndex, MilestoneIndex: msIndex}
}

func (r *Request) MapKey() string {
	switch r.RequestType {
	case RequestTypeMessageID:
		return r.MessageID.ToMapKey()
	case RequestTypeMilestoneIndex:
		return strconv.Itoa(int(r.MilestoneIndex))
	default:
		panic(ErrUnknownRequestType)
	}
}

type Requests []*Request

func (r Requests) Requested() bool {
	return len(r) > 0
}

// implements a priority queue where requests with the lowest milestone index are popped first.
type priorityqueue struct {
	// must be first field for 64-bit alignment.
	// otherwise it crashes under 32-bit ARM systems
	// see: https://golang.org/pkg/sync/atomic/#pkg-note-BUG
	avgLatency        atomic.Int64
	queue             []*Request
	queued            map[string]*Request
	pending           map[string]*Request
	processing        map[string]*Request
	latencyResolution int64
	latencySum        int64
	latencyEntries    int64
	filter            FilterFunc
	sync.RWMutex
}

func (pq *priorityqueue) Next() (r *Request) {
	pq.Lock()
	defer pq.Unlock()

	// Pop() doesn't gracefully handle empty queues, so we check it ourselves
	if len(pq.queued) == 0 {
		return nil
	}
	return heap.Pop(pq).(*Request)
}

func (pq *priorityqueue) Enqueue(r *Request) bool {
	pq.Lock()
	defer pq.Unlock()

	requestMapKey := r.MapKey()

	if _, queued := pq.queued[requestMapKey]; queued {
		return false
	}
	if _, pending := pq.pending[requestMapKey]; pending {
		return false
	}
	if _, processing := pq.processing[requestMapKey]; processing {
		return false
	}
	if pq.filter != nil && !pq.filter(r) {
		return false
	}
	r.EnqueueTime = time.Now()
	heap.Push(pq, r)
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

	if len(pq.queued) != 0 {
		return len(pq.queued)
	}
	enqueued := len(pq.pending)
	s := time.Now()
	for k, v := range pq.pending {
		if pq.filter != nil && !pq.filter(v) {
			delete(pq.pending, k)
			enqueued--
			continue
		}
		if discardOlderThan == 0 || v.PreventDiscard || s.Sub(v.EnqueueTime) < discardOlderThan {
			// no need to examine the queued set
			// as addition and removal are synced over Push and Pops
			heap.Push(pq, v)
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
		queued[i] = v
		i++
	}
	pending = make([]*Request, len(pq.pending))
	var j int
	for _, v := range pq.pending {
		pending[j] = v
		j++
	}
	processing = make([]*Request, len(pq.processing))
	var k int
	for _, v := range pq.processing {
		processing[k] = v
		k++
	}
	return queued, pending, processing
}

func (pq *priorityqueue) Filter(f FilterFunc) {
	pq.Lock()
	defer pq.Unlock()

	if f != nil {
		filteredQueue := make([]*Request, 0)
		for _, r := range pq.queued {
			if !f(r) {
				delete(pq.queued, r.MapKey())
				continue
			}
			filteredQueue = append(filteredQueue, r)
		}
		pq.queue = filteredQueue
		for k, v := range pq.pending {
			if !f(v) {
				delete(pq.pending, k)
			}
		}
	}
	pq.filter = f
}

func (pq *priorityqueue) Len() int { return len(pq.queue) }

func (pq *priorityqueue) Less(i, j int) bool {
	// requests for older milestones (lower number) have priority
	return pq.queue[i].MilestoneIndex < pq.queue[j].MilestoneIndex
}

func (pq *priorityqueue) Swap(i, j int) {
	pq.queue[i], pq.queue[j] = pq.queue[j], pq.queue[i]
	pq.queue[i].index = i
	pq.queue[j].index = j
}

func (pq *priorityqueue) Push(x interface{}) {
	r := x.(*Request)
	pq.queue = append(pq.queue, r)

	requestMapKey := r.MapKey()

	// mark as queued and remove from pending
	delete(pq.pending, requestMapKey)
	pq.queued[requestMapKey] = r
}

func (pq *priorityqueue) Pop() interface{} {
	for {
		old := pq.queue
		n := len(pq.queue)
		if n == 0 {
			// queue is empty
			return nil
		}
		r := old[n-1]
		old[n-1] = nil // avoid memory leak
		pq.queue = old[0 : n-1]

		requestMapKey := r.MapKey()
		if _, queued := pq.queued[requestMapKey]; !queued {
			// the request is not queued anymore
			// => remove it from the heap and jump to the next entry
			continue
		}

		// mark as pending and remove from queued
		delete(pq.queued, requestMapKey)
		pq.pending[requestMapKey] = r
		return r
	}
}

func (pq *priorityqueue) Peek() *Request {
	pq.Lock()
	defer pq.Unlock()

	if len(pq.queued) == 0 {
		return nil
	}
	return pq.queue[len(pq.queue)-1]
}
