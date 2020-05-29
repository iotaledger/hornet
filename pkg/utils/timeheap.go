package utils

import (
	"container/heap"
	"sync"
	"time"
)

// timeHeapEntry is an element for TimeHeap.
type timeHeapEntry struct {
	timestamp time.Time
	count     uint64
}

// TimeHeap implements a heap sorted by time, where older elements are popped during GetAveragePerSecond call.
type TimeHeap struct {
	lock  *sync.Mutex
	list  []*timeHeapEntry
	total uint64
}

// NewTimeHeap creates a new TimeHeap object.
func NewTimeHeap() *TimeHeap {
	h := &TimeHeap{lock: &sync.Mutex{}}
	heap.Init(h)
	return h
}

// Add a new entry to the container with a count for the average calculation.
func (h *TimeHeap) Add(count uint64) {
	h.lock.Lock()
	defer h.lock.Unlock()
	heap.Push(h, &timeHeapEntry{timestamp: time.Now(), count: count})
	h.total += count
}

// GetAveragePerSecond calculates the average per second of all entries in the given duration.
// older elements are removed from the container.
func (h *TimeHeap) GetAveragePerSecond(timeBefore time.Duration) float32 {
	h.lock.Lock()
	defer h.lock.Unlock()

	lenHeap := len((*h).list)
	if lenHeap > 0 {
		for i := 0; i < lenHeap; i++ {
			oldest := heap.Pop(h)
			if time.Since(oldest.(*timeHeapEntry).timestamp) < timeBefore {
				heap.Push(h, oldest)
				break
			}
			h.total -= oldest.(*timeHeapEntry).count
		}
	}

	return float32(h.total) / float32(timeBefore.Seconds())
}

///////////////// heap interface /////////////////
func (h TimeHeap) Len() int           { return len(h.list) }
func (h TimeHeap) Less(i, j int) bool { return h.list[i].timestamp.Before(h.list[j].timestamp) }
func (h TimeHeap) Swap(i, j int)      { h.list[i], h.list[j] = h.list[j], h.list[i] }

func (h *TimeHeap) Push(x interface{}) {
	(*h).list = append((*h).list, x.(*timeHeapEntry))
}

func (h *TimeHeap) Pop() interface{} {
	old := (*h).list
	n := len(old)
	x := old[n-1]
	old[n-1] = nil // avoid memory leak
	(*h).list = old[0 : n-1]
	return x
}
