package mqtt

import (
	"encoding/json"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

var (
	latestMilestoneIndex milestone.Index = 0
	solidMilestoneIndex  milestone.Index = 0
)

func onNewLatestMilestone(cachedMs *tangle.CachedMilestone) {
	defer cachedMs.Release(true)

	latestMilestoneIndex = cachedMs.GetMilestone().Index

	if err := publishMilestones(&milestoneInfo{
		LatestMilestoneIndex: latestMilestoneIndex,
		SolidMilestoneIndex:  solidMilestoneIndex,
	}); err != nil {
		log.Warn(err.Error())
	}
}

func onNewSolidMilestone(cachedMs *tangle.CachedMilestone) {
	defer cachedMs.Release(true)

	solidMilestoneIndex = cachedMs.GetMilestone().Index

	if err := publishMilestones(&milestoneInfo{
		LatestMilestoneIndex: latestMilestoneIndex,
		SolidMilestoneIndex:  solidMilestoneIndex,
	}); err != nil {
		log.Warn(err.Error())
	}
}

func publishMilestones(ms *milestoneInfo) error {
	milestoneInfoJSON, err := json.Marshal(ms)
	if err != nil {
		return err
	}

	mqttBroker.Send(topicMilestones, milestoneInfoJSON)
	return nil
}
