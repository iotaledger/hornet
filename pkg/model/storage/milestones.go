package storage

import iotago "github.com/iotaledger/iota.go/v3"

func MilestoneCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(cachedMilestone *CachedMilestone))(params[0].(*CachedMilestone).Retain()) // milestone pass +1
}

func MilestoneIndexCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(index iotago.MilestoneIndex))(params[0].(iotago.MilestoneIndex))
}

func MilestoneWithBlockIDAndRequestedCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(blockID iotago.BlockID, cachedMilestone *CachedMilestone, requested bool))(params[0].(iotago.BlockID), params[1].(*CachedMilestone).Retain(), params[2].(bool)) // milestone pass +1
}
