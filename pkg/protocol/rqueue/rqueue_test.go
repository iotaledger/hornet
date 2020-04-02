package rqueue_test

import (
	"testing"

	"github.com/gohornet/hornet/pkg/protocol/rqueue"
	"github.com/stretchr/testify/assert"
)

func TestRequestQueue(t *testing.T) {
	q := rqueue.New()

	requests := []*rqueue.Request{
		{
			Hash:           "A",
			MilestoneIndex: 10,
		},
		{
			Hash:           "B",
			MilestoneIndex: 7,
		},
		{
			Hash:           "Z",
			MilestoneIndex: 7,
		},
		{
			Hash:           "C",
			MilestoneIndex: 5,
		},
		{
			Hash:           "D",
			MilestoneIndex: 2,
		},
	}

	for _, r := range requests {
		assert.True(t, q.Enqueue(r))
		assert.True(t, q.IsQueued(r.Hash))
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
			assert.Contains(t, []string{"B", "Z"}, r.Hash)
		} else {
			assert.Equal(t, r, requests[i])
		}
		assert.True(t, q.IsPending(r.Hash))
	}

	// queued drained, therefore all reqs pending and non queued
	queued, pending, processing = q.Size()
	assert.Zero(t, queued)
	assert.Equal(t, len(requests), pending)
	assert.Zero(t, processing)

	// mark last from test set as received
	q.Received(requests[len(requests)-1].Hash)

	// Check processing
	queued, pending, processing = q.Size()
	assert.Zero(t, queued)
	assert.Equal(t, len(requests)-1, pending)
	assert.Equal(t, processing, 1)

	q.Processed(requests[len(requests)-1].Hash)

	// Check processed
	queued, pending, processing = q.Size()
	assert.Zero(t, queued)
	assert.Equal(t, len(requests)-1, pending)
	assert.Zero(t, processing)

	// enqueue pending again
	newlyEnqueued := q.EnqueuePending(0)
	queued, pending, processing = q.Size()
	assert.Equal(t, queued, newlyEnqueued)
	assert.Zero(t, pending)
	assert.Zero(t, processing)
	// -1 since we marked one request as received
	assert.Equal(t, len(requests)-1, queued)

	// request with the highest priority should be in the front
	assert.Equal(t, requests[0], q.Peek())

	// mark last from test set as received and processed
	q.Received(requests[len(requests)-1].Hash)
	q.Processed(requests[len(requests)-1].Hash)

	queued, pending, processing = q.Size()
	assert.Equal(t, queued, len(requests)-1)
	assert.Zero(t, pending)
	assert.Zero(t, processing)

	// use debug call to get all requests
	queuedReqs, pendingReqs, processingReq := q.Requests()
	assert.Equal(t, len(requests)-1, len(queuedReqs))
	for i := 0; i < len(requests)-1; i++ {
		queuedReq := queuedReqs[i]
		assert.False(t, queuedReq.Hash == requests[len(requests)-1].Hash)
	}
	assert.Zero(t, len(pendingReqs))
	assert.Zero(t, len(processingReq))
}
