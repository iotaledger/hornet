package storage

import iotago "github.com/iotaledger/iota.go/v3"

type CachedBlockFunc func(blockID iotago.BlockID) (*CachedBlock, error)

type BlocksMemcache struct {
	cachedBlockFunc CachedBlockFunc
	cachedBlocks    map[iotago.BlockID]*CachedBlock
}

// NewBlocksMemcache creates a new BlocksMemcache instance.
func NewBlocksMemcache(cachedBlockFunc CachedBlockFunc) *BlocksMemcache {
	return &BlocksMemcache{
		cachedBlockFunc: cachedBlockFunc,
		cachedBlocks:    make(map[iotago.BlockID]*CachedBlock),
	}
}

// Cleanup releases all the cached objects that have been used.
// This MUST be called by the user at the end.
func (c *BlocksMemcache) Cleanup(forceRelease bool) {

	// release all blocks at the end
	for _, cachedBlock := range c.cachedBlocks {
		cachedBlock.Release(forceRelease) // block -1
	}
	c.cachedBlocks = make(map[iotago.BlockID]*CachedBlock)
}

// CachedBlock returns a cached block object.
// block +1.
func (c *BlocksMemcache) CachedBlock(blockID iotago.BlockID) (*CachedBlock, error) {
	var err error

	// load up block
	cachedBlock, exists := c.cachedBlocks[blockID]
	if !exists {
		cachedBlock, err = c.cachedBlockFunc(blockID) // block +1 (this is the one that gets cleared by "Cleanup")
		if err != nil {
			return nil, err
		}
		if cachedBlock == nil {
			//nolint:nilnil // nil, nil is ok in this context, even if it is not go idiomatic
			return nil, nil
		}

		// add the cachedObject to the map, it will be released by calling "Cleanup" at the end
		c.cachedBlocks[blockID] = cachedBlock
	}

	return cachedBlock.Retain(), nil // block +1
}
