package rqueue

import (
	"container/heap"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

// Queue implements a queue which contains requests for needed data.
type Queue interface {
	// Lock acquires the queue mutex.
	Lock()
	// Unlock releases the queue mutex.
	Unlock()
	// NextWithoutLocking returns the next request to send, pops it from the queue and marks it as pending (without locking the queue).
	NextWithoutLocking() *Request
	// Next returns the next request to send, pops it from the queue and marks it as pending.
	Next() *Request
	// PeekWithoutLocking returns the next request to send without popping it from the queue (without locking the queue).
	PeekWithoutLocking() *Request
	// Peek returns the next request to send without popping it from the queue.
	Peek() *Request
	// Enqueue enqueues the given request if it isn't already queued or pending.
	Enqueue(*Request) (enqueued bool)
	// IsQueued tells whether a given request for the given transaction hash is queued.
	IsQueued(hash hornet.Hash) bool
	// IsPending tells whether a given request was popped from the queue and is now pending.
	IsPending(hash hornet.Hash) bool
	// IsProcessing tells whether a given request was popped from the queue, received and is now processing.
	IsProcessing(hash hornet.Hash) bool
	// Received marks a request as received and thereby removes it from the pending set.
	// It is added to the processing set.
	// Returns the origin request which was pending or nil if the hash was not requested.
	Received(hash hornet.Hash) *Request
	// Processed marks a request as fulfilled and thereby removes it from the processing set.
	// Returns the origin request which was pending or nil if the hash was not requested.
	Processed(hash hornet.Hash) *Request
	// EnqueuePending enqueues all pending requests back into the queue.
	// It also discards requests in the pending set of which their enqueue time is over the given delta threshold.
	// If discardOlderThan is zero, no requests are discarded.
	EnqueuePending(discardOlderThan time.Duration) (enqueued int)
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

// New creates a new Queue where request are prioritized over their milestone index (lower = higher priority).
func New(latencyResolution ...int32) Queue {
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

// Request is a request for a particular transaction.
type Request struct {
	// The hash of the transaction to request.
	Hash hornet.Hash
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

func (pq *priorityqueue) NextWithoutLocking() *Request {
	// Pop() doesn't gracefully handle empty queues, so we check it ourselves
	if len(pq.queued) == 0 {
		return nil
	}
	return heap.Pop(pq).(*Request)
}

func (pq *priorityqueue) Next() (r *Request) {
	pq.Lock()
	defer pq.Unlock()
	return pq.NextWithoutLocking()
}

func (pq *priorityqueue) Enqueue(r *Request) bool {
	pq.Lock()
	defer pq.Unlock()
	if _, queued := pq.queued[string(r.Hash)]; queued {
		return false
	}
	if _, pending := pq.pending[string(r.Hash)]; pending {
		return false
	}
	if _, processing := pq.processing[string(r.Hash)]; processing {
		return false
	}
	if pq.filter != nil && !pq.filter(r) {
		return false
	}
	r.EnqueueTime = time.Now()
	heap.Push(pq, r)
	return true
}

func (pq *priorityqueue) IsQueued(hash hornet.Hash) bool {
	pq.RLock()
	_, k := pq.queued[string(hash)]
	pq.RUnlock()
	return k
}

func (pq *priorityqueue) IsPending(hash hornet.Hash) bool {
	pq.RLock()
	_, k := pq.pending[string(hash)]
	pq.RUnlock()
	return k
}

func (pq *priorityqueue) IsProcessing(hash hornet.Hash) bool {
	pq.RLock()
	_, k := pq.processing[string(hash)]
	pq.RUnlock()
	return k
}

func (pq *priorityqueue) Received(hash hornet.Hash) *Request {
	pq.Lock()
	defer pq.Unlock()

	if req, wasPending := pq.pending[string(hash)]; wasPending {
		pq.latencySum += time.Since(req.EnqueueTime).Milliseconds()
		pq.latencyEntries++
		if pq.latencyEntries == pq.latencyResolution {
			pq.avgLatency.Store(pq.latencySum / pq.latencyResolution)
			pq.latencySum = 0
			pq.latencyEntries = 0
		}
		delete(pq.pending, string(hash))
		if len(pq.pending) == 0 {
			pq.latencySum = 0
			pq.avgLatency.Store(0)
		}

		// add the request to processing
		pq.processing[string(hash)] = req

		return req
	}

	// check if the request is in the queue (was enqueued again after request)
	return pq.queued[string(hash)]
}

func (pq *priorityqueue) Processed(hash hornet.Hash) *Request {
	pq.Lock()
	req, wasProcessing := pq.processing[string(hash)]
	if wasProcessing {
		delete(pq.processing, string(hash))
	}
	pq.Unlock()
	return req
}

func (pq *priorityqueue) EnqueuePending(discardOlderThan time.Duration) int {
	pq.Lock()
	defer pq.Unlock()
	if len(pq.queued) != 0 {
		return 0
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
	x := len(pq.queued)
	y := len(pq.pending)
	z := len(pq.processing)
	pq.RUnlock()
	return x, y, z
}

func (pq *priorityqueue) Empty() bool {
	pq.RLock()
	empty := len(pq.queued) == 0 && len(pq.pending) == 0 && len(pq.processing) == 0
	pq.RUnlock()
	return empty
}

func (pq *priorityqueue) AvgLatency() int64 {
	return pq.avgLatency.Load()
}

func (pq *priorityqueue) Requests() (queued []*Request, pending []*Request, processing []*Request) {
	pq.Lock()
	defer pq.Unlock()
	queued = make([]*Request, len(pq.queue))
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
		for _, r := range pq.queue {
			if !f(r) {
				delete(pq.queued, string(r.Hash))
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

	// mark as queued and remove from pending
	delete(pq.pending, string(r.Hash))
	pq.queued[string(r.Hash)] = r
}

func (pq *priorityqueue) Pop() interface{} {
	old := pq.queue
	n := len(pq.queue)
	r := old[n-1]
	old[n-1] = nil // avoid memory leak
	pq.queue = old[0 : n-1]

	// mark as pending and remove from queued
	delete(pq.queued, string(r.Hash))
	pq.pending[string(r.Hash)] = r
	return r
}

func (pq *priorityqueue) PeekWithoutLocking() *Request {
	if len(pq.queue) == 0 {
		return nil
	}
	return pq.queue[len(pq.queue)-1]
}

func (pq *priorityqueue) Peek() *Request {
	pq.RWMutex.Lock()
	defer pq.RWMutex.Unlock()
	return pq.PeekWithoutLocking()
}
