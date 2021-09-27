package storage

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
)

type MetadataMemcache struct {
	storage        *Storage
	cachedMsgMetas map[string]*CachedMetadata
}

// NewMetadataMemcache creates a new NewMetadataMemcache instance.
func NewMetadataMemcache(dbStorage *Storage) *MetadataMemcache {
	return &MetadataMemcache{
		storage:        dbStorage,
		cachedMsgMetas: make(map[string]*CachedMetadata),
	}
}

// Cleanup releases all the cached objects that have been used.
// This MUST be called by the user at the end.
func (c *MetadataMemcache) Cleanup(forceRelease bool) {

	// release all msg metadata at the end
	for _, cachedMsgMeta := range c.cachedMsgMetas {
		cachedMsgMeta.Release(forceRelease) // meta -1
	}
	c.cachedMsgMetas = make(map[string]*CachedMetadata)
}

// CachedMetadataOrNil returns a cached metadata object.
// metadata +1
func (c *MetadataMemcache) CachedMetadataOrNil(messageID hornet.MessageID) *CachedMetadata {
	messageIDMapKey := messageID.ToMapKey()

	// load up msg metadata
	cachedMsgMeta, exists := c.cachedMsgMetas[messageIDMapKey]
	if !exists {
		cachedMsgMeta = c.storage.CachedMessageMetadataOrNil(messageID) // meta +1
		if cachedMsgMeta == nil {
			return nil
		}

		// add the cachedObject to the map, it will be released by calling "Cleanup" at the end
		c.cachedMsgMetas[messageIDMapKey] = cachedMsgMeta
	}

	return cachedMsgMeta
}
