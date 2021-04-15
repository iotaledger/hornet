package storage

import (
	"time"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	iotago "github.com/iotaledger/iota.go/v2"
)

type CachedChild struct {
	objectstorage.CachedObject
}

type CachedChildren []*CachedChild

func (cachedChildren CachedChildren) Retain() CachedChildren {
	cachedResult := make(CachedChildren, len(cachedChildren))
	for i, cachedChild := range cachedChildren {
		cachedResult[i] = cachedChild.Retain()
	}
	return cachedResult
}

func (cachedChildren CachedChildren) Release(force ...bool) {
	for _, cachedChild := range cachedChildren {
		cachedChild.Release(force...)
	}
}

func (c *CachedChild) Retain() *CachedChild {
	return &CachedChild{c.CachedObject.Retain()}
}

func (c *CachedChild) GetChild() *Child {
	return c.Get().(*Child)
}

func childrenFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	child := NewChild(hornet.MessageIDFromSlice(key[:iotago.MessageIDLength]), hornet.MessageIDFromSlice(key[iotago.MessageIDLength:iotago.MessageIDLength+iotago.MessageIDLength]))
	return child, nil
}

func (s *Storage) GetChildrenStorageSize() int {
	return s.childrenStorage.GetSize()
}

func (s *Storage) configureChildrenStorage(store kvstore.KVStore, opts *profile.CacheOpts) {

	cacheTime, _ := time.ParseDuration(opts.CacheTime)
	leakDetectionMaxConsumerHoldTime, _ := time.ParseDuration(opts.LeakDetectionOptions.MaxConsumerHoldTime)

	s.childrenStorage = objectstorage.New(
		store.WithRealm([]byte{common.StorePrefixChildren}),
		childrenFactory,
		objectstorage.CacheTime(cacheTime),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(iotago.MessageIDLength, iotago.MessageIDLength),
		objectstorage.KeysOnly(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.ReleaseExecutorWorkerCount(opts.ReleaseExecutorWorkerCount),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   leakDetectionMaxConsumerHoldTime,
			}),
	)
}

// children +-0
func (s *Storage) GetChildrenMessageIDs(messageID hornet.MessageID, iteratorOptions ...IteratorOption) hornet.MessageIDs {
	var childrenMessageIDs hornet.MessageIDs

	s.childrenStorage.ForEachKeyOnly(func(key []byte) bool {
		childrenMessageIDs = append(childrenMessageIDs, hornet.MessageIDFromSlice(key[iotago.MessageIDLength:iotago.MessageIDLength+iotago.MessageIDLength]))
		return true
	}, append(iteratorOptions, objectstorage.WithIteratorPrefix(messageID))...)

	return childrenMessageIDs
}

// ContainsChild returns if the given child exists in the cache/persistence layer.
func (s *Storage) ContainsChild(messageID hornet.MessageID, childMessageID hornet.MessageID, readOptions ...ReadOption) bool {
	return s.childrenStorage.Contains(append(messageID, childMessageID...), readOptions...)
}

// GetCachedChildrenOfMessageID returns the cached children of a message.
// children +1
func (s *Storage) GetCachedChildrenOfMessageID(messageID hornet.MessageID, iteratorOptions ...IteratorOption) CachedChildren {
	cachedChildren := make(CachedChildren, 0)
	s.childrenStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedChildren = append(cachedChildren, &CachedChild{CachedObject: cachedObject})
		return true
	}, append(iteratorOptions, objectstorage.WithIteratorPrefix(messageID))...)
	return cachedChildren
}

// ChildConsumer consumes the given child during looping through all children.
type ChildConsumer func(messageID hornet.MessageID, childMessageID hornet.MessageID) bool

// ForEachChild loops over all children.
func (s *Storage) ForEachChild(consumer ChildConsumer, iteratorOptions ...IteratorOption) {
	s.childrenStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(hornet.MessageIDFromSlice(key[:iotago.MessageIDLength]), hornet.MessageIDFromSlice(key[iotago.MessageIDLength:iotago.MessageIDLength+iotago.MessageIDLength]))
	}, iteratorOptions...)
}

// child +1
func (s *Storage) StoreChild(parentMessageID hornet.MessageID, childMessageID hornet.MessageID) *CachedChild {
	child := NewChild(parentMessageID, childMessageID)
	return &CachedChild{CachedObject: s.childrenStorage.Store(child)}
}

// child +-0
func (s *Storage) DeleteChild(messageID hornet.MessageID, childMessageID hornet.MessageID) {
	child := NewChild(messageID, childMessageID)
	s.childrenStorage.Delete(child.ObjectStorageKey())
}

// child +-0
func (s *Storage) DeleteChildren(messageID hornet.MessageID, iteratorOptions ...IteratorOption) {

	var keysToDelete [][]byte

	s.childrenStorage.ForEachKeyOnly(func(key []byte) bool {
		keysToDelete = append(keysToDelete, key)
		return true
	}, append(iteratorOptions, objectstorage.WithIteratorPrefix(messageID))...)

	for _, key := range keysToDelete {
		s.childrenStorage.Delete(key)
	}
}

func (s *Storage) ShutdownChildrenStorage() {
	s.childrenStorage.Shutdown()
}

func (s *Storage) FlushChildrenStorage() {
	s.childrenStorage.Flush()
}
