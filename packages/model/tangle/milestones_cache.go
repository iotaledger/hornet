package tangle

import (
	"github.com/iotaledger/hive.go/lru_cache"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/profile"
)

var (
	MilestoneCache *lru_cache.LRUCache
)

func InitMilestoneCache() {
	opts := profile.GetProfile().Caches.Milestones
	MilestoneCache = lru_cache.NewLRUCache(opts.Size, &lru_cache.LRUCacheOptions{
		EvictionCallback:  onEvictMilestones,
		EvictionBatchSize: opts.EvictionSize,
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
	MilestoneCache.DeleteWithoutEviction(milestoneIndex)
}

func FlushMilestoneCache() {
	MilestoneCache.DeleteAll()
}
