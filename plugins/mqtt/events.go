package mqtt

import (
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/model/utxo"
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

func onUTXOOutput(output *utxo.Output, spent bool) {
	publishOutput(output, spent)
}
