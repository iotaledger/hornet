package storage

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
)

type CachedMessageMetadataFunc func(messageID hornet.MessageID) (*CachedMetadata, error)

type MetadataMemcache struct {
	cachedMessageMetadataFunc CachedMessageMetadataFunc
	cachedMsgMetas            map[string]*CachedMetadata
}

// NewMetadataMemcache creates a new NewMetadataMemcache instance.
func NewMetadataMemcache(cachedMessageMetadataFunc CachedMessageMetadataFunc) *MetadataMemcache {
	return &MetadataMemcache{
		cachedMessageMetadataFunc: cachedMessageMetadataFunc,
		cachedMsgMetas:            make(map[string]*CachedMetadata),
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

// CachedMessageMetadata returns a cached metadata object.
// metadata +1
func (c *MetadataMemcache) CachedMessageMetadata(messageID hornet.MessageID) (*CachedMetadata, error) {
	messageIDMapKey := messageID.ToMapKey()

	var err error

	// load up msg metadata
	cachedMsgMeta, exists := c.cachedMsgMetas[messageIDMapKey]
	if !exists {
		cachedMsgMeta, err = c.cachedMessageMetadataFunc(messageID) // meta +1 (this is the one that gets cleared by "Cleanup")
		if err != nil {
			return nil, err
		}
		if cachedMsgMeta == nil {
			return nil, nil
		}

		// add the cachedObject to the map, it will be released by calling "Cleanup" at the end
		c.cachedMsgMetas[messageIDMapKey] = cachedMsgMeta
	}

	return cachedMsgMeta.Retain(), nil // meta +1
}
