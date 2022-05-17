package dag

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
)

type MemcachedTraverserStorage struct {
	traverserStorage TraverserStorage
	metadataMemcache *storage.MetadataMemcache
}

func NewMemcachedTraverserStorage(traverserStorage TraverserStorage, metadataMemcache *storage.MetadataMemcache) *MemcachedTraverserStorage {
	return &MemcachedTraverserStorage{
		traverserStorage: traverserStorage,
		metadataMemcache: metadataMemcache,
	}
}

func (m *MemcachedTraverserStorage) CachedBlockMetadata(blockID hornet.BlockID) (*storage.CachedMetadata, error) {
	return m.metadataMemcache.CachedBlockMetadata(blockID)
}

func (m *MemcachedTraverserStorage) ChildrenMessageIDs(blockID hornet.BlockID, iteratorOptions ...storage.IteratorOption) (hornet.BlockIDs, error) {
	return m.traverserStorage.ChildrenMessageIDs(blockID, iteratorOptions...)
}

func (m *MemcachedTraverserStorage) SolidEntryPointsContain(blockID hornet.BlockID) (bool, error) {
	return m.traverserStorage.SolidEntryPointsContain(blockID)

}
func (m *MemcachedTraverserStorage) SolidEntryPointsIndex(blockID hornet.BlockID) (milestone.Index, bool, error) {
	return m.traverserStorage.SolidEntryPointsIndex(blockID)
}

func (m *MemcachedTraverserStorage) Cleanup(forceRelease bool) {
	m.metadataMemcache.Cleanup(forceRelease)
}

type MemcachedParentsTraverserStorage struct {
	parentsTraverserStorage ParentsTraverserStorage
	metadataMemcache        *storage.MetadataMemcache
}

func NewMemcachedParentsTraverserStorage(parentsTraverserStorage ParentsTraverserStorage, metadataMemcache *storage.MetadataMemcache) *MemcachedParentsTraverserStorage {
	return &MemcachedParentsTraverserStorage{
		parentsTraverserStorage: parentsTraverserStorage,
		metadataMemcache:        metadataMemcache,
	}
}

func (m *MemcachedParentsTraverserStorage) CachedBlockMetadata(blockID hornet.BlockID) (*storage.CachedMetadata, error) {
	return m.metadataMemcache.CachedBlockMetadata(blockID)
}

func (m *MemcachedParentsTraverserStorage) SolidEntryPointsContain(blockID hornet.BlockID) (bool, error) {
	return m.parentsTraverserStorage.SolidEntryPointsContain(blockID)

}
func (m *MemcachedParentsTraverserStorage) SolidEntryPointsIndex(blockID hornet.BlockID) (milestone.Index, bool, error) {
	return m.parentsTraverserStorage.SolidEntryPointsIndex(blockID)
}

func (m *MemcachedParentsTraverserStorage) Cleanup(forceRelease bool) {
	m.metadataMemcache.Cleanup(forceRelease)
}

type MemcachedChildrenTraverserStorage struct {
	childrenTraverserStorage ChildrenTraverserStorage
	metadataMemcache         *storage.MetadataMemcache
}

func NewMemcachedChildrenTraverserStorage(childrenTraverserStorage ChildrenTraverserStorage, metadataMemcache *storage.MetadataMemcache) *MemcachedChildrenTraverserStorage {
	return &MemcachedChildrenTraverserStorage{
		childrenTraverserStorage: childrenTraverserStorage,
		metadataMemcache:         metadataMemcache,
	}
}

func (m *MemcachedChildrenTraverserStorage) CachedMessageMetadata(blockID hornet.BlockID) (*storage.CachedMetadata, error) {
	return m.metadataMemcache.CachedBlockMetadata(blockID)
}

func (m *MemcachedChildrenTraverserStorage) ChildrenMessageIDs(blockID hornet.BlockID, iteratorOptions ...storage.IteratorOption) (hornet.BlockIDs, error) {
	return m.childrenTraverserStorage.ChildrenMessageIDs(blockID, iteratorOptions...)
}

func (m *MemcachedChildrenTraverserStorage) Cleanup(forceRelease bool) {
	m.metadataMemcache.Cleanup(forceRelease)
}
