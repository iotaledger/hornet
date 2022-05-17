package storage

import (
	"time"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	iotago "github.com/iotaledger/iota.go/v3"
)

// CachedChild represents a cached Child.
type CachedChild struct {
	objectstorage.CachedObject
}

type CachedChildren []*CachedChild

// Retain registers a new consumer for the cached children.
// child +1
func (cachedChildren CachedChildren) Retain() CachedChildren {
	cachedResult := make(CachedChildren, len(cachedChildren))
	for i, cachedChild := range cachedChildren {
		cachedResult[i] = cachedChild.Retain() // child +1
	}
	return cachedResult
}

// Release releases the cached children, to be picked up by the persistence layer (as soon as all consumers are done).
func (cachedChildren CachedChildren) Release(force ...bool) {
	for _, cachedChild := range cachedChildren {
		cachedChild.Release(force...) // child -1
	}
}

// Retain registers a new consumer for the cached child.
// child +1
func (c *CachedChild) Retain() *CachedChild {
	return &CachedChild{c.CachedObject.Retain()} // child +1
}

// Child retrieves the child, that is cached in this container.
func (c *CachedChild) Child() *Child {
	return c.Get().(*Child)
}

func childrenFactory(key []byte, _ []byte) (objectstorage.StorableObject, error) {
	child := NewChild(hornet.BlockIDFromSlice(key[:iotago.BlockIDLength]), hornet.BlockIDFromSlice(key[iotago.BlockIDLength:iotago.BlockIDLength+iotago.BlockIDLength]))
	return child, nil
}

func (s *Storage) ChildrenStorageSize() int {
	return s.childrenStorage.GetSize()
}

func (s *Storage) configureChildrenStorage(store kvstore.KVStore, opts *profile.CacheOpts) error {

	cacheTime, err := time.ParseDuration(opts.CacheTime)
	if err != nil {
		return err
	}

	leakDetectionMaxConsumerHoldTime, err := time.ParseDuration(opts.LeakDetectionOptions.MaxConsumerHoldTime)
	if err != nil {
		return err
	}

	childrenStore, err := store.WithRealm([]byte{common.StorePrefixChildren})
	if err != nil {
		return err
	}

	s.childrenStorage = objectstorage.New(
		childrenStore,
		childrenFactory,
		objectstorage.CacheTime(cacheTime),
		objectstorage.PartitionKey(iotago.BlockIDLength, iotago.BlockIDLength),
		objectstorage.PersistenceEnabled(true),
		objectstorage.KeysOnly(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.ReleaseExecutorWorkerCount(opts.ReleaseExecutorWorkerCount),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   leakDetectionMaxConsumerHoldTime,
			}),
	)

	return nil
}

// ChildrenBlockIDs returns the block IDs of the children of the given block.
// children +-0
func (s *Storage) ChildrenBlockIDs(blockID hornet.BlockID, iteratorOptions ...IteratorOption) (hornet.BlockIDs, error) {
	var childrenBlockIDs hornet.BlockIDs

	s.childrenStorage.ForEachKeyOnly(func(key []byte) bool {
		childrenBlockIDs = append(childrenBlockIDs, hornet.BlockIDFromSlice(key[iotago.BlockIDLength:iotago.BlockIDLength+iotago.BlockIDLength]))
		return true
	}, append(ObjectStorageIteratorOptions(iteratorOptions...), objectstorage.WithIteratorPrefix(blockID))...)

	return childrenBlockIDs, nil
}

// ContainsChild returns if the given child exists in the cache/persistence layer.
func (s *Storage) ContainsChild(blockID hornet.BlockID, childBlockID hornet.BlockID, readOptions ...ReadOption) bool {
	return s.childrenStorage.Contains(append(blockID, childBlockID...), readOptions...)
}

// CachedChildrenOfBlockID returns the cached children of a block.
// children +1
func (s *Storage) CachedChildrenOfBlockID(blockID hornet.BlockID, iteratorOptions ...IteratorOption) CachedChildren {

	cachedChildren := make(CachedChildren, 0)
	s.childrenStorage.ForEach(func(_ []byte, cachedObject objectstorage.CachedObject) bool {
		cachedChildren = append(cachedChildren, &CachedChild{CachedObject: cachedObject})
		return true
	}, append(ObjectStorageIteratorOptions(iteratorOptions...), objectstorage.WithIteratorPrefix(blockID))...)
	return cachedChildren
}

// ChildConsumer consumes the given child during looping through all children.
type ChildConsumer func(blockID hornet.BlockID, childBlockID hornet.BlockID) bool

// ForEachChild loops over all children.
func (s *Storage) ForEachChild(consumer ChildConsumer, iteratorOptions ...IteratorOption) {

	s.childrenStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(hornet.BlockIDFromSlice(key[:iotago.BlockIDLength]), hornet.BlockIDFromSlice(key[iotago.BlockIDLength:iotago.BlockIDLength+iotago.BlockIDLength]))
	}, ObjectStorageIteratorOptions(iteratorOptions...)...)
}

// ForEachChild loops over all children.
func (ns *NonCachedStorage) ForEachChild(consumer ChildConsumer, iteratorOptions ...IteratorOption) {

	ns.storage.childrenStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(hornet.BlockIDFromSlice(key[:iotago.BlockIDLength]), hornet.BlockIDFromSlice(key[iotago.BlockIDLength:iotago.BlockIDLength+iotago.BlockIDLength]))
	}, append(ObjectStorageIteratorOptions(iteratorOptions...), objectstorage.WithIteratorSkipCache(true))...)
}

// StoreChild stores the child in the persistence layer and returns a cached object.
// child +1
func (s *Storage) StoreChild(parentBlockID hornet.BlockID, childBlockID hornet.BlockID) *CachedChild {
	child := NewChild(parentBlockID, childBlockID)
	return &CachedChild{CachedObject: s.childrenStorage.Store(child)}
}

// DeleteChild deletes the child in the cache/persistence layer.
// child +-0
func (s *Storage) DeleteChild(blockID hornet.BlockID, childBlockID hornet.BlockID) {
	child := NewChild(blockID, childBlockID)
	s.childrenStorage.Delete(child.ObjectStorageKey())
}

// DeleteChildren deletes the children of the given block in the cache/persistence layer.
// child +-0
func (s *Storage) DeleteChildren(blockID hornet.BlockID, iteratorOptions ...IteratorOption) {

	var keysToDelete [][]byte

	s.childrenStorage.ForEachKeyOnly(func(key []byte) bool {
		keysToDelete = append(keysToDelete, key)
		return true
	}, append(ObjectStorageIteratorOptions(iteratorOptions...), objectstorage.WithIteratorPrefix(blockID))...)

	for _, key := range keysToDelete {
		s.childrenStorage.Delete(key)
	}
}

// ShutdownChildrenStorage shuts down the children storage.
func (s *Storage) ShutdownChildrenStorage() {
	s.childrenStorage.Shutdown()
}

// FlushChildrenStorage flushes the children storage.
func (s *Storage) FlushChildrenStorage() {
	s.childrenStorage.Flush()
}
