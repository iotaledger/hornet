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

type CachedIndexation struct {
	objectstorage.CachedObject
}

func (c *CachedIndexation) GetIndexation() *Indexation {
	return c.Get().(*Indexation)
}

func indexationFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	return &Indexation{
		index:     key[:IndexationIndexLength],
		messageID: hornet.MessageIDFromSlice(key[IndexationIndexLength : IndexationIndexLength+iotago.MessageIDLength]),
	}, nil
}

func (s *Storage) GetIndexationStorageSize() int {
	return s.indexationStorage.GetSize()
}

func (s *Storage) configureIndexationStorage(store kvstore.KVStore, opts *profile.CacheOpts) {

	cacheTime, _ := time.ParseDuration(opts.CacheTime)
	leakDetectionMaxConsumerHoldTime, _ := time.ParseDuration(opts.LeakDetectionOptions.MaxConsumerHoldTime)

	s.indexationStorage = objectstorage.New(
		store.WithRealm([]byte{common.StorePrefixIndexation}),
		indexationFactory,
		objectstorage.CacheTime(cacheTime),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(IndexationIndexLength, iotago.MessageIDLength),
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

// indexation +-0
func (s *Storage) GetIndexMessageIDs(index []byte, iteratorOptions ...IteratorOption) hornet.MessageIDs {
	var messageIDs hornet.MessageIDs

	indexPadded := PadIndexationIndex(index)

	s.indexationStorage.ForEachKeyOnly(func(key []byte) bool {
		messageIDs = append(messageIDs, hornet.MessageIDFromSlice(key[IndexationIndexLength:IndexationIndexLength+iotago.MessageIDLength]))
		return true
	}, append(iteratorOptions, objectstorage.WithIteratorPrefix(indexPadded[:]))...)

	return messageIDs
}

// IndexConsumer consumes the messageID during looping through all messages with given index.
type IndexConsumer func(messageID hornet.MessageID) bool

// ForEachMessageIDWithIndex loops over all messages with the given index.
func (s *Storage) ForEachMessageIDWithIndex(index []byte, consumer IndexConsumer, iteratorOptions ...IteratorOption) {
	indexPadded := PadIndexationIndex(index)

	s.indexationStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(hornet.MessageIDFromSlice(key[IndexationIndexLength : IndexationIndexLength+iotago.MessageIDLength]))
	}, append(iteratorOptions, objectstorage.WithIteratorPrefix(indexPadded[:]))...)
}

// CachedIndexationConsumer consumes the given indexation during looping through all indexations.
type CachedIndexationConsumer func(indexation *CachedIndexation) bool

// ForEachIndexation loops over all indexations.
// indexation +1
func (s *Storage) ForEachIndexation(consumer CachedIndexationConsumer, iteratorOptions ...IteratorOption) {
	s.indexationStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		return consumer(&CachedIndexation{CachedObject: cachedObject})
	}, iteratorOptions...)
}

// indexation +1
func (s *Storage) StoreIndexation(index []byte, messageID hornet.MessageID) *CachedIndexation {
	indexation := NewIndexation(index, messageID)
	return &CachedIndexation{CachedObject: s.indexationStorage.Store(indexation)}
}

// indexation +-0
func (s *Storage) DeleteIndexation(index []byte, messageID hornet.MessageID) {
	indexation := NewIndexation(index, messageID)
	s.indexationStorage.Delete(indexation.ObjectStorageKey())
}

// indexation +-0
func (s *Storage) DeleteIndexationByKey(key []byte) {
	s.indexationStorage.Delete(key)
}

func (s *Storage) ShutdownIndexationStorage() {
	s.indexationStorage.Shutdown()
}

func (s *Storage) FlushIndexationStorage() {
	s.indexationStorage.Flush()
}
