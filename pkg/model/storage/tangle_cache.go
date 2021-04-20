package storage

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/syncutils"
)

type TangleCacheUnreferencedMessage struct {
	msg      *CachedMessage
	children CachedChildren
}

func (m *TangleCacheUnreferencedMessage) Release(forceRelease ...bool) {
	m.msg.Release(forceRelease...)
	m.children.Release(forceRelease...)
}

func CachedMessageCaller(handler interface{}, params ...interface{}) {
	handler.(func(msIndex milestone.Index, cachedMessage *CachedMessage, cachedChildren CachedChildren))(params[0].(milestone.Index), params[1].(*CachedMessage).Retain(), params[2].(CachedChildren).Retain())
}

func CachedMetadataCaller(handler interface{}, params ...interface{}) {
	handler.(func(msIndex milestone.Index, cachedMetadata *CachedMetadata))(params[0].(milestone.Index), params[1].(*CachedMetadata).Retain())
}

// NewTangleCache creates a new NewTangleCache instance.
func NewTangleCache() *TangleCache {
	return &TangleCache{
		unreferencedMessages:                  make(map[string]*TangleCacheUnreferencedMessage),
		unreferencedMessagesPerMilestoneIndex: make(map[milestone.Index]hornet.MessageIDs),
		cachedMsgs:                            make(map[milestone.Index][]*CachedMessage),
		cachedMsgMetas:                        make(map[milestone.Index][]*CachedMetadata),
		cachedChildren:                        make(map[milestone.Index][]*CachedChild),
		minMilestoneIndex:                     0,
	}
}

// TangleCache holds an object storage reference to objects, based on their milestone index.
// This is used to keep the latest cone of the tangle in the cache, without relying on cacheTimes at all.
type TangleCache struct {
	syncutils.Mutex

	unreferencedMessages                  map[string]*TangleCacheUnreferencedMessage
	unreferencedMessagesPerMilestoneIndex map[milestone.Index]hornet.MessageIDs
	cachedMsgs                            map[milestone.Index][]*CachedMessage
	cachedMsgMetas                        map[milestone.Index][]*CachedMetadata
	cachedChildren                        map[milestone.Index][]*CachedChild

	minMilestoneIndex milestone.Index
}

func (c *TangleCache) SetMinMilestoneIndex(msIndex milestone.Index) {
	c.Lock()
	defer c.Unlock()
	c.minMilestoneIndex = msIndex
}

func (c *TangleCache) AddUnreferencedMessage(msIndex milestone.Index, cachedMsg *CachedMessage, cachedChildren CachedChildren) {
	c.Lock()
	defer c.Unlock()

	defer cachedMsg.Release(true)
	defer cachedChildren.Release(true)

	if msIndex < c.minMilestoneIndex {
		return
	}

	msgID := cachedMsg.GetMessage().GetMessageID()
	if _, exists := c.unreferencedMessages[msgID.ToMapKey()]; exists {
		// message already exists in the cache
		return
	}

	if _, exists := c.unreferencedMessagesPerMilestoneIndex[msIndex]; !exists {
		c.unreferencedMessagesPerMilestoneIndex[msIndex] = hornet.MessageIDs{}
	}

	unreferenced := &TangleCacheUnreferencedMessage{msg: cachedMsg.Retain(), children: cachedChildren.Retain()}
	c.unreferencedMessages[msgID.ToMapKey()] = unreferenced
	c.unreferencedMessagesPerMilestoneIndex[msIndex] = append(c.unreferencedMessagesPerMilestoneIndex[msIndex], msgID)
}

func (c *TangleCache) AddCachedMessage(msIndex milestone.Index, cachedMsg *CachedMessage) {
	c.Lock()
	defer c.Unlock()

	defer cachedMsg.Release(true)

	if msIndex < c.minMilestoneIndex {
		return
	}

	if _, exists := c.cachedMsgs[msIndex]; !exists {
		c.cachedMsgs[msIndex] = []*CachedMessage{}
	}
	c.cachedMsgs[msIndex] = append(c.cachedMsgs[msIndex], cachedMsg.Retain())
}

func (c *TangleCache) AddCachedMetadata(msIndex milestone.Index, cachedMsgMeta *CachedMetadata) {
	c.Lock()
	defer c.Unlock()

	defer cachedMsgMeta.Release(true)

	msgIDMapKey := cachedMsgMeta.GetMetadata().GetMessageID().ToMapKey()
	if unreferencedMsg, exists := c.unreferencedMessages[msgIDMapKey]; exists {
		// remove the message from the unreferenced messages map
		unreferencedMsg.Release(true)
		delete(c.unreferencedMessages, msgIDMapKey)
	}

	if msIndex < c.minMilestoneIndex {
		return
	}

	if _, exists := c.cachedMsgMetas[msIndex]; !exists {
		c.cachedMsgMetas[msIndex] = []*CachedMetadata{}
	}
	c.cachedMsgMetas[msIndex] = append(c.cachedMsgMetas[msIndex], cachedMsgMeta.Retain())
}

func (c *TangleCache) AddCachedChildren(msIndex milestone.Index, cachedChildren CachedChildren) {
	c.Lock()
	defer c.Unlock()

	defer cachedChildren.Release(true)

	if msIndex < c.minMilestoneIndex {
		return
	}

	if _, exists := c.cachedChildren[msIndex]; !exists {
		c.cachedChildren[msIndex] = []*CachedChild{}
	}
	c.cachedChildren[msIndex] = append(c.cachedChildren[msIndex], cachedChildren.Retain()...)
}

func (c *TangleCache) ReleaseUnreferencedMessages(msIndex milestone.Index, forceRelease ...bool) {
	c.Lock()
	defer c.Unlock()

	for index, unreferencedMessageIDs := range c.unreferencedMessagesPerMilestoneIndex {
		if index > msIndex {
			// only release entries that belong to older milestones
			continue
		}

		for _, msgID := range unreferencedMessageIDs {
			msgIDMapKey := msgID.ToMapKey()
			unreferencedMsg, exists := c.unreferencedMessages[msgIDMapKey]
			if !exists {
				// message does not exists in the cache anymore
				return
			}

			unreferencedMsg.Release(forceRelease...)
			delete(c.unreferencedMessages, msgIDMapKey)
		}
		delete(c.unreferencedMessagesPerMilestoneIndex, index)
	}
}
func (c *TangleCache) ReleaseCachedMessages(msIndex milestone.Index, forceRelease ...bool) {
	c.Lock()
	defer c.Unlock()

	for index, cachedMessages := range c.cachedMsgs {
		if index > msIndex {
			// only release entries that belong to older milestones
			continue
		}

		for _, cachedMsg := range cachedMessages {
			cachedMsg.Release(forceRelease...)
		}
		delete(c.cachedMsgs, index)
	}
}

func (c *TangleCache) ReleaseCachedMetadata(msIndex milestone.Index, forceRelease ...bool) {
	c.Lock()
	defer c.Unlock()

	for index, cachedMsgMetas := range c.cachedMsgMetas {
		if index > msIndex {
			// only release entries that belong to older milestones
			continue
		}

		for _, cachedMsgMeta := range cachedMsgMetas {
			cachedMsgMeta.Release(forceRelease...)
		}
		delete(c.cachedMsgMetas, index)
	}
}

func (c *TangleCache) ReleaseCachedChildren(msIndex milestone.Index, forceRelease ...bool) {
	c.Lock()
	defer c.Unlock()

	for index, cachedChildren := range c.cachedChildren {
		if index > msIndex {
			// only release entries that belong to older milestones
			continue
		}

		for _, cachedChild := range cachedChildren {
			cachedChild.Release(forceRelease...)
		}
		delete(c.cachedChildren, index)
	}
}

// Cleanup releases all the cached objects that have been used.
// This MUST be called by the user at the end or if no new milestones come in.
func (c *TangleCache) Cleanup() {
	c.Lock()
	defer c.Unlock()

	// release all unreferenced messages
	for _, unreferencedMsg := range c.unreferencedMessages {
		unreferencedMsg.Release(true)
	}
	c.unreferencedMessages = make(map[string]*TangleCacheUnreferencedMessage)
	c.unreferencedMessagesPerMilestoneIndex = make(map[milestone.Index]hornet.MessageIDs)

	// release all messages
	for _, cachedMessages := range c.cachedMsgs {
		for _, cachedMsg := range cachedMessages {
			cachedMsg.Release(true)
		}
	}
	c.cachedMsgs = make(map[milestone.Index][]*CachedMessage)

	// release all msg metadata
	for _, cachedMsgMetas := range c.cachedMsgMetas {
		for _, cachedMsgMeta := range cachedMsgMetas {
			cachedMsgMeta.Release(true)
		}
	}
	c.cachedMsgMetas = make(map[milestone.Index][]*CachedMetadata)

	// release all children
	for _, cachedChildren := range c.cachedChildren {
		for _, cachedChild := range cachedChildren {
			cachedChild.Release(true)
		}
	}
	c.cachedChildren = make(map[milestone.Index][]*CachedChild)
}

// FreeMemory copies the content of the internal maps to newly created maps.
// This is neccessary, otherwise the GC is not able to free the memory used by the old maps.
// "delete" doesn't shrink the maximum memory used by the map, since it only marks the entry as deleted.
func (c *TangleCache) FreeMemory() {
	c.Lock()
	defer c.Unlock()

	unreferencedMessagesMap := make(map[string]*TangleCacheUnreferencedMessage)
	for msgIDMapKey, unreferencedMsg := range c.unreferencedMessages {
		unreferencedMessagesMap[msgIDMapKey] = unreferencedMsg
	}
	c.unreferencedMessages = unreferencedMessagesMap

	unreferencedMessagesPerMilestoneIndexMap := make(map[milestone.Index]hornet.MessageIDs)
	for index, unreferencedMessageIDs := range c.unreferencedMessagesPerMilestoneIndex {
		unreferencedMessagesPerMilestoneIndexMap[index] = unreferencedMessageIDs
	}
	c.unreferencedMessagesPerMilestoneIndex = unreferencedMessagesPerMilestoneIndexMap

	cachedMsgsMap := make(map[milestone.Index][]*CachedMessage)
	for index, cachedMessages := range c.cachedMsgs {
		cachedMsgsMap[index] = cachedMessages
	}
	c.cachedMsgs = cachedMsgsMap

	cachedMsgMetasMap := make(map[milestone.Index][]*CachedMetadata)
	for index, cachedMsgMeta := range c.cachedMsgMetas {
		cachedMsgMetasMap[index] = cachedMsgMeta
	}
	c.cachedMsgMetas = cachedMsgMetasMap

	cachedChildrenMap := make(map[milestone.Index][]*CachedChild)
	for index, cachedChildren := range c.cachedChildren {
		cachedChildrenMap[index] = cachedChildren
	}
	c.cachedChildren = cachedChildrenMap
}
