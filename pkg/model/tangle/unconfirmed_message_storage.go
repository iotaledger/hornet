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

var unconfirmedMessagesStorage *objectstorage.ObjectStorage

type CachedUnconfirmedMessage struct {
	objectstorage.CachedObject
}

type CachedUnconfirmedMessages []*CachedUnconfirmedMessage

func (cachedUnconfirmedMessages CachedUnconfirmedMessages) Release(force ...bool) {
	for _, cachedUnconfirmedMessage := range cachedUnconfirmedMessages {
		cachedUnconfirmedMessage.Release(force...)
	}
}

func (c *CachedUnconfirmedMessage) GetUnconfirmedMessage() *UnconfirmedMessage {
	return c.Get().(*UnconfirmedMessage)
}

func unconfirmedMessageFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {

	unconfirmedTx := NewUnconfirmedMessage(milestone.Index(binary.LittleEndian.Uint32(key[:4])), hornet.MessageIDFromBytes(key[4:36]))
	return unconfirmedTx, nil
}

func configureUnconfirmedMessageStorage(store kvstore.KVStore, opts profile.CacheOpts) {

	unconfirmedMessagesStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixUnconfirmedMessages}),
		unconfirmedMessageFactory,
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

// GetUnconfirmedMessageIDs returns all message IDs of unconfirmed messages for that milestone.
func GetUnconfirmedMessageIDs(msIndex milestone.Index, forceRelease bool) hornet.MessageIDs {

	var unconfirmedMessageIDs hornet.MessageIDs

	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(msIndex))

	unconfirmedMessagesStorage.ForEachKeyOnly(func(key []byte) bool {
		unconfirmedMessageIDs = append(unconfirmedMessageIDs, hornet.MessageIDFromBytes(key[4:36]))
		return true
	}, false, key)

	return unconfirmedMessageIDs
}

// UnconfirmedMessageConsumer consumes the given unconfirmed message during looping through all unconfirmed messages in the persistence layer.
type UnconfirmedMessageConsumer func(msIndex milestone.Index, messageID *hornet.MessageID) bool

// ForEachUnconfirmedMessage loops over all unconfirmed messages.
func ForEachUnconfirmedMessage(consumer UnconfirmedMessageConsumer, skipCache bool) {
	unconfirmedMessagesStorage.ForEachKeyOnly(func(key []byte) bool {
		return consumer(milestone.Index(binary.LittleEndian.Uint32(key[:4])), hornet.MessageIDFromBytes(key[4:36]))
	}, skipCache)
}

// unconfirmedTx +1
func StoreUnconfirmedMessage(msIndex milestone.Index, messageID *hornet.MessageID) *CachedUnconfirmedMessage {
	unconfirmedTx := NewUnconfirmedMessage(msIndex, messageID)
	return &CachedUnconfirmedMessage{CachedObject: unconfirmedMessagesStorage.Store(unconfirmedTx)}
}

// DeleteUnconfirmedMessages deletes unconfirmed message entries.
func DeleteUnconfirmedMessages(msIndex milestone.Index) int {

	msIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(msIndexBytes, uint32(msIndex))

	var keysToDelete [][]byte

	unconfirmedMessagesStorage.ForEachKeyOnly(func(key []byte) bool {
		keysToDelete = append(keysToDelete, key)
		return true
	}, false, msIndexBytes)

	for _, key := range keysToDelete {
		unconfirmedMessagesStorage.Delete(key)
	}

	return len(keysToDelete)
}

func ShutdownUnconfirmedMessagesStorage() {
	unconfirmedMessagesStorage.Shutdown()
}

func FlushUnconfirmedMessagesStorage() {
	unconfirmedMessagesStorage.Flush()
}
