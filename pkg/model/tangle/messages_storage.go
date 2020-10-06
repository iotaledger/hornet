package tangle

import (
	"fmt"
	"time"

	iotago "github.com/iotaledger/iota.go"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/profile"
)

var (
	messageRealm    kvstore.KVStore
	messagesStorage *objectstorage.ObjectStorage
	metadataStorage *objectstorage.ObjectStorage
)

func MessageCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMsg *CachedMessage))(params[0].(*CachedMessage).Retain())
}

func MessageMetadataCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMsgMeta *CachedMetadata))(params[0].(*CachedMetadata).Retain())
}

func MessageIDCaller(handler interface{}, params ...interface{}) {
	handler.(func(messageID *hornet.MessageID))(params[0].(*hornet.MessageID))
}

func NewMessageCaller(handler interface{}, params ...interface{}) {
	handler.(func(cachedMsg *CachedMessage, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index))(params[0].(*CachedMessage).Retain(), params[1].(milestone.Index), params[2].(milestone.Index))
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

// msg +1
func (cachedMsgs CachedMessages) Retain() CachedMessages {
	cachedResult := CachedMessages{}
	for _, cachedMsg := range cachedMsgs {
		cachedResult = append(cachedResult, cachedMsg.Retain())
	}
	return cachedResult
}

// msg -1
func (cachedMsgs CachedMessages) Release(force ...bool) {
	for _, cachedMsg := range cachedMsgs {
		cachedMsg.Release(force...)
	}
}

func (c *CachedMessage) GetMessage() *Message {
	return c.msg.Get().(*Message)
}

// meta +1
func (c *CachedMessage) GetCachedMetadata() *CachedMetadata {
	return &CachedMetadata{c.metadata.Retain()}
}

func (c *CachedMessage) GetMetadata() *MessageMetadata {
	return c.metadata.Get().(*MessageMetadata)
}

func (c *CachedMetadata) GetMetadata() *MessageMetadata {
	return c.Get().(*MessageMetadata)
}

// msg +1
func (c *CachedMessage) Retain() *CachedMessage {
	return &CachedMessage{
		c.msg.Retain(),
		c.metadata.Retain(),
	}
}

func (c *CachedMetadata) Retain() *CachedMetadata {
	return &CachedMetadata{c.CachedObject.Retain()}
}

func (c *CachedMessage) Exists() bool {
	return c.msg.Exists()
}

// msg -1
// meta -1
func (c *CachedMessage) ConsumeMessageAndMetadata(consumer func(*Message, *MessageMetadata)) {

	c.msg.Consume(func(txObject objectstorage.StorableObject) {
		c.metadata.Consume(func(metadataObject objectstorage.StorableObject) {
			consumer(txObject.(*Message), metadataObject.(*MessageMetadata))
		}, true)
	}, true)
}

// msg -1
// meta -1
func (c *CachedMessage) ConsumeMessage(consumer func(*Message)) {
	defer c.metadata.Release(true)
	c.msg.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Message))
	}, true)
}

// msg -1
// meta -1
func (c *CachedMessage) ConsumeMetadata(consumer func(*MessageMetadata)) {
	defer c.msg.Release(true)
	c.metadata.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*MessageMetadata))
	}, true)
}

// meta -1
func (c *CachedMetadata) ConsumeMetadata(consumer func(*MessageMetadata)) {
	c.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*MessageMetadata))
	}, true)
}

// msg -1
func (c *CachedMessage) Release(force ...bool) {
	c.msg.Release(force...)
	c.metadata.Release(force...)
}

func messageFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	msg := &Message{
		messageID: hornet.MessageIDFromBytes(key[:iotago.MessageHashLength]),
		message:   &iotago.Message{},
	}

	if _, err := msg.message.Deserialize(data, iotago.DeSeriModeNoValidation); err != nil {
		return nil, err
	}

	return msg, nil
}

func GetMessageStorageSize() int {
	return messagesStorage.GetSize()
}

func configureMessageStorage(store kvstore.KVStore, opts profile.CacheOpts) {

	messageRealm = store.WithRealm([]byte{StorePrefixMessages})

	messagesStorage = objectstorage.New(
		messageRealm,
		messageFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)

	metadataStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixMessageMetadata}),
		MetadataFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.StoreOnCreation(false),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// msg +1
func GetCachedMessageOrNil(messageID *hornet.MessageID) *CachedMessage {
	cachedMsg := messagesStorage.Load(messageID.Slice()) // msg +1
	if !cachedMsg.Exists() {
		cachedMsg.Release(true) // msg -1
		return nil
	}

	cachedMeta := metadataStorage.Load(messageID.Slice()) // meta +1
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

// metadata +1
func GetCachedMessageMetadataOrNil(messageID *hornet.MessageID) *CachedMetadata {
	cachedMeta := metadataStorage.Load(messageID.Slice()) // meta +1
	if !cachedMeta.Exists() {
		cachedMeta.Release(true) // metadata -1
		return nil
	}
	return &CachedMetadata{CachedObject: cachedMeta}
}

// GetStoredMetadataOrNil returns a metadata object without accessing the cache layer.
func GetStoredMetadataOrNil(messageID *hornet.MessageID) *MessageMetadata {
	storedMeta := metadataStorage.LoadObjectFromStore(messageID.Slice())
	if storedMeta == nil {
		return nil
	}
	return storedMeta.(*MessageMetadata)
}

// ContainsMessage returns if the given message exists in the cache/persistence layer.
func ContainsMessage(messageID *hornet.MessageID) bool {
	return messagesStorage.Contains(messageID.Slice())
}

// MessageExistsInStore returns if the given message exists in the persistence layer.
func MessageExistsInStore(messageID *hornet.MessageID) bool {
	return messagesStorage.ObjectExistsInStore(messageID.Slice())
}

// msg +1
func StoreMessageIfAbsent(message *Message) (cachedMsg *CachedMessage, newlyAdded bool) {

	// Store msg + metadata atomically in the same callback
	var cachedMeta objectstorage.CachedObject

	cachedMsgData := messagesStorage.ComputeIfAbsent(message.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject { // msg +1
		newlyAdded = true

		metadata := NewMessageMetadata(message.GetMessageID(), message.GetParent1MessageID(), message.GetParent2MessageID())
		cachedMeta = metadataStorage.Store(metadata) // meta +1

		message.Persist()
		message.SetModified()
		return message
	})

	// if we didn't create a new entry - retrieve the corresponding metadata (it should always exist since it gets created atomically)
	if !newlyAdded {
		cachedMeta = metadataStorage.Load(message.GetMessageID().Slice()) // meta +1
	}

	return &CachedMessage{msg: cachedMsgData, metadata: cachedMeta}, newlyAdded
}

// MessageIDConsumer consumes the given message ID during looping through all messages in the persistence layer.
type MessageIDConsumer func(messageID *hornet.MessageID) bool

// ForEachMessageID loops over all message IDs.
func ForEachMessageID(consumer MessageIDConsumer, skipCache bool) {
	messagesStorage.ForEachKeyOnly(func(messageID []byte) bool {
		return consumer(hornet.MessageIDFromBytes(messageID))
	}, skipCache)
}

// ForEachMessageMetadataMessageID loops over all message metadata message IDs.
func ForEachMessageMetadataMessageID(consumer MessageIDConsumer, skipCache bool) {
	metadataStorage.ForEachKeyOnly(func(messageID []byte) bool {
		return consumer(hornet.MessageIDFromBytes(messageID))
	}, skipCache)
}

// DeleteMessage deletes the message and metadata in the cache/persistence layer.
func DeleteMessage(messageID *hornet.MessageID) {
	// metadata has to be deleted before the msg, otherwise we could run into a data race in the object storage
	metadataStorage.Delete(messageID.Slice())
	messagesStorage.Delete(messageID.Slice())
}

// DeleteMessageMetadata deletes the metadata in the cache/persistence layer.
func DeleteMessageMetadata(messageID *hornet.MessageID) {
	metadataStorage.Delete(messageID.Slice())
}

func ShutdownMessagesStorage() {
	messagesStorage.Shutdown()
	metadataStorage.Shutdown()
}

func FlushMessagesStorage() {
	messagesStorage.Flush()
	metadataStorage.Flush()
}

// msg +1
func AddMessageToStorage(message *Message, latestMilestoneIndex milestone.Index, requested bool, forceRelease bool, reapply bool) (cachedMessage *CachedMessage, alreadyAdded bool) {

	cachedMessage, isNew := StoreMessageIfAbsent(message) // msg +1
	if !isNew && !reapply {
		return cachedMessage, true
	}

	StoreChild(cachedMessage.GetMessage().GetParent1MessageID(), cachedMessage.GetMessage().GetMessageID()).Release(forceRelease)
	if *cachedMessage.GetMessage().GetParent1MessageID() != *cachedMessage.GetMessage().GetParent2MessageID() {
		StoreChild(cachedMessage.GetMessage().GetParent2MessageID(), cachedMessage.GetMessage().GetMessageID()).Release(forceRelease)
	}

	indexationPayload := CheckIfIndexation(cachedMessage.GetMessage())
	if indexationPayload != nil {
		// store indexation if the message contains an indexation payload
		StoreIndexation(indexationPayload.Index, cachedMessage.GetMessage().GetMessageID()).Release(true)
	}

	// Store only non-requested messages, since all requested messages are referenced by a milestone anyway
	// This is only used to delete unreferenced messages from the database at pruning
	if !requested {
		StoreUnreferencedMessage(latestMilestoneIndex, cachedMessage.GetMessage().GetMessageID()).Release(true)
	}

	ms, err := CheckIfMilestone(message)
	if err != nil {
		// Invalid milestone
		Events.ReceivedInvalidMilestone.Trigger(fmt.Errorf("invalid milestone detected! Err: %w", err))
	}

	if ms != nil {

		cachedMilestone := storeMilestone(milestone.Index(ms.Index), cachedMessage.GetMessage().GetMessageID(), time.Unix(int64(ms.Timestamp), 0))

		Events.ReceivedValidMilestone.Trigger(cachedMilestone) // milestone pass +1

		// Force release to store milestones without caching
		cachedMilestone.Release(true) // milestone +-0
	}

	return cachedMessage, false
}

func ReadMessageBytesFromStore(messageID *hornet.MessageID) ([]byte, error) {
	messageBytes, err := messageRealm.Get(messageID.Slice())
	if err != nil {
		return nil, err
	}
	return messageBytes, nil
}
