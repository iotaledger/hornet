//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package gossip_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
	"github.com/iotaledger/hornet/v2/pkg/tpkg"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestRequestQueue(t *testing.T) {
	q := gossip.NewRequestQueue()

	var (
		hashA = tpkg.RandBlockID()
		hashB = tpkg.RandBlockID()
		hashZ = tpkg.RandBlockID()
		hashC = tpkg.RandBlockID()
		hashD = tpkg.RandBlockID()
	)

	requests := []*gossip.Request{
		{
			// 0
			BlockID:        hashA,
			MilestoneIndex: 10,
		},
		{
			// 1
			BlockID:        hashB,
			MilestoneIndex: 7,
		},
		{
			// 2
			BlockID:        hashZ,
			MilestoneIndex: 7,
		},
		{
			// 3
			BlockID:        hashC,
			MilestoneIndex: 5,
		},
		{
			// 4
			BlockID:        hashD,
			MilestoneIndex: 2,
		},
	}

	for _, r := range requests {
		assert.True(t, q.Enqueue(r))
		assert.True(t, q.IsQueued(r.BlockID))
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
			assert.Contains(t, iotago.BlockIDs{hashB, hashZ}, r.BlockID)
		} else {
			assert.Equal(t, r, requests[i])
		}
		assert.True(t, q.IsPending(r.BlockID))
	}

	// queued drained, therefore all reqs pending and non queued
	queued, pending, processing = q.Size()
	assert.Zero(t, queued)
	assert.Equal(t, len(requests), pending)
	assert.Zero(t, processing)

	// mark last from test set as received
	req := q.Received(requests[len(requests)-1].BlockID)

	// check if the correct request was returned
	assert.Equal(t, req, requests[len(requests)-1])

	// check processing
	queued, pending, processing = q.Size()
	assert.Zero(t, queued)
	assert.Equal(t, len(requests)-1, pending)
	assert.Equal(t, processing, 1)

	// mark last from test set as processed
	req = q.Processed(requests[len(requests)-1].BlockID)

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
	assert.Equal(t, requests[3], q.Peek())

	// mark last from test set as received and processed
	q.Received(requests[len(requests)-1].BlockID)
	q.Processed(requests[len(requests)-1].BlockID)

	queued, pending, processing = q.Size()
	assert.Equal(t, queued, len(requests)-1)
	assert.Zero(t, pending)
	assert.Zero(t, processing)

	// use debug call to get all requests
	queuedReqs, pendingReqs, processingReq := q.Requests()
	assert.Equal(t, len(requests)-1, len(queuedReqs))
	for i := 0; i < len(requests)-1; i++ {
		queuedReq := queuedReqs[i]
		assert.NotEqual(t, queuedReq.BlockID, requests[len(requests)-1].BlockID)
	}
	assert.Zero(t, len(pendingReqs))
	assert.Zero(t, len(processingReq))

	// test edge case
	// request was pending but marked as queued again, and is then marked as received

	// start with a fresh queue
	q = gossip.NewRequestQueue()

	for _, r := range requests {
		assert.True(t, q.Enqueue(r))
		assert.True(t, q.IsQueued(r.BlockID))
	}

	for i := 0; i < len(requests); i++ {
		q.Next()
	}

	// enqueue requests again
	q.EnqueuePending(0)

	for _, r := range requests {
		req := q.Received(r.BlockID)

		// check if the correct request was returned
		assert.Equal(t, req, r)
	}

	for _, r := range requests {
		req := q.Processed(r.BlockID)

		// check if the correct request was returned
		assert.Equal(t, req, r)
	}
}
