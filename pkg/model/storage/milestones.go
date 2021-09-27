package storage

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
)

func MilestoneCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMs *CachedMilestone))(params[0].(*CachedMilestone).Retain())
}

func MilestoneWithRequestedCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMs *CachedMilestone, requested bool))(params[0].(*CachedMilestone).Retain(), params[1].(bool))
}

// MilestoneCachedMessageOrNil returns the cached message of a milestone index or nil if it doesn't exist.
// message +1
func (s *Storage) MilestoneCachedMessageOrNil(milestoneIndex milestone.Index) *CachedMessage {

	cachedMs := s.CachedMilestoneOrNil(milestoneIndex) // milestone +1
	if cachedMs == nil {
		return nil
	}
	defer cachedMs.Release(true) // milestone -1

	return s.CachedMessageOrNil(cachedMs.Milestone().MessageID)
}
