package storage

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
)

type CachedMessageMetadataFunc func(blockID hornet.BlockID) (*CachedMetadata, error)

type MetadataMemcache struct {
	cachedMessageMetadataFunc CachedMessageMetadataFunc
	cachedBlockMetas          map[string]*CachedMetadata
}

// NewMetadataMemcache creates a new NewMetadataMemcache instance.
func NewMetadataMemcache(cachedMessageMetadataFunc CachedMessageMetadataFunc) *MetadataMemcache {
	return &MetadataMemcache{
		cachedMessageMetadataFunc: cachedMessageMetadataFunc,
		cachedBlockMetas:          make(map[string]*CachedMetadata),
	}
}

// Cleanup releases all the cached objects that have been used.
// This MUST be called by the user at the end.
func (c *MetadataMemcache) Cleanup(forceRelease bool) {

	// release all msg metadata at the end
	for _, cachedBlockMeta := range c.cachedBlockMetas {
		cachedBlockMeta.Release(forceRelease) // meta -1
	}
	c.cachedBlockMetas = make(map[string]*CachedMetadata)
}

// CachedBlockMetadata returns a cached metadata object.
// meta +1
func (c *MetadataMemcache) CachedBlockMetadata(blockID hornet.BlockID) (*CachedMetadata, error) {
	blockIDMapKey := blockID.ToMapKey()

	var err error

	// load up msg metadata
	cachedBlockMeta, exists := c.cachedBlockMetas[blockIDMapKey]
	if !exists {
		cachedBlockMeta, err = c.cachedMessageMetadataFunc(blockID) // meta +1 (this is the one that gets cleared by "Cleanup")
		if err != nil {
			return nil, err
		}
		if cachedBlockMeta == nil {
			return nil, nil
		}

		// add the cachedObject to the map, it will be released by calling "Cleanup" at the end
		c.cachedBlockMetas[blockIDMapKey] = cachedBlockMeta
	}

	return cachedBlockMeta.Retain(), nil // meta +1
}
