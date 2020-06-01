package tangle

import (
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

func (bundle *Bundle) setMilestone(milestone bool) {
	bundle.Lock()
	defer bundle.Unlock()

	if milestone != bundle.metadata.HasFlag(MetadataIsMilestone) {
		bundle.metadata = bundle.metadata.ModifyFlag(MetadataIsMilestone, milestone)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) IsMilestone() bool {
	bundle.RLock()
	defer bundle.RUnlock()

	return bundle.metadata.HasFlag(MetadataIsMilestone)
}

func (bundle *Bundle) GetMilestoneIndex() milestone.Index {
	bundle.milestoneIndexOnce.Do(func() {
		cachedTailTx := bundle.GetTail() // tx +1
		bundle.milestoneIndex = milestone.Index(trinary.TrytesToInt(cachedTailTx.GetTransaction().Tx.ObsoleteTag))
		cachedTailTx.Release() // tx -1
	})

	return bundle.milestoneIndex
}

func (bundle *Bundle) GetMilestoneHash() hornet.Hash {
	return bundle.tailTx
}
