package tangle

import (
	"github.com/iotaledger/iota.go/trinary"
	"github.com/gohornet/hornet/packages/model/milestone_index"
)

func (bundle *Bundle) IsMilestone() bool {
	bundle.metadataMutex.RLock()
	defer bundle.metadataMutex.RUnlock()

	return bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_IS_MILESTONE)
}

func (bundle *Bundle) SetMilestone(milestone bool) {
	bundle.metadataMutex.Lock()
	defer bundle.metadataMutex.Unlock()

	if milestone != bundle.metadata.HasFlag(HORNET_BUNDLE_METADATA_IS_MILESTONE) {
		bundle.metadata = bundle.metadata.ModifyingFlag(HORNET_BUNDLE_METADATA_IS_MILESTONE, milestone)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) GetMilestoneIndex() milestone_index.MilestoneIndex {
	return milestone_index.MilestoneIndex(trinary.TrytesToInt(bundle.GetTail().Tx.ObsoleteTag))
}

func (bundle *Bundle) GetMilestoneHash() trinary.Hash {
	return bundle.GetTail().GetHash()
}

func (bundle *Bundle) GetTrunk() trinary.Hash {
	return bundle.GetHead().GetTrunk()
}

func (bundle *Bundle) GetBranch() trinary.Hash {
	return bundle.GetHead().GetBranch()
}
