package tangle

import (
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"sort"

	"github.com/gohornet/hornet/packages/syncutils"
)

type MilestoneQueue struct {
	syncutils.RWMutex
	pending map[milestone_index.MilestoneIndex]*Bundle
}

func NewMilestoneQueue() *MilestoneQueue {
	return &MilestoneQueue{
		pending: make(map[milestone_index.MilestoneIndex]*Bundle),
	}
}

func (q *MilestoneQueue) Push(milestone *Bundle) bool {
	q.Lock()
	_, found := q.pending[milestone.GetMilestoneIndex()]
	if !found {
		q.pending[milestone.GetMilestoneIndex()] = milestone
	}
	q.Unlock()
	return !found
}

func (q *MilestoneQueue) Pop() *Bundle {
	q.Lock()
	defer q.Unlock()

	ms := GetKeysHornetMilestoneIndexHornetBundle(q.pending)
	sort.Slice(ms, func(i, j int) bool { return ms[i] < ms[j] })
	if len(ms) > 0 {
		r := q.pending[ms[0]]
		delete(q.pending, ms[0])
		return r
	}
	return nil
}

func (q *MilestoneQueue) GetSize() int {
	q.RLock()
	defer q.RUnlock()

	return len(q.pending)
}
