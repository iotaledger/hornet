package dag

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
)

// ParentsTraverserStorage provides the interface to the used storage in the ParentsTraverser.
type ParentsTraverserStorage interface {
	CachedBlockMetadata(blockID hornet.BlockID) (*storage.CachedMetadata, error)
	SolidEntryPointsContain(blockID hornet.BlockID) (bool, error)
	SolidEntryPointsIndex(blockID hornet.BlockID) (milestone.Index, bool, error)
}

// ChildrenTraverserStorage provides the interface to the used storage in the ChildrenTraverser.
type ChildrenTraverserStorage interface {
	CachedBlockMetadata(blockID hornet.BlockID) (*storage.CachedMetadata, error)
	SolidEntryPointsContain(blockID hornet.BlockID) (bool, error)
	ChildrenMessageIDs(blockID hornet.BlockID, iteratorOptions ...storage.IteratorOption) (hornet.BlockIDs, error)
}

// TraverserStorage provides the interface to the used storage in the ParentsTraverser and ChildrenTraverser.
type TraverserStorage interface {
	ParentsTraverserStorage
	ChildrenTraverserStorage
}
