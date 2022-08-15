package storage

import iotago "github.com/iotaledger/iota.go/v3"

type CachedBlockMetadataFunc func(blockID iotago.BlockID) (*CachedMetadata, error)

type MetadataMemcache struct {
	cachedBlockMetadataFunc CachedBlockMetadataFunc
	cachedBlockMetas        map[iotago.BlockID]*CachedMetadata
}

// NewMetadataMemcache creates a new NewMetadataMemcache instance.
func NewMetadataMemcache(cachedBlockMetadataFunc CachedBlockMetadataFunc) *MetadataMemcache {
	return &MetadataMemcache{
		cachedBlockMetadataFunc: cachedBlockMetadataFunc,
		cachedBlockMetas:        make(map[iotago.BlockID]*CachedMetadata),
	}
}

// Cleanup releases all the cached objects that have been used.
// This MUST be called by the user at the end.
func (c *MetadataMemcache) Cleanup(forceRelease bool) {

	// release all block metadata at the end
	for _, cachedBlockMeta := range c.cachedBlockMetas {
		cachedBlockMeta.Release(forceRelease) // meta -1
	}
	c.cachedBlockMetas = make(map[iotago.BlockID]*CachedMetadata)
}

// CachedBlockMetadata returns a cached metadata object.
// meta +1.
func (c *MetadataMemcache) CachedBlockMetadata(blockID iotago.BlockID) (*CachedMetadata, error) {
	var err error

	// load up block metadata
	cachedBlockMeta, exists := c.cachedBlockMetas[blockID]
	if !exists {
		cachedBlockMeta, err = c.cachedBlockMetadataFunc(blockID) // meta +1 (this is the one that gets cleared by "Cleanup")
		if err != nil {
			return nil, err
		}
		if cachedBlockMeta == nil {
			//nolint:nilnil // nil, nil is ok in this context, even if it is not go idiomatic
			return nil, nil
		}

		// add the cachedObject to the map, it will be released by calling "Cleanup" at the end
		c.cachedBlockMetas[blockID] = cachedBlockMeta
	}

	return cachedBlockMeta.Retain(), nil // meta +1
}
