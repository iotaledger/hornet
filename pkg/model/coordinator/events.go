package coordinator

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

// CheckpointCaller is used to signal issued checkpoints.
func CheckpointCaller(handler interface{}, params ...interface{}) {
	handler.(func(index int, lastIndex int, txHash hornet.Hash))(params[0].(int), params[1].(int), params[2].(hornet.Hash))
}

// MilestoneCaller is used to signal issued milestones.
func MilestoneCaller(handler interface{}, params ...interface{}) {
	handler.(func(index milestone.Index, tailTxHash hornet.Hash))(params[0].(milestone.Index), params[1].(hornet.Hash))
}
