package tangle

import (
	"errors"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/encoding/b1t6"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

var (
	// ErrInvalidAuditPathLength is returned when audit path is too long to fit in the message fragment.
	ErrInvalidAuditPathLength = errors.New("invalid audit path length")
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

// GetMilestoneMerkleTreeHash returns the Merle tree hash in the milestone bundle.
func (bundle *Bundle) GetMilestoneMerkleTreeHash() ([]byte, error) {

	headTx := bundle.GetHead()
	defer headTx.Release(true)

	hashTrytesLen := b1t6.EncodedLen(coordinatorMilestoneMerkleHashFunc.Size()) / consts.TritsPerTryte
	auditPathTrytesLen := int(coordinatorMerkleTreeDepth) * consts.HashTrytesSize

	txSig := headTx.GetTransaction().Tx.SignatureMessageFragment
	if auditPathTrytesLen >= len(txSig) || auditPathTrytesLen+hashTrytesLen > len(txSig) {
		return nil, ErrInvalidAuditPathLength
	}
	return b1t6.DecodeTrytes(txSig[auditPathTrytesLen : auditPathTrytesLen+hashTrytesLen])
}
