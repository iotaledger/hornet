package queue

import (
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"time"

	"github.com/iotaledger/iota.go/trinary"
)

type request struct {
	hash    trinary.Hash
	bytes   []byte
	msIndex milestone_index.MilestoneIndex

	received  bool
	processed bool

	timeFirstRequest time.Time
	timeLastRequest  time.Time

	index int // The index of the item in the heap.
}

func newRequest(txHash trinary.Hash, ms milestone_index.MilestoneIndex, requested bool) *request {
	txHashTrits := trinary.MustTrytesToTrits(txHash)
	/*
		if trinary.TrailingZeros(txHashTrits) < int64(ownMWM) {
			panic(fmt.Sprintf("Invalid hash requested: %v (invalid PoW). This should never happen!", txHash))
		}
	*/
	txHashBytes := trinary.TritsToBytes(txHashTrits)[:49]

	r := &request{
		hash:    txHash,
		bytes:   txHashBytes,
		msIndex: ms,
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
