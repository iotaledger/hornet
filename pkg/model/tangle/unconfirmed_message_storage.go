package tangle

import (
	"encoding/binary"
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/profile"
)

var unreferencedMessagesStorage *objectstorage.ObjectStorage

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

	unreferencedTx := NewUnreferencedMessage(milestone.Index(binary.LittleEndian.Uint32(key[:4])), hornet.MessageIDFromBytes(key[4:36]))
	return unreferencedTx, nil
}

func configureUnreferencedMessageStorage(store kvstore.KVStore, opts profile.CacheOpts) {

	unreferencedMessagesStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixUnreferencedMessages}),
		unreferencedMessageFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(4, 32),
		objectstorage.KeysOnly(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// GetUnreferencedMessageIDs returns all message IDs of unreferenced messages for that milestone.
func GetUnreferencedMessageIDs(msIndex milestone.Index, forceRelease bool) hornet.MessageIDs {

	var unreferencedMessageIDs hornet.MessageIDs

	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(msIndex))

	unreferencedMessagesStorage.ForEachKeyOnly(func(key []byte) bool {
		unreferencedMessageIDs = append(unreferencedMessageIDs, hornet.MessageIDFromBytes(key[4:36]))
		return true
	}, false, key)

	return unreferencedMessageIDs
}

// UnreferencedMessageConsumer consumes the given unreferenced message during looping through all unreferenced messages in the persistence layer.
type UnreferencedMessageConsumer func(msIndex milestone.Index, messageID *hornet.MessageID) bool

// ForEachUnreferencedMessage loops over all unreferenced messages.
func ForEachUnreferencedMessage(consumer UnreferencedMessageConsumer, skipCache bool) {
	unreferencedMessagesStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(milestone.Index(binary.LittleEndian.Uint32(key[:4])), hornet.MessageIDFromBytes(key[4:36]))
	}, skipCache)
}

// unreferencedTx +1
func StoreUnreferencedMessage(msIndex milestone.Index, messageID *hornet.MessageID) *CachedUnreferencedMessage {
	unreferencedTx := NewUnreferencedMessage(msIndex, messageID)
	return &CachedUnreferencedMessage{CachedObject: unreferencedMessagesStorage.Store(unreferencedTx)}
}

// DeleteUnreferencedMessages deletes unreferenced message entries.
func DeleteUnreferencedMessages(msIndex milestone.Index) int {

	msIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(msIndexBytes, uint32(msIndex))

	var keysToDelete [][]byte

	unreferencedMessagesStorage.ForEachKeyOnly(func(key []byte) bool {
		keysToDelete = append(keysToDelete, key)
		return true
	}, false, msIndexBytes)

	for _, key := range keysToDelete {
		unreferencedMessagesStorage.Delete(key)
	}

	return len(keysToDelete)
}

func ShutdownUnreferencedMessagesStorage() {
	unreferencedMessagesStorage.Shutdown()
}

func FlushUnreferencedMessagesStorage() {
	unreferencedMessagesStorage.Flush()
}
