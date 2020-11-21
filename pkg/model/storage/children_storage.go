package storage

import (
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/profile"
)

type CachedChild struct {
	objectstorage.CachedObject
}

type CachedChildren []*CachedChild

func (cachedChildren CachedChildren) Release(force ...bool) {
	for _, cachedChild := range cachedChildren {
		cachedChild.Release(force...)
	}
}

func (c *CachedChild) GetChild() *Child {
	return c.Get().(*Child)
}

func childrenFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	child := NewChild(hornet.MessageIDFromBytes(key[:iotago.MessageIDLength]), hornet.MessageIDFromBytes(key[iotago.MessageIDLength:iotago.MessageIDLength+iotago.MessageIDLength]))
	return child, nil
}

func (s *Storage) GetChildrenStorageSize() int {
	return s.childrenStorage.GetSize()
}

func (s *Storage) configureChildrenStorage(store kvstore.KVStore, opts *profile.CacheOpts) {

	s.childrenStorage = objectstorage.New(
		store.WithRealm([]byte{common.StorePrefixChildren}),
		childrenFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(iotago.MessageIDLength, iotago.MessageIDLength),
		objectstorage.KeysOnly(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// children +-0
func (s *Storage) GetChildrenMessageIDs(messageID *hornet.MessageID, maxFind ...int) hornet.MessageIDs {
	var childrenMessageIDs hornet.MessageIDs

	i := 0
	s.childrenStorage.ForEachKeyOnly(func(key []byte) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			return false
		}

		childrenMessageIDs = append(childrenMessageIDs, hornet.MessageIDFromBytes(key[iotago.MessageIDLength:iotago.MessageIDLength+iotago.MessageIDLength]))
		return true
	}, false, messageID.Slice())

	return childrenMessageIDs
}

// ContainsChild returns if the given child exists in the cache/persistence layer.
func (s *Storage) ContainsChild(messageID *hornet.MessageID, childMessageID *hornet.MessageID) bool {
	return s.childrenStorage.Contains(append(messageID.Slice(), childMessageID.Slice()...))
}

// ChildConsumer consumes the given child during looping through all children in the persistence layer.
type ChildConsumer func(messageID *hornet.MessageID, childMessageID *hornet.MessageID) bool

// ForEachChild loops over all children.
func (s *Storage) ForEachChild(consumer ChildConsumer, skipCache bool) {
	s.childrenStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(hornet.MessageIDFromBytes(key[:iotago.MessageIDLength]), hornet.MessageIDFromBytes(key[iotago.MessageIDLength:iotago.MessageIDLength+iotago.MessageIDLength]))
	}, skipCache)
}

// child +1
func (s *Storage) StoreChild(parentMessageID *hornet.MessageID, childMessageID *hornet.MessageID) *CachedChild {
	child := NewChild(parentMessageID, childMessageID)
	return &CachedChild{CachedObject: s.childrenStorage.Store(child)}
}

// child +-0
func (s *Storage) DeleteChild(messageID *hornet.MessageID, childMessageID *hornet.MessageID) {
	child := NewChild(messageID, childMessageID)
	s.childrenStorage.Delete(child.ObjectStorageKey())
}

// child +-0
func (s *Storage) DeleteChildren(messageID *hornet.MessageID) {

	var keysToDelete [][]byte

	s.childrenStorage.ForEachKeyOnly(func(key []byte) bool {
		keysToDelete = append(keysToDelete, key)
		return true
	}, false, messageID.Slice())

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
