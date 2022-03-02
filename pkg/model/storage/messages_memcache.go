package storage

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
)

type CachedMessageFunc func(messageID hornet.MessageID) (*CachedMessage, error)

type MessagesMemcache struct {
	cachedMessageFunc CachedMessageFunc
	cachedMsgs        map[string]*CachedMessage
}

// NewMessagesMemcache creates a new MessagesMemcache instance.
func NewMessagesMemcache(cachedMessageFunc CachedMessageFunc) *MessagesMemcache {
	return &MessagesMemcache{
		cachedMessageFunc: cachedMessageFunc,
		cachedMsgs:        make(map[string]*CachedMessage),
	}
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

// CachedMessage returns a cached message object.
// msg +1
func (c *MessagesMemcache) CachedMessage(messageID hornet.MessageID) (*CachedMessage, error) {
	messageIDMapKey := messageID.ToMapKey()

	var err error

	// load up msg
	cachedMsg, exists := c.cachedMsgs[messageIDMapKey]
	if !exists {
		cachedMsg, err = c.cachedMessageFunc(messageID) // msg +1 (this is the one that gets cleared by "Cleanup")
		if err != nil {
			return nil, err
		}
		if cachedMsg == nil {
			return nil, nil
		}

		// add the cachedObject to the map, it will be released by calling "Cleanup" at the end
		c.cachedMsgs[messageIDMapKey] = cachedMsg
	}

	return cachedMsg.Retain(), nil // msg +1
}
