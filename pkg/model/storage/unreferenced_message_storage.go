package storage

import (
	"encoding/binary"
	"time"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
)

type CachedUnreferencedMessage struct {
	objectstorage.CachedObject
}

type CachedUnreferencedMessages []*CachedUnreferencedMessage

func (cachedUnreferencedMessages CachedUnreferencedMessages) Release(force ...bool) {
	for _, cachedUnreferencedMessage := range cachedUnreferencedMessages {
		cachedUnreferencedMessage.Release(force...)
	}
}

func (c *CachedUnreferencedMessage) GetUnreferencedMessage() *UnreferencedMessage {
	return c.Get().(*UnreferencedMessage)
}

func unreferencedMessageFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {

	unreferencedTx := NewUnreferencedMessage(milestone.Index(binary.LittleEndian.Uint32(key[:4])), hornet.MessageIDFromSlice(key[4:36]))
	return unreferencedTx, nil
}

func (s *Storage) GetUnreferencedMessageStorageSize() int {
	return s.unreferencedMessagesStorage.GetSize()
}

func (s *Storage) configureUnreferencedMessageStorage(store kvstore.KVStore, opts *profile.CacheOpts) {

	cacheTime, _ := time.ParseDuration(opts.CacheTime)
	leakDetectionMaxConsumerHoldTime, _ := time.ParseDuration(opts.LeakDetectionOptions.MaxConsumerHoldTime)

	s.unreferencedMessagesStorage = objectstorage.New(
		store.WithRealm([]byte{common.StorePrefixUnreferencedMessages}),
		unreferencedMessageFactory,
		objectstorage.CacheTime(cacheTime),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(4, 32),
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

// GetUnreferencedMessageIDs returns all message IDs of unreferenced messages for that milestone.
func (s *Storage) GetUnreferencedMessageIDs(msIndex milestone.Index, iteratorOptions ...IteratorOption) hornet.MessageIDs {

	var unreferencedMessageIDs hornet.MessageIDs

	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(msIndex))

	s.unreferencedMessagesStorage.ForEachKeyOnly(func(key []byte) bool {
		unreferencedMessageIDs = append(unreferencedMessageIDs, hornet.MessageIDFromSlice(key[4:36]))
		return true
	}, append(iteratorOptions, objectstorage.WithIteratorPrefix(key))...)

	return unreferencedMessageIDs
}

// UnreferencedMessageConsumer consumes the given unreferenced message during looping through all unreferenced messages.
type UnreferencedMessageConsumer func(msIndex milestone.Index, messageID hornet.MessageID) bool

// ForEachUnreferencedMessage loops over all unreferenced messages.
func (s *Storage) ForEachUnreferencedMessage(consumer UnreferencedMessageConsumer, iteratorOptions ...IteratorOption) {
	s.unreferencedMessagesStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(milestone.Index(binary.LittleEndian.Uint32(key[:4])), hornet.MessageIDFromSlice(key[4:36]))
	}, iteratorOptions...)
}

// unreferencedTx +1
func (s *Storage) StoreUnreferencedMessage(msIndex milestone.Index, messageID hornet.MessageID) *CachedUnreferencedMessage {
	unreferencedTx := NewUnreferencedMessage(msIndex, messageID)
	return &CachedUnreferencedMessage{CachedObject: s.unreferencedMessagesStorage.Store(unreferencedTx)}
}

// DeleteUnreferencedMessages deletes unreferenced message entries.
func (s *Storage) DeleteUnreferencedMessages(msIndex milestone.Index, iteratorOptions ...IteratorOption) int {

	msIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(msIndexBytes, uint32(msIndex))

	var keysToDelete [][]byte

	s.unreferencedMessagesStorage.ForEachKeyOnly(func(key []byte) bool {
		keysToDelete = append(keysToDelete, key)
		return true
	}, append(iteratorOptions, objectstorage.WithIteratorPrefix(msIndexBytes))...)

	for _, key := range keysToDelete {
		s.unreferencedMessagesStorage.Delete(key)
	}

	return len(keysToDelete)
}

func (s *Storage) ShutdownUnreferencedMessagesStorage() {
	s.unreferencedMessagesStorage.Shutdown()
}

func (s *Storage) FlushUnreferencedMessagesStorage() {
	s.unreferencedMessagesStorage.Flush()
}
