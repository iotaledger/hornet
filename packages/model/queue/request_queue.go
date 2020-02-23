package queue

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

const (
	RequestQueueTickerInterval = 2 * time.Second
)

type RequestQueue struct {
	syncutils.Mutex
	requestedStorage *objectstorage.ObjectStorage
	lifo             []*request
	pending          []*request
	ticker           *time.Ticker
	tickerDone       chan bool
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
		requestedStorage: objectstorage.New(
			nil,
			nil,
			requestFactory,
			objectstorage.CacheTime(0),
			objectstorage.PersistenceEnabled(false),
			objectstorage.LeakDetectionEnabled(false, objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: 20,
				MaxConsumerHoldTime:   120 * time.Second,
			}),
		),
		ticker:     time.NewTicker(RequestQueueTickerInterval),
		tickerDone: make(chan bool),
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

func (s *RequestQueue) GetStorageSize() int {
	return s.requestedStorage.GetSize()
}

// request +1
func (s *RequestQueue) GetCachedRequestOrNil(transactionHash trinary.Hash) *CachedRequest {
	cachedRequest := s.requestedStorage.Get(trinary.MustTrytesToBytes(transactionHash)[:49])
	if !cachedRequest.Exists() {
		cachedRequest.Release()
		return nil
	}
	return &CachedRequest{CachedObject: cachedRequest}
}

// request +-0
func (s *RequestQueue) ContainsRequest(transactionHash trinary.Hash) bool {
	return s.requestedStorage.Contains(trinary.MustTrytesToBytes(transactionHash)[:49])
}

// request +1
func (s *RequestQueue) PutRequest(request *request) *CachedRequest {
	return &CachedRequest{s.requestedStorage.Put(request)}
}

// request +-0
func (s *RequestQueue) DeleteRequest(txHash trinary.Hash) {
	s.requestedStorage.Delete(trinary.MustTrytesToBytes(txHash)[:49])
}

func (s *RequestQueue) retryPending() {

	s.Lock()

	stillPending := []*request{}

	for _, r := range s.pending {
		if r.isProcessed() {
			// Request done, release consumer and delete
			r.cachedRequest.Release() // request -1
			s.DeleteRequest(r.hash)
		} else if r.isReceived() {
			// Request received but not processed yet, so re-add to pending queue
			stillPending = append(stillPending, r)
		} else {
			// We haven't received any answer for this request, so re-add it to our lifo queue
			if s.ContainsRequest(r.hash) { // Check if the request is still in the storage (there was a problem that deleted requests are still in pending)
				s.lifo = append(s.lifo, r)
			}
		}
	}

	s.pending = stillPending
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

				if request.isProcessed() {
					// Request done, release consumer and delete
					request.cachedRequest.Release() // request -1
					s.DeleteRequest(request.hash)
				} else {
					// Request received but not processed yet, so re-add to pending queue
					s.pending = append(s.pending, request)
				}
				continue
			}
			request.updateTimes()
			s.lifo = append(s.lifo[:i], s.lifo[i+1:]...)
			s.pending = append(s.pending, request)
			return request.hashBytes, request.hash, request.msIndex
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

				if request.isProcessed() {
					// Request done, release consumer and delete
					request.cachedRequest.Release() // request -1
					s.DeleteRequest(request.hash)
				} else {
					// Request received but not processed yet, so re-add to pending queue
					s.pending = append(s.pending, request)
				}
				continue
			} else if request.msIndex < startIndex || request.msIndex > endIndex {
				// Not in range, skip it
				continue
			}
			request.updateTimes()
			s.lifo = append(s.lifo[:i], s.lifo[i+1:]...)
			s.pending = append(s.pending, request)
			return request.hashBytes, request.hash, request.msIndex
		}
	}
	return nil, "", 0

}

func (s *RequestQueue) add(txHash trinary.Hash, msIndex milestone_index.MilestoneIndex, markRequested bool) bool {

	if len(txHash) == 0 {
		return false
	}

	if s.ContainsRequest(txHash) { // +-0
		return false
	}

	request := newRequest(txHash, msIndex, markRequested)

	request.cachedRequest = s.PutRequest(request) // request +1		Consumer stays registered until request is done
	if markRequested {
		s.pending = append(s.pending, request)
	} else {
		s.lifo = append(s.lifo, request)
	}

	return true
}

func (s *RequestQueue) AddMulti(hashes trinary.Hashes, msIndex milestone_index.MilestoneIndex, markRequested bool) []bool {
	if len(hashes) == 0 {
		return nil
	}

	s.Lock()
	defer s.Unlock()

	added := make([]bool, len(hashes))
	for i, hash := range hashes {
		added[i] = s.add(hash, msIndex, markRequested)
	}
	return added
}

func (s *RequestQueue) Add(txHash trinary.Hash, msIndex milestone_index.MilestoneIndex, markRequested bool) bool {
	s.Lock()
	defer s.Unlock()

	return s.add(txHash, msIndex, markRequested)
}

func (s *RequestQueue) MarkReceived(txHash trinary.Hash) (bool, milestone_index.MilestoneIndex) {

	s.Lock()
	defer s.Unlock()

	cachedRequest := s.GetCachedRequestOrNil(txHash) // request +1
	if cachedRequest == nil {
		return false, milestone_index.MilestoneIndex(0)
	}
	defer cachedRequest.Release() // request -1

	request := cachedRequest.GetRequest()
	request.markReceived()
	return true, request.msIndex
}

func (s *RequestQueue) MarkProcessed(txHash trinary.Hash) {
	s.Lock()
	defer s.Unlock()

	cachedRequest := s.GetCachedRequestOrNil(txHash) // request +1
	if cachedRequest != nil {
		request := cachedRequest.GetRequest()
		request.markProcessed()
		cachedRequest.Release() // request -1
	}
}

func (s *RequestQueue) IsEmpty() bool {

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
		contains := s.ContainsRequest(req.hash)
		exists := tangle.ContainsTransaction(req.hash)
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
		contains := s.ContainsRequest(req.hash)
		exists := tangle.ContainsTransaction(req.hash)
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
