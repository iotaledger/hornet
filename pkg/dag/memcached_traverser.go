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

func (m *MemcachedTraverserStorage) CachedMessageMetadata(messageID hornet.MessageID) (*storage.CachedMetadata, error) {
	return m.metadataMemcache.CachedMessageMetadata(messageID)
}

func (m *MemcachedTraverserStorage) ChildrenMessageIDs(messageID hornet.MessageID, iteratorOptions ...storage.IteratorOption) (hornet.MessageIDs, error) {
	return m.traverserStorage.ChildrenMessageIDs(messageID, iteratorOptions...)
}

func (m *MemcachedTraverserStorage) SolidEntryPointsContain(messageID hornet.MessageID) (bool, error) {
	return m.traverserStorage.SolidEntryPointsContain(messageID)

}
func (m *MemcachedTraverserStorage) SolidEntryPointsIndex(messageID hornet.MessageID) (milestone.Index, bool, error) {
	return m.traverserStorage.SolidEntryPointsIndex(messageID)
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

func (m *MemcachedParentsTraverserStorage) CachedMessageMetadata(messageID hornet.MessageID) (*storage.CachedMetadata, error) {
	return m.metadataMemcache.CachedMessageMetadata(messageID)
}

func (m *MemcachedParentsTraverserStorage) SolidEntryPointsContain(messageID hornet.MessageID) (bool, error) {
	return m.parentsTraverserStorage.SolidEntryPointsContain(messageID)

}
func (m *MemcachedParentsTraverserStorage) SolidEntryPointsIndex(messageID hornet.MessageID) (milestone.Index, bool, error) {
	return m.parentsTraverserStorage.SolidEntryPointsIndex(messageID)
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

func (m *MemcachedChildrenTraverserStorage) CachedMessageMetadata(messageID hornet.MessageID) (*storage.CachedMetadata, error) {
	return m.metadataMemcache.CachedMessageMetadata(messageID)
}

func (m *MemcachedChildrenTraverserStorage) ChildrenMessageIDs(messageID hornet.MessageID, iteratorOptions ...storage.IteratorOption) (hornet.MessageIDs, error) {
	return m.childrenTraverserStorage.ChildrenMessageIDs(messageID, iteratorOptions...)
}

func (m *MemcachedChildrenTraverserStorage) Cleanup(forceRelease bool) {
	m.metadataMemcache.Cleanup(forceRelease)
}
