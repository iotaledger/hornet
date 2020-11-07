package tangle

import (
	"time"

	iotago "github.com/iotaledger/iota.go"

	"github.com/dchest/blake2b"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/profile"
)

type CachedIndexation struct {
	objectstorage.CachedObject
}

func (c *CachedIndexation) GetIndexation() *Indexation {
	return c.Get().(*Indexation)
}

func indexationFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	return &Indexation{
		indexationHash: hornet.MessageIDFromBytes(key[:IndexationHashLength]),
		messageID:      hornet.MessageIDFromBytes(key[IndexationHashLength : IndexationHashLength+iotago.MessageIDLength]),
	}, nil
}

func (t *Tangle) GetIndexationStorageSize() int {
	return t.indexationStorage.GetSize()
}

func (t *Tangle) configureIndexationStorage(store kvstore.KVStore, opts *profile.CacheOpts) {

	t.indexationStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixIndexation}),
		indexationFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(IndexationHashLength, iotago.MessageIDLength),
		objectstorage.KeysOnly(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// indexation +-0
func (t *Tangle) GetIndexMessageIDs(index string, maxFind ...int) hornet.MessageIDs {
	var messageIDs hornet.MessageIDs

	indexationHash := blake2b.Sum256([]byte(index))

	i := 0
	t.indexationStorage.ForEachKeyOnly(func(key []byte) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			return false
		}

		messageIDs = append(messageIDs, hornet.MessageIDFromBytes(key[IndexationHashLength:IndexationHashLength+iotago.MessageIDLength]))
		return true
	}, false, indexationHash[:])

	return messageIDs
}

// IndexConsumer consumes the messageID during looping through all messages with given index in the persistence layer.
type IndexConsumer func(messageID *hornet.MessageID) bool

// ForEachMessageIDWithIndex loops over all messages with the given index.
func (t *Tangle) ForEachMessageIDWithIndex(index string, consumer IndexConsumer, skipCache bool) {
	indexationHash := blake2b.Sum256([]byte(index))

	t.indexationStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(hornet.MessageIDFromBytes(key[IndexationHashLength : IndexationHashLength+iotago.MessageIDLength]))
	}, skipCache, indexationHash[:])
}

// indexation +1
func (t *Tangle) StoreIndexation(index string, messageID *hornet.MessageID) *CachedIndexation {
	indexation := NewIndexation(index, messageID)
	return &CachedIndexation{CachedObject: t.indexationStorage.Store(indexation)}
}

// indexation +-0
func (t *Tangle) DeleteIndexation(index string, messageID *hornet.MessageID) {
	indexation := NewIndexation(index, messageID)
	t.indexationStorage.Delete(indexation.ObjectStorageKey())
}

func (t *Tangle) ShutdownIndexationStorage() {
	t.indexationStorage.Shutdown()
}

func (t *Tangle) FlushIndexationStorage() {
	t.indexationStorage.Flush()
}
