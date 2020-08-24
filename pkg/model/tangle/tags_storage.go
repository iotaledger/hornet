package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/profile"
)

var tagsStorage *objectstorage.ObjectStorage

type CachedTag struct {
	objectstorage.CachedObject
}

type CachedTags []*CachedTag

// tag -1
func (cachedTags CachedTags) Release(force ...bool) {
	for _, cachedTag := range cachedTags {
		cachedTag.Release(force...)
	}
}

func (c *CachedTag) GetTag() *hornet.Tag {
	return c.Get().(*hornet.Tag)
}

func tagsFactory(key []byte) (objectstorage.StorableObject, int, error) {
	tag := hornet.NewTag(key[:17], key[17:66])
	return tag, 66, nil
}

func GetTagsStorageSize() int {
	return tagsStorage.GetSize()
}

func configureTagsStorage(store kvstore.KVStore, opts profile.CacheOpts) {

	tagsStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixTags}),
		tagsFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(17, 49),
		objectstorage.KeysOnly(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// tag +-0
func GetTagHashes(txTag hornet.Hash, forceRelease bool, maxFind ...int) hornet.Hashes {
	var tagHashes hornet.Hashes

	i := 0
	tagsStorage.ForEachKeyOnly(func(key []byte) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			return false
		}

		tagHashes = append(tagHashes, hornet.Hash(key[17:66]))
		return true
	}, false, txTag)

	return tagHashes
}

// ContainsTag returns if the given tag exists in the cache/persistence layer.
func ContainsTag(txTag hornet.Hash, txHash hornet.Hash) bool {
	return tagsStorage.Contains(append(txTag, txHash...))
}

// TagConsumer consumes the given tag during looping through all tags in the persistence layer.
type TagConsumer func(txTag hornet.Hash, txHash hornet.Hash) bool

// ForEachTag loops over all tags.
func ForEachTag(consumer TagConsumer, skipCache bool) {
	tagsStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(key[:17], key[17:66])
	}, skipCache)
}

// tag +1
func StoreTag(txTag hornet.Hash, txHash hornet.Hash) *CachedTag {
	tag := hornet.NewTag(txTag[:17], txHash[:49])
	return &CachedTag{CachedObject: tagsStorage.Store(tag)}
}

// tag +-0
func DeleteTag(txTag hornet.Hash, txHash hornet.Hash) {
	tagsStorage.Delete(append(txTag[:17], txHash[:49]...))
}

func ShutdownTagsStorage() {
	tagsStorage.Shutdown()
}

func FlushTagsStorage() {
	tagsStorage.Flush()
}
