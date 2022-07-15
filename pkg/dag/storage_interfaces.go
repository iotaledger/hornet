package dag

import (
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	iotago "github.com/iotaledger/iota.go/v3"
)

// ParentsTraverserStorage provides the interface to the used storage in the ParentsTraverser.
type ParentsTraverserStorage interface {
	CachedBlockMetadata(blockID iotago.BlockID) (*storage.CachedMetadata, error)
	SolidEntryPointsContain(blockID iotago.BlockID) (bool, error)
	SolidEntryPointsIndex(blockID iotago.BlockID) (iotago.MilestoneIndex, bool, error)
}

// ChildrenTraverserStorage provides the interface to the used storage in the ChildrenTraverser.
type ChildrenTraverserStorage interface {
	CachedBlockMetadata(blockID iotago.BlockID) (*storage.CachedMetadata, error)
	SolidEntryPointsContain(blockID iotago.BlockID) (bool, error)
	ChildrenBlockIDs(blockID iotago.BlockID, iteratorOptions ...storage.IteratorOption) (iotago.BlockIDs, error)
}

// TraverserStorage provides the interface to the used storage in the ParentsTraverser and ChildrenTraverser.
type TraverserStorage interface {
	ParentsTraverserStorage
	ChildrenTraverserStorage
}
