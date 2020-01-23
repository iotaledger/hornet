package tangle

import (
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/iotaledger/iota.go/trinary"
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
		bundle.metadata = bundle.metadata.ModifyFlag(HORNET_BUNDLE_METADATA_IS_MILESTONE, milestone)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) GetMilestoneIndex() milestone_index.MilestoneIndex {
	tail := bundle.GetTail() //+1
	defer tail.Release()     //-1
	return milestone_index.MilestoneIndex(trinary.TrytesToInt(tail.GetTransaction().Tx.ObsoleteTag))
}

func (bundle *Bundle) GetMilestoneHash() trinary.Hash {
	tail := bundle.GetTail() //+1
	defer tail.Release()     //-1
	return tail.GetTransaction().GetHash()
}

func (bundle *Bundle) GetTrunk() trinary.Hash {
	head := bundle.GetHead() //+1
	defer head.Release()     //-1
	return head.GetTransaction().GetTrunk()
}

func (bundle *Bundle) GetBranch() trinary.Hash {
	head := bundle.GetHead() //+1
	defer head.Release()     //-1
	return head.GetTransaction().GetBranch()
}
