package mqtt

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
)

type milestoneInfo struct {
	// The latest known milestone index.
	LatestMilestoneIndex milestone.Index `json:"latestMilestoneIndex"`
	// The current solid milestone's index.
	SolidMilestoneIndex milestone.Index `json:"solidMilestoneIndex"`
}
