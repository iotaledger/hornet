package datastructure

import (
	"time"
)

type LRUCacheOptions struct {
	EvictionCallback  func(keyOrBatchedKeys interface{}, valueOrBatchedValues interface{})
	EvictionBatchSize uint64
	IdleTimeout       time.Duration
}

var DEFAULT_OPTIONS = &LRUCacheOptions{
	EvictionCallback:  nil,
	EvictionBatchSize: 1,
	IdleTimeout:       30 * time.Second,
}
