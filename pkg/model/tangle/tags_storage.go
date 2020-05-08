package tangle

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/database"
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
	tag := &hornet.Tag{
		Tag:    make([]byte, 17),
		TxHash: make([]byte, 49),
	}
	copy(tag.Tag, key[:17])
	copy(tag.TxHash, key[17:])
	return tag, 66, nil
}

func GetTagsStorageSize() int {
	return tagsStorage.GetSize()
}

func configureTagsStorage() {

	opts := profile.LoadProfile().Caches.Tags

	tagsStorage = objectstorage.New(
		database.StorageWithPrefix(DBPrefixTags),
		tagsFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(17, 49),
		objectstorage.KeysOnly(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// tag +-0
func GetTagHashes(txTag trinary.Trytes, forceRelease bool, maxFind ...int) []trinary.Hash {
	var tagHashes []trinary.Hash

	i := 0
	tagsStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			cachedObject.Release(true) // tag -1
			return false
		}

		if !cachedObject.Exists() {
			cachedObject.Release(true) // tag -1
			return true
		}

		tagHashes = append(tagHashes, (&CachedTag{CachedObject: cachedObject}).GetTag().GetTransactionHash())
		cachedObject.Release(forceRelease) // tag -1
		return true
	}, trinary.MustTrytesToBytes(trinary.MustPad(txTag, 27))[:17])

	return tagHashes
}

// tag +1
func StoreTag(txTag trinary.Trytes, txHash trinary.Hash) *CachedTag {

	tag := &hornet.Tag{
		Tag:    trinary.MustTrytesToBytes(trinary.MustPad(txTag, 27))[:17],
		TxHash: trinary.MustTrytesToBytes(txHash)[:49],
	}

	cachedObj := tagsStorage.ComputeIfAbsent(tag.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject { // tag +1
		tag.Persist()
		tag.SetModified()
		return tag
	})

	return &CachedTag{CachedObject: cachedObj}
}

// tag +-0
func DeleteTag(txTag trinary.Trytes, txHash trinary.Hash) {
	tagsStorage.Delete(append(trinary.MustTrytesToBytes(trinary.MustPad(txTag, 27))[:17], trinary.MustTrytesToBytes(txHash)[:49]...))
}

// DeleteTagFromBadger deletes the tag from the persistence layer without accessing the cache.
func DeleteTagFromBadger(txTag trinary.Trytes, txHashBytes []byte) {
	tagsStorage.DeleteEntryFromBadger(append(trinary.MustTrytesToBytes(trinary.MustPad(txTag, 27))[:17], txHashBytes...))
}

// tag +-0
func DeleteTags(txTag trinary.Trytes) {

	tagsStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		tagsStorage.Delete(key)
		cachedObject.Release(true)
		return true
	}, trinary.MustTrytesToBytes(trinary.MustPad(txTag, 27))[:17])
}

func ShutdownTagsStorage() {
	tagsStorage.Shutdown()
}

func FlushTagsStorage() {
	tagsStorage.Flush()
}
