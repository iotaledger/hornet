package storage

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
)

func MilestoneCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMilestone *CachedMilestone))(params[0].(*CachedMilestone).Retain()) // milestone pass +1
}

func MilestoneWithRequestedCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMilestone *CachedMilestone, requested bool))(params[0].(*CachedMilestone).Retain(), params[1].(bool)) // milestone pass +1
}

// MilestoneCachedMessageOrNil returns the cached message of a milestone index or nil if it doesn't exist.
// message +1
func (s *Storage) MilestoneCachedMessageOrNil(milestoneIndex milestone.Index) *CachedMessage {

	cachedMilestone := s.CachedMilestoneOrNil(milestoneIndex) // milestone +1
	if cachedMilestone == nil {
		return nil
	}
	defer cachedMilestone.Release(true) // milestone -1

	return s.CachedMessageOrNil(cachedMilestone.Milestone().MessageID) // message +1
}
