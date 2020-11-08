package mqtt

import (
	"github.com/gohornet/hornet/pkg/model/tangle"
)

func onNewLatestMilestone(cachedMs *tangle.CachedMilestone) {
	defer cachedMs.Release(true)

	publishMilestoneOnTopic(topicMilestonesLatest, cachedMs.GetMilestone())
}

func onNewSolidMilestone(cachedMs *tangle.CachedMilestone) {
	defer cachedMs.Release(true)

	publishMilestoneOnTopic(topicMilestonesSolid, cachedMs.GetMilestone())
}

func onMessageMetadata(cachedMetadata *tangle.CachedMetadata) {
	defer cachedMetadata.Release(true)

	publishMessageMetadata(cachedMetadata.Retain())
}
