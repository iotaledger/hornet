package gossip_test

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	iotago "github.com/iotaledger/iota.go/v2"
)

func randBytes(length int) []byte {
	var b []byte
	for i := 0; i < length; i++ {
		b = append(b, byte(rand.Intn(256)))
	}
	return b
}

func randMessageID() hornet.MessageID {
	return hornet.MessageID(randBytes(iotago.MessageIDLength))
}

func TestRequestQueue(t *testing.T) {
	q := gossip.NewRequestQueue()

	var (
		hashA = randMessageID()
		hashB = randMessageID()
		hashZ = randMessageID()
		hashC = randMessageID()
		hashD = randMessageID()
	)

	requests := []*gossip.Request{
		{
			MessageID:      hashA,
			MilestoneIndex: 10,
		},
		{
			MessageID:      hashB,
			MilestoneIndex: 7,
		},
		{
			MessageID:      hashZ,
			MilestoneIndex: 7,
		},
		{
			MessageID:      hashC,
			MilestoneIndex: 5,
		},
		{
			MessageID:      hashD,
			MilestoneIndex: 2,
		},
	}

	for _, r := range requests {
		assert.True(t, q.Enqueue(r))
		assert.True(t, q.IsQueued(r.MessageID))
	}

	queued, pending, processing := q.Size()
	assert.Equal(t, len(requests), queued)
	assert.Zero(t, pending)
	assert.Zero(t, processing)

	for i := len(requests) - 1; i >= -1; i-- {
		r := q.Next()
		// should return nil when empty
		if i == -1 {
			assert.Nil(t, r)
			continue
		}
		// since we have two request under the same milestone/priority
		// we need to make a special case
		if i == 1 || i == 2 {
			assert.Contains(t, hornet.MessageIDs{hashB, hashZ}, r.MessageID)
		} else {
			assert.Equal(t, r, requests[i])
		}
		assert.True(t, q.IsPending(r.MessageID))
	}

	// queued drained, therefore all reqs pending and non queued
	queued, pending, processing = q.Size()
	assert.Zero(t, queued)
	assert.Equal(t, len(requests), pending)
	assert.Zero(t, processing)

	// mark last from test set as received
	req := q.Received(requests[len(requests)-1].MessageID)

	// check if the correct request was returned
	assert.Equal(t, req, requests[len(requests)-1])

	// check processing
	queued, pending, processing = q.Size()
	assert.Zero(t, queued)
	assert.Equal(t, len(requests)-1, pending)
	assert.Equal(t, processing, 1)

	// mark last from test set as processed
	req = q.Processed(requests[len(requests)-1].MessageID)

	// check if the correct request was returned
	assert.Equal(t, req, requests[len(requests)-1])

	// check processed
	queued, pending, processing = q.Size()
	assert.Zero(t, queued)
	assert.Equal(t, len(requests)-1, pending)
	assert.Zero(t, processing)

	// enqueue pending again
	queuedCnt := q.EnqueuePending(0)
	queued, pending, processing = q.Size()
	assert.Equal(t, queued, queuedCnt)
	assert.Zero(t, pending)
	assert.Zero(t, processing)
	// -1 since we marked one request as received
	assert.Equal(t, len(requests)-1, queued)

	// request with the highest priority should be in the front
	assert.Equal(t, requests[0], q.Peek())

	// mark last from test set as received and processed
	q.Received(requests[len(requests)-1].MessageID)
	q.Processed(requests[len(requests)-1].MessageID)

	queued, pending, processing = q.Size()
	assert.Equal(t, queued, len(requests)-1)
	assert.Zero(t, pending)
	assert.Zero(t, processing)

	// use debug call to get all requests
	queuedReqs, pendingReqs, processingReq := q.Requests()
	assert.Equal(t, len(requests)-1, len(queuedReqs))
	for i := 0; i < len(requests)-1; i++ {
		queuedReq := queuedReqs[i]
		assert.False(t, bytes.Equal(queuedReq.MessageID, requests[len(requests)-1].MessageID))
	}
	assert.Zero(t, len(pendingReqs))
	assert.Zero(t, len(processingReq))

	// test edge case
	// request was pending but marked as queued again, and is then marked as received

	// start with a fresh queue
	q = gossip.NewRequestQueue()

	for _, r := range requests {
		assert.True(t, q.Enqueue(r))
		assert.True(t, q.IsQueued(r.MessageID))
	}

	for i := 0; i < len(requests); i++ {
		q.Next()
	}

	// enqueue requests again
	q.EnqueuePending(0)

	for _, r := range requests {
		req := q.Received(r.MessageID)

		// check if the correct request was returned
		assert.Equal(t, req, r)
	}

	for _, r := range requests {
		req := q.Processed(r.MessageID)

		// check if the correct request was returned
		assert.Equal(t, req, r)
	}
}
