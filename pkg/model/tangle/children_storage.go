package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/profile"
)

var childrenStorage *objectstorage.ObjectStorage

type CachedChild struct {
	objectstorage.CachedObject
}

type CachedChildren []*CachedChild

func (cachedApprovers CachedChildren) Release(force ...bool) {
	for _, cachedApprover := range cachedApprovers {
		cachedApprover.Release(force...)
	}
}

func (c *CachedChild) GetChild() *hornet.Child {
	return c.Get().(*hornet.Child)
}

func childrenFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	child := hornet.NewChild(key[:32], key[32:64])
	return child, nil
}

func GetApproversStorageSize() int {
	return childrenStorage.GetSize()
}

func configureApproversStorage(store kvstore.KVStore, opts profile.CacheOpts) {

	childrenStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixChildren}),
		childrenFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(32, 32),
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
func GetChildrenMessageIDs(messageID hornet.Hash, maxFind ...int) hornet.Hashes {
	var childrenMessageIDs hornet.Hashes

	i := 0
	childrenStorage.ForEachKeyOnly(func(key []byte) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			return false
		}

		childrenMessageIDs = append(childrenMessageIDs, key[32:64])
		return true
	}, false, messageID)

	return childrenMessageIDs
}

// ContainsChild returns if the given approver exists in the cache/persistence layer.
func ContainsChild(messageID hornet.Hash, childMessageID hornet.Hash) bool {
	return childrenStorage.Contains(append(messageID, childMessageID...))
}

// ChildConsumer consumes the given approver during looping through all approvers in the persistence layer.
type ChildConsumer func(messageID hornet.Hash, childMessageID hornet.Hash) bool

// ForEachChild loops over all approvers.
func ForEachChild(consumer ChildConsumer, skipCache bool) {
	childrenStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(key[:32], key[32:64])
	}, skipCache)
}

// child +1
func StoreChild(parentMessageID hornet.Hash, childMessageID hornet.Hash) *CachedChild {
	child := hornet.NewChild(parentMessageID, childMessageID)
	return &CachedChild{CachedObject: childrenStorage.Store(child)}
}

// child +-0
func DeleteChild(messageID hornet.Hash, childMessageID hornet.Hash) {
	approver := hornet.NewChild(messageID, childMessageID)
	childrenStorage.Delete(approver.ObjectStorageKey())
}

// child +-0
func DeleteChildren(messageID hornet.Hash) {

	var keysToDelete [][]byte

	childrenStorage.ForEachKeyOnly(func(key []byte) bool {
		keysToDelete = append(keysToDelete, key)
		return true
	}, false, messageID)

	for _, key := range keysToDelete {
		childrenStorage.Delete(key)
	}
}

func ShutdownChildrenStorage() {
	childrenStorage.Shutdown()
}

func FlushChildrenStorage() {
	childrenStorage.Flush()
}
