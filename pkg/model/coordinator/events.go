package coordinator

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

// CheckpointCaller is used to signal issued checkpoints.
func CheckpointCaller(handler interface{}, params ...interface{}) {
	handler.(func(checkpointIndex int, tipIndex int, tipsTotal int, messageID hornet.MessageID))(params[0].(int), params[1].(int), params[2].(int), params[3].(hornet.MessageID))
}

// MilestoneCaller is used to signal issued milestones.
func MilestoneCaller(handler interface{}, params ...interface{}) {
	handler.(func(index milestone.Index, messageID hornet.MessageID))(params[0].(milestone.Index), params[1].(hornet.MessageID))
}

// QuorumFinishedCaller is used to signal a finished quorum call.
func QuorumFinishedCaller(handler interface{}, params ...interface{}) {
	handler.(func(result *QuorumFinishedResult))(params[0].(*QuorumFinishedResult))
}
