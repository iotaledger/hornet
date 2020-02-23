package tangle

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/profile"
)

var tagsStorage *objectstorage.ObjectStorage

type CachedTag struct {
	objectstorage.CachedObject
}

type CachedTags []*CachedTag

// tag -1
func (cachedTags CachedTags) Release() {
	for _, cachedTag := range cachedTags {
		cachedTag.Release()
	}
}

func (c *CachedTag) GetTag() *hornet.Tag {
	return c.Get().(*hornet.Tag)
}

func tagsFactory(key []byte) objectstorage.StorableObject {
	tag := &hornet.Tag{
		Tag:    make([]byte, 17),
		TxHash: make([]byte, 49),
	}
	copy(tag.Tag, key[:17])
	copy(tag.TxHash, key[17:])
	return tag
}

func GetTagsStorageSize() int {
	return tagsStorage.GetSize()
}

func configureTagsStorage() {

	opts := profile.GetProfile().Caches.Tags

	tagsStorage = objectstorage.New(
		database.GetHornetBadgerInstance(),
		[]byte{DBPrefixTags},
		tagsFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(17, 49),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// tag +1
func GetCachedTags(txTag trinary.Trytes, maxFind ...int) CachedTags {

	cachedTags := CachedTags{}

	i := 0
	tagsStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			cachedObject.Release() // tag -1
			return false
		}

		if !cachedObject.Exists() {
			cachedObject.Release() // tag -1
			return true
		}

		cachedTags = append(cachedTags, &CachedTag{cachedObject})
		return true
	}, trinary.MustTrytesToBytes(trinary.MustPad(txTag, 27))[:17])

	return cachedTags
}

// tag +1
func StoreTag(txTag trinary.Trytes, txHash trinary.Hash) *CachedTag {

	tag := &hornet.Tag{
		Tag:    trinary.MustTrytesToBytes(trinary.MustPad(txTag, 27))[:17],
		TxHash: trinary.MustTrytesToBytes(txHash)[:49],
	}

	return &CachedTag{tagsStorage.Store(tag)}
}

// tag +-0
func DeleteTag(txTag trinary.Trytes, txHash trinary.Hash) {
	tagsStorage.Delete(append(trinary.MustTrytesToBytes(trinary.MustPad(txTag, 27))[:17], trinary.MustTrytesToBytes(txHash)[:49]...))
}

// tag +-0
func DeleteTags(txTag trinary.Trytes) {

	tagsStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		tagsStorage.Delete(key)
		return true
	}, trinary.MustTrytesToBytes(trinary.MustPad(txTag, 27))[:17])
}

func ShutdownTagsStorage() {
	tagsStorage.Shutdown()
}
