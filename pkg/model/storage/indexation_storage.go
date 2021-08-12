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

// CachedIndexation represents a cached indexation.
type CachedIndexation struct {
	objectstorage.CachedObject
}

// Indexation retrieves the indexation, that is cached in this container.
func (c *CachedIndexation) Indexation() *Indexation {
	return c.Get().(*Indexation)
}

func indexationFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	return &Indexation{
		index:     key[:IndexationIndexLength],
		messageID: hornet.MessageIDFromSlice(key[IndexationIndexLength : IndexationIndexLength+iotago.MessageIDLength]),
	}, nil
}

func (s *Storage) IndexationStorageSize() int {
	return s.indexationStorage.GetSize()
}

func (s *Storage) configureIndexationStorage(store kvstore.KVStore, opts *profile.CacheOpts) error {

	cacheTime, err := time.ParseDuration(opts.CacheTime)
	if err != nil {
		return err
	}

	leakDetectionMaxConsumerHoldTime, err := time.ParseDuration(opts.LeakDetectionOptions.MaxConsumerHoldTime)
	if err != nil {
		return err
	}

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

	return nil
}

// IndexMessageIDs returns all known message IDs for the given index.
// indexation +-0
func (s *Storage) IndexMessageIDs(index []byte, iteratorOptions ...IteratorOption) hornet.MessageIDs {
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

// StoreIndexation stores the indexation in the persistence layer and returns a cached object.
// indexation +1
func (s *Storage) StoreIndexation(index []byte, messageID hornet.MessageID) *CachedIndexation {
	indexation := NewIndexation(index, messageID)
	return &CachedIndexation{CachedObject: s.indexationStorage.Store(indexation)}
}

// DeleteIndexation deletes the indexation in the cache/persistence layer.
// indexation +-0
func (s *Storage) DeleteIndexation(index []byte, messageID hornet.MessageID) {
	indexation := NewIndexation(index, messageID)
	s.indexationStorage.Delete(indexation.ObjectStorageKey())
}

// DeleteIndexationByKey deletes the indexation by key in the cache/persistence layer.
// indexation +-0
func (s *Storage) DeleteIndexationByKey(key []byte) {
	s.indexationStorage.Delete(key)
}

// ShutdownIndexationStorage shuts down the indexation storage.
func (s *Storage) ShutdownIndexationStorage() {
	s.indexationStorage.Shutdown()
}

// FlushIndexationStorage flushes the indexation storage.
func (s *Storage) FlushIndexationStorage() {
	s.indexationStorage.Flush()
}
