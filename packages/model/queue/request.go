package queue

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/model/milestone"
)

type request struct {
	objectstorage.StorableObjectFlags

	hash      trinary.Hash
	hashBytes []byte
	msIndex   milestone.Index

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
}

func (r *request) ObjectStorageKey() []byte {
	return r.hashBytes
}

func (r *request) ObjectStorageValue() (data []byte) {
	return nil
}

func (r *request) UnmarshalObjectStorageValue(data []byte) (err error, consumedBytes int) {
	return nil, 0
}

func newRequest(txHash trinary.Hash, msIndex milestone.Index, requested bool) *request {
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

func requestFactory(key []byte) (objectstorage.StorableObject, error, int) {
	req := &request{
		hashBytes: make([]byte, len(key)),
	}
	copy(req.hashBytes, key)
	return req, nil, len(key)
}
