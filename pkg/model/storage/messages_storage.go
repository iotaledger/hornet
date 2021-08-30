package storage

import (
	"time"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	iotago "github.com/iotaledger/iota.go/v2"
)

func MessageCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMsg *CachedMessage))(params[0].(*CachedMessage).Retain())
}

func MessageMetadataCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMsgMeta *CachedMetadata))(params[0].(*CachedMetadata).Retain())
}

func MessageIDCaller(handler interface{}, params ...interface{}) {
	handler.(func(messageID hornet.MessageID))(params[0].(hornet.MessageID))
}

func NewMessageCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMsg *CachedMessage, latestMilestoneIndex milestone.Index, confirmedMilestoneIndex milestone.Index))(params[0].(*CachedMessage).Retain(), params[1].(milestone.Index), params[2].(milestone.Index))
}

func MessageReferencedCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMeta *CachedMetadata, msIndex milestone.Index, confTime uint64))(params[0].(*CachedMetadata).Retain(), params[1].(milestone.Index), params[2].(uint64))
}

// CachedMessage contains two cached objects, one for message data and one for metadata.
type CachedMessage struct {
	msg      objectstorage.CachedObject
	metadata objectstorage.CachedObject
}

// CachedMetadata contains the cached object only for metadata.
type CachedMetadata struct {
	objectstorage.CachedObject
}

type CachedMessages []*CachedMessage

// Retain registers a new consumer for the cached messages.
// msg +1
func (cachedMsgs CachedMessages) Retain() CachedMessages {
	cachedResult := make(CachedMessages, len(cachedMsgs))
	for i, cachedMsg := range cachedMsgs {
		cachedResult[i] = cachedMsg.Retain()
	}
	return cachedResult
}

// Release releases the cached messsages, to be picked up by the persistence layer (as soon as all consumers are done).
// msg -1
func (cachedMsgs CachedMessages) Release(force ...bool) {
	for _, cachedMsg := range cachedMsgs {
		cachedMsg.Release(force...)
	}
}

// Message retrieves the message, that is cached in this container.
func (c *CachedMessage) Message() *Message {
	return c.msg.Get().(*Message)
}

// CachedMetadata returns the underlying cached metadata.
// meta +1
func (c *CachedMessage) CachedMetadata() *CachedMetadata {
	return &CachedMetadata{c.metadata.Retain()}
}

// Metadata retrieves the metadata, that is cached in this container.
func (c *CachedMessage) Metadata() *MessageMetadata {
	return c.metadata.Get().(*MessageMetadata)
}

// Metadata retrieves the metadata, that is cached in this container.
func (c *CachedMetadata) Metadata() *MessageMetadata {
	return c.Get().(*MessageMetadata)
}

// Retain registers a new consumer for the cached message and metadata.
// msg +1
func (c *CachedMessage) Retain() *CachedMessage {
	return &CachedMessage{
		c.msg.Retain(),
		c.metadata.Retain(),
	}
}

// Retain registers a new consumer for the cached metadata.
func (c *CachedMetadata) Retain() *CachedMetadata {
	return &CachedMetadata{c.CachedObject.Retain()}
}

// Exists returns true if the message in this container does exist
// (could be found in the database and was not marked as deleted).
func (c *CachedMessage) Exists() bool {
	return c.msg.Exists()
}

// ConsumeMessageAndMetadata consumes the underlying message and metadata.
// msg -1
// meta -1
func (c *CachedMessage) ConsumeMessageAndMetadata(consumer func(*Message, *MessageMetadata)) {

	c.msg.Consume(func(txObject objectstorage.StorableObject) {
		c.metadata.Consume(func(metadataObject objectstorage.StorableObject) {
			consumer(txObject.(*Message), metadataObject.(*MessageMetadata))
		}, true)
	}, true)
}

// ConsumeMessage consumes the underlying message.
// msg -1
// meta -1
func (c *CachedMessage) ConsumeMessage(consumer func(*Message)) {
	defer c.metadata.Release(true)
	c.msg.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Message))
	}, true)
}

// ConsumeMetadata consumes the underlying metadata.
// msg -1
// meta -1
func (c *CachedMessage) ConsumeMetadata(consumer func(*MessageMetadata)) {
	defer c.msg.Release(true)
	c.metadata.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*MessageMetadata))
	}, true)
}

// ConsumeMetadata consumes the metadata.
// meta -1
func (c *CachedMetadata) ConsumeMetadata(consumer func(*MessageMetadata)) {
	c.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*MessageMetadata))
	}, true)
}

// Release releases the cached message and metadata, to be picked up by the persistence layer (as soon as all consumers are done).
// msg -1
func (c *CachedMessage) Release(force ...bool) {
	c.msg.Release(force...)
	c.metadata.Release(force...)
}

func messageFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	msg := &Message{
		messageID: hornet.MessageIDFromSlice(key[:iotago.MessageIDLength]),
		data:      data,
	}

	return msg, nil
}

func (s *Storage) MessageStorageSize() int {
	return s.messagesStorage.GetSize()
}

func (s *Storage) MessageMetadataStorageSize() int {
	return s.metadataStorage.GetSize()
}

func (s *Storage) configureMessageStorage(store kvstore.KVStore, opts *profile.CacheOpts) error {

	cacheTime, err := time.ParseDuration(opts.CacheTime)
	if err != nil {
		return err
	}

	leakDetectionMaxConsumerHoldTime, err := time.ParseDuration(opts.LeakDetectionOptions.MaxConsumerHoldTime)
	if err != nil {
		return err
	}

	s.messagesStorage = objectstorage.New(
		store.WithRealm([]byte{common.StorePrefixMessages}),
		messageFactory,
		objectstorage.CacheTime(cacheTime),
		objectstorage.PersistenceEnabled(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.ReleaseExecutorWorkerCount(opts.ReleaseExecutorWorkerCount),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   leakDetectionMaxConsumerHoldTime,
			}),
	)

	s.metadataStorage = objectstorage.New(
		store.WithRealm([]byte{common.StorePrefixMessageMetadata}),
		MetadataFactory,
		objectstorage.CacheTime(cacheTime),
		objectstorage.PersistenceEnabled(true),
		objectstorage.StoreOnCreation(false),
		objectstorage.ReleaseExecutorWorkerCount(opts.ReleaseExecutorWorkerCount),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   leakDetectionMaxConsumerHoldTime,
			}),
	)

	return nil
}

// CachedMessageOrNil returns a cached message object.
// msg +1
func (s *Storage) CachedMessageOrNil(messageID hornet.MessageID) *CachedMessage {
	cachedMsg := s.messagesStorage.Load(messageID) // msg +1
	if !cachedMsg.Exists() {
		cachedMsg.Release(true) // msg -1
		return nil
	}

	cachedMeta := s.metadataStorage.Load(messageID) // meta +1
	if !cachedMeta.Exists() {
		cachedMsg.Release(true)  // msg -1
		cachedMeta.Release(true) // meta -1
		return nil
	}

	return &CachedMessage{
		msg:      cachedMsg,
		metadata: cachedMeta,
	}
}

// CachedMessageMetadataOrNil returns a cached metadata object.
// metadata +1
func (s *Storage) CachedMessageMetadataOrNil(messageID hornet.MessageID) *CachedMetadata {
	cachedMeta := s.metadataStorage.Load(messageID) // meta +1
	if !cachedMeta.Exists() {
		cachedMeta.Release(true) // metadata -1
		return nil
	}
	return &CachedMetadata{CachedObject: cachedMeta}
}

// StoredMetadataOrNil returns a metadata object without accessing the cache layer.
func (s *Storage) StoredMetadataOrNil(messageID hornet.MessageID) *MessageMetadata {
	storedMeta := s.metadataStorage.LoadObjectFromStore(messageID)
	if storedMeta == nil {
		return nil
	}
	return storedMeta.(*MessageMetadata)
}

// ContainsMessage returns if the given message exists in the cache/persistence layer.
func (s *Storage) ContainsMessage(messageID hornet.MessageID, readOptions ...ReadOption) bool {
	return s.messagesStorage.Contains(messageID, readOptions...)
}

// MessageExistsInStore returns if the given message exists in the persistence layer.
func (s *Storage) MessageExistsInStore(messageID hornet.MessageID) bool {
	return s.messagesStorage.ObjectExistsInStore(messageID)
}

// MessageMetadataExistsInStore returns if the given message metadata exists in the persistence layer.
func (s *Storage) MessageMetadataExistsInStore(messageID hornet.MessageID) bool {
	return s.metadataStorage.ObjectExistsInStore(messageID)
}

// StoreMessageIfAbsent returns a cached object and stores the message in the persistence layer if it was absent.
// msg +1
func (s *Storage) StoreMessageIfAbsent(message *Message) (cachedMsg *CachedMessage, newlyAdded bool) {

	// Store msg + metadata atomically in the same callback
	var cachedMeta objectstorage.CachedObject

	cachedMsgData := s.messagesStorage.ComputeIfAbsent(message.ObjectStorageKey(), func(_ []byte) objectstorage.StorableObject { // msg +1
		newlyAdded = true

		metadata := &MessageMetadata{
			messageID: message.MessageID(),
			parents:   hornet.MessageIDsFromSliceOfArrays(message.message.Parents),
		}

		cachedMeta = s.metadataStorage.Store(metadata) // meta +1

		message.Persist()
		message.SetModified()
		return message
	})

	// if we didn't create a new entry - retrieve the corresponding metadata (it should always exist since it gets created atomically)
	if !newlyAdded {
		cachedMeta = s.metadataStorage.Load(message.MessageID()) // meta +1
	}

	return &CachedMessage{msg: cachedMsgData, metadata: cachedMeta}, newlyAdded
}

// MessageIDConsumer consumes the given message ID during looping through all messages.
type MessageIDConsumer func(messageID hornet.MessageID) bool

// ForEachMessageID loops over all message IDs.
func (s *Storage) ForEachMessageID(consumer MessageIDConsumer, iteratorOptions ...IteratorOption) {
	s.messagesStorage.ForEachKeyOnly(func(messageID []byte) bool {
		return consumer(hornet.MessageIDFromSlice(messageID))
	}, iteratorOptions...)
}

// ForEachMessageMetadataMessageID loops over all message metadata message IDs.
func (s *Storage) ForEachMessageMetadataMessageID(consumer MessageIDConsumer, iteratorOptions ...IteratorOption) {
	s.metadataStorage.ForEachKeyOnly(func(messageID []byte) bool {
		return consumer(hornet.MessageIDFromSlice(messageID))
	}, iteratorOptions...)
}

// DeleteMessage deletes the message and metadata in the cache/persistence layer.
func (s *Storage) DeleteMessage(messageID hornet.MessageID) {
	// metadata has to be deleted before the msg, otherwise we could run into a data race in the object storage
	s.metadataStorage.Delete(messageID)
	s.messagesStorage.Delete(messageID)
}

// DeleteMessageMetadata deletes the metadata in the cache/persistence layer.
func (s *Storage) DeleteMessageMetadata(messageID hornet.MessageID) {
	s.metadataStorage.Delete(messageID)
}

// ShutdownMessagesStorage shuts down the messages storage.
func (s *Storage) ShutdownMessagesStorage() {
	s.messagesStorage.Shutdown()
	s.metadataStorage.Shutdown()
}

// FlushMessagesStorage flushes the messages storage.
func (s *Storage) FlushMessagesStorage() {
	s.messagesStorage.Flush()
	s.metadataStorage.Flush()
}

// AddMessageToStorage adds a new message to the cache/persistence layer,
// including all additional information like metadata, children,
// indexation, unreferenced messages and milestone entries.
// msg +1
func (s *Storage) AddMessageToStorage(message *Message, latestMilestoneIndex milestone.Index, requested bool, forceRelease bool, reapply bool) (cachedMessage *CachedMessage, alreadyAdded bool) {

	cachedMessage, isNew := s.StoreMessageIfAbsent(message) // msg +1
	if !isNew && !reapply {
		return cachedMessage, true
	}

	for _, parent := range message.Parents() {
		s.StoreChild(parent, cachedMessage.Message().MessageID()).Release(forceRelease)
	}

	indexationPayload := CheckIfIndexation(cachedMessage.Message())
	if indexationPayload != nil {
		// store indexation if the message contains an indexation payload
		s.StoreIndexation(indexationPayload.Index, cachedMessage.Message().MessageID()).Release(true)
	}

	// Store only non-requested messages, since all requested messages are referenced by a milestone anyway
	// This is only used to delete unreferenced messages from the database at pruning
	if !requested {
		s.StoreUnreferencedMessage(latestMilestoneIndex, cachedMessage.Message().MessageID()).Release(true)
	}

	if ms := s.VerifyMilestone(message); ms != nil {
		s.StoreMilestone(cachedMessage.Retain(), ms, requested)
	}

	return cachedMessage, false
}
