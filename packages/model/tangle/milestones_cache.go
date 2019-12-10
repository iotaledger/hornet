package tangle

import (
	"github.com/gohornet/hornet/packages/datastructure"
	"github.com/gohornet/hornet/packages/model/milestone_index"
)

var (
	milestoneCache *datastructure.LRUCache
)

func InitMilestoneCache() {
	milestoneCache = datastructure.NewLRUCache(MilestoneCacheSize, &datastructure.LRUCacheOptions{
		EvictionCallback:  onEvictMilestones,
		EvictionBatchSize: 100,
	})
}

func onEvictMilestones(_ interface{}, values interface{}) {
	valT := values.([]interface{})

	var milestones []*Bundle
	for _, obj := range valT {
		milestones = append(milestones, obj.(*Bundle))
	}

	if err := StoreMilestonesInDatabase(milestones); err != nil {
		panic(err)
	}
}

func DiscardMilestoneFromCache(milestoneIndex milestone_index.MilestoneIndex) {
	milestoneCache.DeleteWithoutEviction(milestoneIndex)
}

func FlushMilestoneCache() {
	milestoneCache.DeleteAll()
}
