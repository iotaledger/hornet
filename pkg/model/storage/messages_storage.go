package storage

import (
	"time"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	iotago "github.com/iotaledger/iota.go/v3"
)

func MessageCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedBlock *CachedMessage))(params[0].(*CachedMessage).Retain()) // message pass +1
}

func MessageMetadataCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedBlockMeta *CachedMetadata))(params[0].(*CachedMetadata).Retain()) // message pass +1
}

func MessageIDCaller(handler interface{}, params ...interface{}) {
	handler.(func(blockID hornet.BlockID))(params[0].(hornet.BlockID))
}

func NewMessageCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedBlock *CachedMessage, latestMilestoneIndex milestone.Index, confirmedMilestoneIndex milestone.Index))(params[0].(*CachedMessage).Retain(), params[1].(milestone.Index), params[2].(milestone.Index)) // message pass +1
}

func MessageReferencedCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedBlockMeta *CachedMetadata, msIndex milestone.Index, confTime uint32))(params[0].(*CachedMetadata).Retain(), params[1].(milestone.Index), params[2].(uint32)) // message pass +1
}

// CachedMessage contains two cached objects, one for message data and one for metadata.
type CachedMessage struct {
	msg      objectstorage.CachedObject
	metadata objectstorage.CachedObject
}

func NewCachedMessage(msg objectstorage.CachedObject, metadata objectstorage.CachedObject) *CachedMessage {
	return &CachedMessage{
		msg:      msg,
		metadata: metadata,
	}
}

// CachedMetadata contains the cached object only for metadata.
type CachedMetadata struct {
	objectstorage.CachedObject
}

func NewCachedMetadata(metadata objectstorage.CachedObject) *CachedMetadata {
	return &CachedMetadata{CachedObject: metadata}
}

type CachedMessages []*CachedMessage

// Retain registers a new consumer for the cached messages.
// message +1
func (cachedBlocks CachedMessages) Retain() CachedMessages {
	cachedResult := make(CachedMessages, len(cachedBlocks))
	for i, cachedBlock := range cachedBlocks {
		cachedResult[i] = cachedBlock.Retain() // message +1
	}
	return cachedResult
}

// Release releases the cached messages, to be picked up by the persistence layer (as soon as all consumers are done).
// message -1
func (cachedBlocks CachedMessages) Release(force ...bool) {
	for _, cachedBlock := range cachedBlocks {
		cachedBlock.Release(force...) // message -1
	}
}

// Message retrieves the message, that is cached in this container.
func (c *CachedMessage) Message() *Message {
	return c.msg.Get().(*Message)
}

// CachedMetadata returns the underlying cached metadata.
// meta +1
func (c *CachedMessage) CachedMetadata() *CachedMetadata {
	return &CachedMetadata{c.metadata.Retain()} // meta +1
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
// message +1
func (c *CachedMessage) Retain() *CachedMessage {
	return &CachedMessage{
		c.msg.Retain(),      // message +1
		c.metadata.Retain(), // meta +1
	}
}

// Retain registers a new consumer for the cached metadata.
// meta +1
func (c *CachedMetadata) Retain() *CachedMetadata {
	return &CachedMetadata{c.CachedObject.Retain()} // meta +1
}

// Exists returns true if the message in this container does exist
// (could be found in the database and was not marked as deleted).
func (c *CachedMessage) Exists() bool {
	return c.msg.Exists()
}

// ConsumeMessageAndMetadata consumes the underlying message and metadata.
// message -1
// meta -1
func (c *CachedMessage) ConsumeMessageAndMetadata(consumer func(*Message, *MessageMetadata)) {

	c.msg.Consume(func(txObject objectstorage.StorableObject) { // message -1
		c.metadata.Consume(func(metadataObject objectstorage.StorableObject) { // meta -1
			consumer(txObject.(*Message), metadataObject.(*MessageMetadata))
		}, true)
	}, true)
}

// ConsumeMessage consumes the underlying message.
// message -1
// meta -1
func (c *CachedMessage) ConsumeMessage(consumer func(*Message)) {
	defer c.metadata.Release(true)                            // meta -1
	c.msg.Consume(func(object objectstorage.StorableObject) { // message -1
		consumer(object.(*Message))
	}, true)
}

// ConsumeMetadata consumes the underlying metadata.
// message -1
// meta -1
func (c *CachedMessage) ConsumeMetadata(consumer func(*MessageMetadata)) {
	defer c.msg.Release(true)                                      // message -1
	c.metadata.Consume(func(object objectstorage.StorableObject) { // meta -1
		consumer(object.(*MessageMetadata))
	}, true)
}

// ConsumeMetadata consumes the metadata.
// meta -1
func (c *CachedMetadata) ConsumeMetadata(consumer func(*MessageMetadata)) {
	c.Consume(func(object objectstorage.StorableObject) { // meta -1
		consumer(object.(*MessageMetadata))
	}, true)
}

// Release releases the cached message and metadata, to be picked up by the persistence layer (as soon as all consumers are done).
// message -1
func (c *CachedMessage) Release(force ...bool) {
	c.msg.Release(force...)      // message -1
	c.metadata.Release(force...) // meta -1
}

func MessageFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	msg := &Message{
		blockID: hornet.BlockIDFromSlice(key[:iotago.BlockIDLength]),
		data:    data,
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

	messagesStore, err := store.WithRealm([]byte{common.StorePrefixBlocks})
	if err != nil {
		return err
	}

	s.messagesStorage = objectstorage.New(
		messagesStore,
		MessageFactory,
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

	metadataStore, err := store.WithRealm([]byte{common.StorePrefixBlockMetadata})
	if err != nil {
		return err
	}

	s.metadataStorage = objectstorage.New(
		metadataStore,
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
// message +1
func (s *Storage) CachedMessageOrNil(blockID hornet.BlockID) *CachedMessage {
	cachedBlock := s.messagesStorage.Load(blockID) // message +1
	if !cachedBlock.Exists() {
		cachedBlock.Release(true) // message -1
		return nil
	}

	cachedBlockMeta := s.metadataStorage.Load(blockID) // meta +1
	if !cachedBlockMeta.Exists() {
		cachedBlock.Release(true)     // message -1
		cachedBlockMeta.Release(true) // meta -1
		return nil
	}

	return &CachedMessage{
		msg:      cachedBlock,
		metadata: cachedBlockMeta,
	}
}

// CachedMessage returns a cached message object.
// message +1
func (s *Storage) CachedMessage(blockID hornet.BlockID) (*CachedMessage, error) {
	return s.CachedMessageOrNil(blockID), nil // message +1
}

// Message returns an iotago message object.
func (s *Storage) Message(blockID hornet.BlockID) (*iotago.Block, error) {
	cachedBlock, err := s.CachedMessage(blockID)
	if err != nil {
		return nil, err
	}

	if cachedBlock == nil {
		return nil, nil
	}

	defer cachedBlock.Release(true)
	return cachedBlock.Message().Message(), nil
}

// CachedMessageMetadataOrNil returns a cached metadata object.
// meta +1
func (s *Storage) CachedMessageMetadataOrNil(blockID hornet.BlockID) *CachedMetadata {
	cachedBlockMeta := s.metadataStorage.Load(blockID) // meta +1
	if !cachedBlockMeta.Exists() {
		cachedBlockMeta.Release(true) // meta -1
		return nil
	}
	return &CachedMetadata{CachedObject: cachedBlockMeta}
}

// CachedBlockMetadata returns a cached metadata object.
// meta +1
func (s *Storage) CachedBlockMetadata(blockID hornet.BlockID) (*CachedMetadata, error) {
	return s.CachedMessageMetadataOrNil(blockID), nil
}

// StoredMetadataOrNil returns a metadata object without accessing the cache layer.
func (s *Storage) StoredMetadataOrNil(blockID hornet.BlockID) *MessageMetadata {
	storedMeta := s.metadataStorage.LoadObjectFromStore(blockID)
	if storedMeta == nil {
		return nil
	}
	return storedMeta.(*MessageMetadata)
}

// ContainsBlock returns if the given message exists in the cache/persistence layer.
func (s *Storage) ContainsBlock(blockID hornet.BlockID, readOptions ...ReadOption) bool {
	return s.messagesStorage.Contains(blockID, readOptions...)
}

// MessageExistsInStore returns if the given message exists in the persistence layer.
func (s *Storage) MessageExistsInStore(blockID hornet.BlockID) bool {
	return s.messagesStorage.ObjectExistsInStore(blockID)
}

// MessageMetadataExistsInStore returns if the given message metadata exists in the persistence layer.
func (s *Storage) MessageMetadataExistsInStore(blockID hornet.BlockID) bool {
	return s.metadataStorage.ObjectExistsInStore(blockID)
}

// StoreMessageIfAbsent returns a cached object and stores the message in the persistence layer if it was absent.
// message +1
func (s *Storage) StoreBlockIfAbsent(message *Message) (cachedBlock *CachedMessage, newlyAdded bool) {

	// Store msg + metadata atomically in the same callback
	var cachedBlockMeta objectstorage.CachedObject

	cachedBlockData := s.messagesStorage.ComputeIfAbsent(message.ObjectStorageKey(), func(_ []byte) objectstorage.StorableObject { // message +1
		newlyAdded = true

		metadata := &MessageMetadata{
			blockID: message.MessageID(),
			parents: message.Parents(),
		}

		cachedBlockMeta = s.metadataStorage.Store(metadata) // meta +1

		message.Persist(true)
		message.SetModified(true)
		return message
	})

	// if we didn't create a new entry - retrieve the corresponding metadata (it should always exist since it gets created atomically)
	if !newlyAdded {
		cachedBlockMeta = s.metadataStorage.Load(message.MessageID()) // meta +1
	}

	return &CachedMessage{msg: cachedBlockData, metadata: cachedBlockMeta}, newlyAdded
}

// MessageIDConsumer consumes the given message ID during looping through all messages.
type MessageIDConsumer func(blockID hornet.BlockID) bool

// ForEachMessageID loops over all message IDs.
func (s *Storage) ForEachMessageID(consumer MessageIDConsumer, iteratorOptions ...IteratorOption) {

	s.messagesStorage.ForEachKeyOnly(func(blockID []byte) bool {
		return consumer(hornet.BlockIDFromSlice(blockID))
	}, ObjectStorageIteratorOptions(iteratorOptions...)...)
}

// ForEachMessageID loops over all message IDs.
func (ns *NonCachedStorage) ForEachMessageID(consumer MessageIDConsumer, iteratorOptions ...IteratorOption) {

	ns.storage.messagesStorage.ForEachKeyOnly(func(blockID []byte) bool {
		return consumer(hornet.BlockIDFromSlice(blockID))
	}, append(ObjectStorageIteratorOptions(iteratorOptions...), objectstorage.WithIteratorSkipCache(true))...)
}

// ForEachMessageMetadataMessageID loops over all message metadata message IDs.
func (s *Storage) ForEachMessageMetadataMessageID(consumer MessageIDConsumer, iteratorOptions ...IteratorOption) {

	s.metadataStorage.ForEachKeyOnly(func(blockID []byte) bool {
		return consumer(hornet.BlockIDFromSlice(blockID))
	}, ObjectStorageIteratorOptions(iteratorOptions...)...)
}

// ForEachMessageMetadataMessageID loops over all message metadata message IDs.
func (ns *NonCachedStorage) ForEachMessageMetadataMessageID(consumer MessageIDConsumer, iteratorOptions ...IteratorOption) {

	ns.storage.metadataStorage.ForEachKeyOnly(func(blockID []byte) bool {
		return consumer(hornet.BlockIDFromSlice(blockID))
	}, append(ObjectStorageIteratorOptions(iteratorOptions...), objectstorage.WithIteratorSkipCache(true))...)
}

// DeleteMessage deletes the message and metadata in the cache/persistence layer.
func (s *Storage) DeleteMessage(blockID hornet.BlockID) {
	// meta has to be deleted before the msg, otherwise we could run into a data race in the object storage
	s.metadataStorage.Delete(blockID)
	s.messagesStorage.Delete(blockID)
}

// DeleteMessageMetadata deletes the metadata in the cache/persistence layer.
func (s *Storage) DeleteMessageMetadata(blockID hornet.BlockID) {
	s.metadataStorage.Delete(blockID)
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
