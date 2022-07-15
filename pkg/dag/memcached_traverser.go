package dag

import (
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	iotago "github.com/iotaledger/iota.go/v3"
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

func (m *MemcachedTraverserStorage) CachedBlockMetadata(blockID iotago.BlockID) (*storage.CachedMetadata, error) {
	return m.metadataMemcache.CachedBlockMetadata(blockID)
}

func (m *MemcachedTraverserStorage) ChildrenBlockIDs(blockID iotago.BlockID, iteratorOptions ...storage.IteratorOption) (iotago.BlockIDs, error) {
	return m.traverserStorage.ChildrenBlockIDs(blockID, iteratorOptions...)
}

func (m *MemcachedTraverserStorage) SolidEntryPointsContain(blockID iotago.BlockID) (bool, error) {
	return m.traverserStorage.SolidEntryPointsContain(blockID)

}
func (m *MemcachedTraverserStorage) SolidEntryPointsIndex(blockID iotago.BlockID) (iotago.MilestoneIndex, bool, error) {
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

func (m *MemcachedParentsTraverserStorage) CachedBlockMetadata(blockID iotago.BlockID) (*storage.CachedMetadata, error) {
	return m.metadataMemcache.CachedBlockMetadata(blockID)
}

func (m *MemcachedParentsTraverserStorage) SolidEntryPointsContain(blockID iotago.BlockID) (bool, error) {
	return m.parentsTraverserStorage.SolidEntryPointsContain(blockID)

}
func (m *MemcachedParentsTraverserStorage) SolidEntryPointsIndex(blockID iotago.BlockID) (iotago.MilestoneIndex, bool, error) {
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

func (m *MemcachedChildrenTraverserStorage) CachedBlockMetadata(blockID iotago.BlockID) (*storage.CachedMetadata, error) {
	return m.metadataMemcache.CachedBlockMetadata(blockID)
}

func (m *MemcachedChildrenTraverserStorage) ChildrenBlockIDs(blockID iotago.BlockID, iteratorOptions ...storage.IteratorOption) (iotago.BlockIDs, error) {
	return m.childrenTraverserStorage.ChildrenBlockIDs(blockID, iteratorOptions...)
}

func (m *MemcachedChildrenTraverserStorage) Cleanup(forceRelease bool) {
	m.metadataMemcache.Cleanup(forceRelease)
}
