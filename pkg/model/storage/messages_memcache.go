package storage

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
)

type CachedBlockFunc func(blockID hornet.BlockID) (*CachedBlock, error)

type BlocksMemcache struct {
	cachedBlockFunc CachedBlockFunc
	cachedBlocks    map[string]*CachedBlock
}

// NewBlocksMemcache creates a new BlocksMemcache instance.
func NewBlocksMemcache(cachedBlockFunc CachedBlockFunc) *BlocksMemcache {
	return &BlocksMemcache{
		cachedBlockFunc: cachedBlockFunc,
		cachedBlocks:    make(map[string]*CachedBlock),
	}
}

// Cleanup releases all the cached objects that have been used.
// This MUST be called by the user at the end.
func (c *BlocksMemcache) Cleanup(forceRelease bool) {

	// release all msgs at the end
	for _, cachedBlock := range c.cachedBlocks {
		cachedBlock.Release(forceRelease) // block -1
	}
	c.cachedBlocks = make(map[string]*CachedBlock)
}

// CachedBlock returns a cached block object.
// block +1
func (c *BlocksMemcache) CachedBlock(blockID hornet.BlockID) (*CachedBlock, error) {
	blockIDMapKey := blockID.ToMapKey()

	var err error

	// load up block
	cachedBlock, exists := c.cachedBlocks[blockIDMapKey]
	if !exists {
		cachedBlock, err = c.cachedBlockFunc(blockID) // block +1 (this is the one that gets cleared by "Cleanup")
		if err != nil {
			return nil, err
		}
		if cachedBlock == nil {
			return nil, nil
		}

		// add the cachedObject to the map, it will be released by calling "Cleanup" at the end
		c.cachedBlocks[blockIDMapKey] = cachedBlock
	}

	return cachedBlock.Retain(), nil // block +1
}
