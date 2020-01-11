package queue

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/lru_cache"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/typeutils"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/profile"
)

const (
	RequestQueueTickerInterval = 2 * time.Second
)

type RequestQueue struct {
	syncutils.Mutex
	requestedCache *lru_cache.LRUCache
	lifo           []*request
	pending        []*request
	ticker         *time.Ticker
	tickerDone     chan bool
}

// Request struct
type DebugRequest struct {
	Hash        string `json:"hash"`
	IsReceived  bool   `json:"received"`
	IsProcessed bool   `json:"processed"`
	InCache     bool   `json:"inCache"`
	InPending   bool   `json:"inPending"`
	InLifo      bool   `json:"inLifo"`
	TxExists    bool   `json:"txExists"`
}

func NewRequestQueue() *RequestQueue {

	queue := &RequestQueue{
		requestedCache: lru_cache.NewLRUCache(profile.GetProfile().Caches.RequestQueue.Size),
		ticker:         time.NewTicker(RequestQueueTickerInterval),
		tickerDone:     make(chan bool),
	}

	go func(q *RequestQueue) {
		for {
			select {
			case <-q.tickerDone:
				return
			case <-q.ticker.C:
				q.retryPending()
			}
		}
	}(queue)

	return queue
}

func (s *RequestQueue) GetCache() *lru_cache.LRUCache {
	return s.requestedCache
}

func (s *RequestQueue) retryPending() {

	s.Lock()

	for _, r := range s.pending {
		if r.isReceived() == false {
			if !tangle.ContainsTransaction(r.hash) {
				// We haven't received any answer for this request, so re-add it to our lifo queue
				s.lifo = append(s.lifo, r)
			}
		}
	}

	s.pending = []*request{}
	s.Unlock()
}

func (s *RequestQueue) Stop() {

	s.ticker.Stop()
	s.tickerDone <- true
}

func (s *RequestQueue) GetNext() ([]byte, trinary.Hash, milestone_index.MilestoneIndex) {

	s.Lock()
	defer s.Unlock()

	length := len(s.lifo)
	if length > 0 {
		for i := length - 1; i >= 0; i-- {
			request := s.lifo[i]
			if request.isReceived() || request.isProcessed() {
				// Remove from lifo since we received an answer for the request
				s.lifo = append(s.lifo[:i], s.lifo[i+1:]...)
				continue
			}
			request.updateTimes()
			s.lifo = append(s.lifo[:i], s.lifo[i+1:]...)
			s.pending = append(s.pending, request)
			return request.bytes, request.hash, request.msIndex
		}
	}

	return nil, "", 0
}

func (s *RequestQueue) GetNextInRange(startIndex milestone_index.MilestoneIndex, endIndex milestone_index.MilestoneIndex) ([]byte, trinary.Hash, milestone_index.MilestoneIndex) {

	s.Lock()
	defer s.Unlock()

	length := len(s.lifo)
	if length > 0 {
		for i := length - 1; i >= 0; i-- {
			request := s.lifo[i]
			if request.isReceived() || request.isProcessed() {
				// Remove from lifo since we received an answer for the request
				s.lifo = append(s.lifo[:i], s.lifo[i+1:]...)
				continue
			} else if request.msIndex < startIndex || request.msIndex > endIndex {
				// Not in range, skip it
				continue
			}
			request.updateTimes()
			s.lifo = append(s.lifo[:i], s.lifo[i+1:]...)
			s.pending = append(s.pending, request)
			return request.bytes, request.hash, request.msIndex
		}
	}
	return nil, "", 0

}

func (s *RequestQueue) Contains(txHash trinary.Hash) (bool, milestone_index.MilestoneIndex) {
	r := s.requestedCache.Get(txHash)
	if typeutils.IsInterfaceNil(r) {
		return false, 0
	}
	request := r.(*request)
	return true, request.msIndex
}

func (s *RequestQueue) add(txHash trinary.Hash, ms milestone_index.MilestoneIndex, markRequested bool) bool {

	if len(txHash) == 0 {
		return false
	}

	if s.requestedCache.Contains(txHash) {
		return false
	}

	request := newRequest(txHash, ms, markRequested)

	s.requestedCache.Set(txHash, request)
	if markRequested {
		s.pending = append(s.pending, request)
	} else {
		s.lifo = append(s.lifo, request)
	}

	return true
}

func (s *RequestQueue) AddMulti(hashes trinary.Hashes, ms milestone_index.MilestoneIndex, markRequested bool) []bool {
	if len(hashes) == 0 {
		return nil
	}

	s.Lock()
	defer s.Unlock()

	added := make([]bool, len(hashes))
	for i, hash := range hashes {
		added[i] = s.add(hash, ms, markRequested)
	}
	return added
}

func (s *RequestQueue) Add(txHash trinary.Hash, ms milestone_index.MilestoneIndex, markRequested bool) bool {
	s.Lock()
	defer s.Unlock()

	return s.add(txHash, ms, markRequested)
}

func (s *RequestQueue) MarkReceived(txHash trinary.Hash) bool {

	s.Lock()
	defer s.Unlock()

	cachedRequest := s.requestedCache.Get(txHash)
	if !typeutils.IsInterfaceNil(cachedRequest) {
		request := cachedRequest.(*request)
		request.markReceived()
		return true
	}

	// If this was already evicted from our cache, check if it is still in the pending queue
	for _, req := range s.pending {
		if req.hash == txHash {
			req.markReceived()
			return true
		}
	}
	return false
}

func (s *RequestQueue) MarkProcessed(txHash trinary.Hash) bool {
	s.Lock()
	defer s.Unlock()

	cachedRequest := s.requestedCache.Get(txHash)
	if !typeutils.IsInterfaceNil(cachedRequest) {
		request := cachedRequest.(*request)
		request.markProcessed()
	} else {
		// If this was already evicted from our cache, check if it is still in the pending queue
		for _, req := range s.pending {
			if req.hash == txHash {
				req.markProcessed()
				break
			}
		}
	}

	// First check if we still have tx waiting to be requested
	if len(s.lifo) > 0 {
		return false
	}

	// Check the current pending if they are all processed
	length := len(s.pending)
	if length > 0 {
		for i := length - 1; i >= 0; i-- {
			request := s.pending[i]
			if !request.isProcessed() {
				// We still have pending tx that are not yet processed
				return false
			}
		}
	}

	// All pending are done, and our lifo is empty
	return true
}

func (s *RequestQueue) CurrentMilestoneIndexAndSize() (index milestone_index.MilestoneIndex, size int) {
	s.Lock()
	defer s.Unlock()

	lengthLifo := len(s.lifo)
	lengthPending := len(s.pending)

	if lengthLifo > 0 {
		n := lengthLifo - 1
		r := s.lifo[n]
		return r.msIndex, lengthLifo + lengthPending
	} else if lengthPending > 0 {
		n := lengthPending - 1
		r := s.pending[n]
		return r.msIndex, lengthLifo + lengthPending
	}

	return 0, 0
}

func (s *RequestQueue) DebugRequests() []*DebugRequest {
	s.Lock()
	defer s.Unlock()

	var requests []*DebugRequest

	for _, req := range s.lifo {
		contains, _ := s.Contains(req.hash)
		exists, _ := tangle.ContainsTransaction(req.hash)
		requests = append(requests, &DebugRequest{
			Hash:        req.hash,
			InCache:     contains,
			InLifo:      true,
			InPending:   false,
			IsProcessed: req.isProcessed(),
			IsReceived:  req.isReceived(),
			TxExists:    exists,
		})
	}

	for _, req := range s.pending {
		contains, _ := s.Contains(req.hash)
		exists, _ := tangle.ContainsTransaction(req.hash)
		requests = append(requests, &DebugRequest{
			Hash:        req.hash,
			InCache:     contains,
			InLifo:      false,
			InPending:   true,
			IsProcessed: req.isProcessed(),
			IsReceived:  req.isReceived(),
			TxExists:    exists,
		})
	}

	return requests
}
