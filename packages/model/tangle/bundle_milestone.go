package tangle

import (
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/milestone"
)

func (bundle *Bundle) setMilestone(milestone bool) {
	bundle.Lock()
	defer bundle.Unlock()

	if milestone != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_IS_MILESTONE) {
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_IS_MILESTONE, milestone)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) IsMilestone() bool {
	bundle.RLock()
	defer bundle.RUnlock()

	return bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_IS_MILESTONE)
}

func (bundle *Bundle) GetMilestoneIndex() milestone.Index {
	cachedTailTx := bundle.GetTail() // tx +1
	index := milestone.Index(trinary.TrytesToInt(cachedTailTx.GetTransaction().Tx.ObsoleteTag))
	cachedTailTx.Release() // tx -1
	return index
}

func (bundle *Bundle) GetMilestoneHash() trinary.Hash {
	return bundle.tailTx
}
