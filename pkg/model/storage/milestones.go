package storage

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
)

func MilestoneCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMilestone *CachedMilestone))(params[0].(*CachedMilestone).Retain()) // milestone pass +1
}

func MilestoneWithMessageIdAndRequestedCaller(handler interface{}, params ...interface{}) {
	handler.(func(messageID hornet.BlockID, cachedMilestone *CachedMilestone, requested bool))(params[0].(hornet.BlockID), params[1].(*CachedMilestone).Retain(), params[2].(bool)) // milestone pass +1
}
