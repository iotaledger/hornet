package queue

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/model/milestone_index"
)

type request struct {
	objectstorage.StorableObjectFlags

	hash      trinary.Hash
	hashBytes []byte
	msIndex   milestone_index.MilestoneIndex

	received  bool
	processed bool

	timeFirstRequest time.Time
	timeLastRequest  time.Time

	index int // The index of the item in the heap.

	cachedRequest *CachedRequest
}

// ObjectStorage interface

func (r *request) Update(other objectstorage.StorableObject) {
	panic("request should never be updated")
	/*
		if obj, ok := other.(*request); !ok {
			panic("invalid object passed to request.Update()")
		} else {
			r.hash = obj.hash
			r.hashBytes = obj.hashBytes
			r.msIndex = obj.msIndex

			r.received = obj.received
			r.processed = obj.processed

			r.timeFirstRequest = obj.timeFirstRequest
			r.timeLastRequest = obj.timeLastRequest

			r.index = obj.index
		}
	*/
}

func (r *request) GetStorageKey() []byte {
	return r.hashBytes
}

func (r *request) MarshalBinary() (data []byte, err error) {
	return nil, nil
}

func (r *request) UnmarshalBinary(data []byte) error {
	return nil
}

func newRequest(txHash trinary.Hash, msIndex milestone_index.MilestoneIndex, requested bool) *request {
	txHashBytes := trinary.MustTrytesToBytes(txHash)[:49]

	r := &request{
		hash:      txHash,
		hashBytes: txHashBytes,
		msIndex:   msIndex,
	}

	if requested {
		r.timeFirstRequest = time.Now()
		r.timeLastRequest = r.timeFirstRequest
	}

	return r
}

func (r *request) markReceived() {
	r.received = true
}

func (r *request) markProcessed() {
	r.processed = true
}

func (r *request) isReceived() bool {
	return r.received
}

func (r *request) isProcessed() bool {
	return r.processed
}

func (r *request) updateTimes() {
	r.timeLastRequest = time.Now()
	if r.timeFirstRequest.IsZero() {
		r.timeFirstRequest = r.timeLastRequest
	}
}

type CachedRequest struct {
	objectstorage.CachedObject
}

func (c *CachedRequest) GetRequest() *request {
	return c.Get().(*request)
}

func requestFactory(key []byte) objectstorage.StorableObject {
	req := &request{
		hashBytes: make([]byte, len(key)),
	}
	copy(req.hashBytes, key)
	return req
}
