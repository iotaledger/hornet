package storage

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
)

type CachedMessageFunc func(blockID hornet.BlockID) (*CachedMessage, error)

type MessagesMemcache struct {
	cachedMessageFunc CachedMessageFunc
	cachedBlocks      map[string]*CachedMessage
}

// NewMessagesMemcache creates a new MessagesMemcache instance.
func NewMessagesMemcache(cachedMessageFunc CachedMessageFunc) *MessagesMemcache {
	return &MessagesMemcache{
		cachedMessageFunc: cachedMessageFunc,
		cachedBlocks:      make(map[string]*CachedMessage),
	}
}

// Cleanup releases all the cached objects that have been used.
// This MUST be called by the user at the end.
func (c *MessagesMemcache) Cleanup(forceRelease bool) {

	// release all msgs at the end
	for _, cachedBlock := range c.cachedBlocks {
		cachedBlock.Release(forceRelease) // message -1
	}
	c.cachedBlocks = make(map[string]*CachedMessage)
}

// CachedMessage returns a cached message object.
// message +1
func (c *MessagesMemcache) CachedMessage(blockID hornet.BlockID) (*CachedMessage, error) {
	blockIDMapKey := blockID.ToMapKey()

	var err error

	// load up msg
	cachedBlock, exists := c.cachedBlocks[blockIDMapKey]
	if !exists {
		cachedBlock, err = c.cachedMessageFunc(blockID) // message +1 (this is the one that gets cleared by "Cleanup")
		if err != nil {
			return nil, err
		}
		if cachedBlock == nil {
			return nil, nil
		}

		// add the cachedObject to the map, it will be released by calling "Cleanup" at the end
		c.cachedBlocks[blockIDMapKey] = cachedBlock
	}

	return cachedBlock.Retain(), nil // message +1
}
