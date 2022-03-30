package dag

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
)

// ParentsTraverserStorage provides the interface to the used storage in the ParentsTraverser.
type ParentsTraverserStorage interface {
	CachedMessageMetadata(messageID hornet.MessageID) (*storage.CachedMetadata, error)
	SolidEntryPointsContain(messageID hornet.MessageID) (bool, error)
	SolidEntryPointsIndex(messageID hornet.MessageID) (milestone.Index, bool, error)
}

// ChildrenTraverserStorage provides the interface to the used storage in the ChildrenTraverser.
type ChildrenTraverserStorage interface {
	CachedMessageMetadata(messageID hornet.MessageID) (*storage.CachedMetadata, error)
	ChildrenMessageIDs(messageID hornet.MessageID, iteratorOptions ...storage.IteratorOption) (hornet.MessageIDs, error)
}

// TraverserStorage provides the interface to the used storage in the ParentsTraverser and ChildrenTraverser.
type TraverserStorage interface {
	ParentsTraverserStorage
	ChildrenTraverserStorage
}
