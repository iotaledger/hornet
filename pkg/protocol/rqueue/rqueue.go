package rqueue

import (
	"container/heap"
	"sync"
	"time"

	"github.com/iotaledger/iota.go/trinary"
	"go.uber.org/atomic"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

// Queue implements a queue which contains requests for needed data.
type Queue interface {
	// Next returns the next request to send, pops it from the queue and marks it as pending.
	Next() *Request
	// Peek returns the next request to send without popping it from the queue.
	Peek() *Request
	// Enqueue enqueues the given request if it isn't already queued or pending.
	Enqueue(*Request) (enqueued bool)
	// IsQueued tells whether a given request for the given transaction hash is queued.
	IsQueued(hash trinary.Hash) bool
	// IsPending tells whether a given request was popped from the queue and is now pending.
	IsPending(hash trinary.Hash) bool
	// IsProcessing tells whether a given request was popped from the queue, received and is now processing.
	IsProcessing(hash trinary.Hash) bool
	// Received marks a request as received and thereby removes it from the pending set.
	// It is added to the processing set.
	// Returns the origin request which was pending or nil if the hash was not requested.
	Received(hash trinary.Hash) *Request
	// Processed marks a request as fulfilled and thereby removes it from the processing set.
	// Returns the origin request which was pending or nil if the hash was not requested.
	Processed(hash trinary.Hash) *Request
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
}

const (
	DefaultLatencyResolution = 100
)

// New creates a new Queue where request are prioritized via their time of the last request (oldest have higher prio)
func New(latencyResolution ...int32) Queue {
	q := &priorityqueue{
		queue:      make([]*Request, 0),
		queued:     make(map[string]struct{}),
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
	Hash trinary.Hash
	// The byte encoded hash of the transaction to request.
	HashBytesEncoded []byte
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
	// the time at which this request was requested the last time.
	LastRequestTime time.Time
}

// implements a priority queue where requests with the oldest request time are popped first.
type priorityqueue struct {
	// must be first field for 64-bit alignment.
	// otherwise it crashes under 32-bit ARM systems
	// see: https://golang.org/pkg/sync/atomic/#pkg-note-BUG
	avgLatency        atomic.Int64
	queue             []*Request
	queued            map[string]struct{}
	pending           map[string]*Request
	processing        map[string]*Request
	latencyResolution int64
	latencySum        int64
	latencyEntries    int64
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
	if _, queued := pq.queued[r.Hash]; queued {
		return false
	}
	if _, pending := pq.pending[r.Hash]; pending {
		return false
	}
	if _, processing := pq.processing[r.Hash]; processing {
		return false
	}
	r.EnqueueTime = time.Now()
	heap.Push(pq, r)
	return true
}

func (pq *priorityqueue) IsQueued(hash trinary.Hash) bool {
	pq.RLock()
	_, k := pq.queued[hash]
	pq.RUnlock()
	return k
}

func (pq *priorityqueue) IsPending(hash trinary.Hash) bool {
	pq.RLock()
	_, k := pq.pending[hash]
	pq.RUnlock()
	return k
}

func (pq *priorityqueue) IsProcessing(hash trinary.Hash) bool {
	pq.RLock()
	_, k := pq.processing[hash]
	pq.RUnlock()
	return k
}

func (pq *priorityqueue) Received(hash trinary.Hash) *Request {
	pq.Lock()
	req, wasPending := pq.pending[hash]
	if wasPending {
		pq.latencySum += time.Since(req.EnqueueTime).Milliseconds()
		pq.latencyEntries++
		if pq.latencyEntries == pq.latencyResolution {
			pq.avgLatency.Store(pq.latencySum / pq.latencyResolution)
			pq.latencySum = 0
			pq.latencyEntries = 0
		}
		delete(pq.pending, hash)
		if len(pq.pending) == 0 {
			pq.latencySum = 0
			pq.avgLatency.Store(0)
		}

		// Add the request to processing
		pq.processing[hash] = req
	}
	pq.Unlock()
	return req
}

func (pq *priorityqueue) Processed(hash trinary.Hash) *Request {
	pq.Lock()
	req, wasProcessing := pq.processing[hash]
	if wasProcessing {
		delete(pq.processing, hash)
	}
	pq.Unlock()
	return req
}

func (pq *priorityqueue) EnqueuePending(discardOlderThan time.Duration) int {
	pq.Lock()
	defer pq.Unlock()
	enqueued := len(pq.pending)
	s := time.Now()
	for _, v := range pq.pending {
		if discardOlderThan == 0 || v.PreventDiscard || s.Sub(v.EnqueueTime) < discardOlderThan {
			// no need to examine the queued set
			// as addition and removal are synced over Push and Pops
			heap.Push(pq, v)
			continue
		}
		// discard request from the queue
		delete(pq.pending, v.Hash)
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
	for i := range pq.queue {
		queued[i] = pq.queue[i]
	}
	pending = make([]*Request, len(pq.pending))
	var i int
	for _, v := range pq.pending {
		pending[i] = v
		i++
	}
	processing = make([]*Request, len(pq.processing))
	var j int
	for _, v := range pq.processing {
		processing[j] = v
		j++
	}
	return queued, pending, processing
}

func (pq *priorityqueue) Len() int { return len(pq.queue) }

func (pq *priorityqueue) Less(i, j int) bool {
	// older requests have priority
	return pq.queue[i].LastRequestTime.Before(pq.queue[j].LastRequestTime)
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
	delete(pq.pending, r.Hash)
	pq.queued[r.Hash] = struct{}{}
}

func (pq *priorityqueue) Pop() interface{} {
	old := pq.queue
	n := len(pq.queue)
	r := old[n-1]
	old[n-1] = nil // avoid memory leak
	pq.queue = old[0 : n-1]

	// mark as pending and remove from queued
	delete(pq.queued, r.Hash)
	pq.pending[r.Hash] = r

	// Set the LastRequestTime, since pop is used before requesting it
	r.LastRequestTime = time.Now()
	return r
}

func (pq *priorityqueue) Peek() *Request {
	pq.RWMutex.Lock()
	defer pq.RWMutex.Unlock()
	if len(pq.queue) == 0 {
		return nil
	}
	return pq.queue[len(pq.queue)-1]
}
