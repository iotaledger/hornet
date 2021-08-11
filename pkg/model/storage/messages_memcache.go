package storage

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
)

// NewMessagesMemcache creates a new MessagesMemcache instance.
func NewMessagesMemcache(storage *Storage) *MessagesMemcache {
	return &MessagesMemcache{
		storage:    storage,
		cachedMsgs: make(map[string]*CachedMessage),
	}
}

type MessagesMemcache struct {
	storage    *Storage
	cachedMsgs map[string]*CachedMessage
}

// Cleanup releases all the cached objects that have been used.
// This MUST be called by the user at the end.
func (c *MessagesMemcache) Cleanup(forceRelease bool) {

	// release all msgs at the end
	for _, cachedMsg := range c.cachedMsgs {
		cachedMsg.Release(forceRelease) // meta -1
	}
	c.cachedMsgs = make(map[string]*CachedMessage)
}

// CachedMessageOrNil returns a cached message object.
// msg +1
func (c *MessagesMemcache) CachedMessageOrNil(messageID hornet.MessageID) *CachedMessage {
	messageIDMapKey := messageID.ToMapKey()

	// load up msg
	cachedMsg, exists := c.cachedMsgs[messageIDMapKey]
	if !exists {
		cachedMsg = c.storage.CachedMessageOrNil(messageID) // msg +1
		if cachedMsg == nil {
			return nil
		}

		// add the cachedObject to the map, it will be released by calling "Cleanup" at the end
		c.cachedMsgs[messageIDMapKey] = cachedMsg
	}

	return cachedMsg
}
