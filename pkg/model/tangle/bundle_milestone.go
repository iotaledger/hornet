package tangle

import (
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/t6b1"
)

func (bundle *Bundle) setMilestone(milestone bool) {
	bundle.Lock()
	defer bundle.Unlock()

	if milestone != bundle.metadata.HasBit(MetadataIsMilestone) {
		bundle.metadata = bundle.metadata.ModifyBit(MetadataIsMilestone, milestone)
		bundle.SetModified(true)
	}
}

func (bundle *Bundle) IsMilestone() bool {
	bundle.RLock()
	defer bundle.RUnlock()

	return bundle.metadata.HasBit(MetadataIsMilestone)
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

func (bundle *Bundle) GetMilestoneMerkleTreeHash() []byte {

	headTx := bundle.GetHead()
	defer headTx.Release(true)

	// t6b1 encoding, so 6 trits per byte
	merkleRootHashSizeInTrytes := coordinatorMilestoneMerkleHashFunc.Size() * 6 / consts.TrinaryRadix
	auditPathLength := coordinatorMerkleTreeDepth * consts.HashTrytesSize

	if (auditPathLength + uint64(merkleRootHashSizeInTrytes)) > consts.SignatureMessageFragmentSizeInTrytes {
		return nil
	}

	merkleRootHashTrytes := headTx.GetTransaction().Tx.SignatureMessageFragment[auditPathLength : int(auditPathLength)+merkleRootHashSizeInTrytes]
	return t6b1.MustTrytesToBytes(merkleRootHashTrytes)[:coordinatorMilestoneMerkleHashFunc.Size()]
}
