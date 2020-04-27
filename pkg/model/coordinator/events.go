package coordinator

import (
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

// CheckpointCaller is used to signal issued checkpoints.
func CheckpointCaller(handler interface{}, params ...interface{}) {
	handler.(func(index int, lastIndex int, txHash trinary.Hash))(params[0].(int), params[1].(int), params[2].(trinary.Hash))
}

// MilestoneCaller is used to signal issued milestones.
func MilestoneCaller(handler interface{}, params ...interface{}) {
	handler.(func(index milestone.Index, tailTxHash trinary.Hash))(params[0].(milestone.Index), params[1].(trinary.Hash))
}
