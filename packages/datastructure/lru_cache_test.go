package datastructure

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/datastructure"
	"github.com/gohornet/hornet/packages/typeutils"
)

func TestLRUCache(t *testing.T) {
	cache := NewLRUCache(5)

	cache.ComputeIfAbsent("test", func() interface{} {
		return 12
	})

	if cache.Get("test") != 12 {
		t.Error("the cache does not contain the added elements")
	}

	if cache.GetSize() != 1 {
		t.Error("the size should be 1")
	}

	if cache.GetCapacity() != 5 {
		t.Error("the capacity should be 5")
	}

	cache.Set("a", 3)
	cache.Set("b", 4)
	cache.Set("c", 5)
	cache.Set("d", 6)

	if cache.GetSize() != 5 {
		t.Error("the size should be 5")
	}

	cache.Set("e", 7)

	if cache.GetSize() != 5 {
		t.Error("the size should be 5")
	}

	if cache.Get("test") != nil {
		t.Error("'test' should have been dropped")
	}

	cache.Set("a", 6)
	cache.Set("f", 8)

	if cache.GetSize() != 5 {
		t.Error("the size should be 5")
	}

	if cache.Get("a") == nil {
		t.Error("'a' should not have been dropped")
	}
	if cache.Get("b") != nil {
		t.Error("'b' should have been dropped")
	}

	{
		key, value := "test2", 1337

		cache.ComputeIfAbsent(key, func() interface{} {
			return value
		})
		if cache.Get(key) != value {
			t.Error("'" + key + "' should have been added")
		}
	}

	if cache.GetSize() != 5 {
		t.Error("the size should be 5")
	}

	if cache.Get("a") != nil {
		cache.Delete("a")
	}
	if cache.GetSize() != 4 {
		t.Error("the size should be 4")
	}

	cache.Delete("f")
	if cache.GetSize() != 3 {
		t.Error("the size should be 3")
	}
}

func TestLRUCache_ComputeIfPresent(t *testing.T) {
	cache := NewLRUCache(5)
	cache.Set(8, 9)

	cache.ComputeIfPresent(8, func(value interface{}) interface{} {
		return 88
	})
	if cache.Get(8) != 88 || cache.GetSize() != 1 {
		t.Error("cache was not updated correctly")
	}

	cache.ComputeIfPresent(8, func(value interface{}) interface{} {
		return nil
	})
	if cache.Get(8) != nil || cache.GetSize() != 0 {
		t.Error("cache was not updated correctly")
	}
}

func TestBatchedEvictLRUCache(t *testing.T) {
	var called bool
	cb := func(keys interface{}, values interface{}) {
		keysT := keys.([]interface{})
		if len(keysT) != 5 {
			t.Fatalf("expected 5 elements to be evicted but got %d", len(keysT))
		}
		// we evicted the first 5 elements we added to the LRU cache
		for i := 0; i < 5; i++ {
			key := keysT[i].(int)
			if key != i {
				t.Fatalf("expected element with key %d to be evicted at pos %d", i, key)
			}
		}
		called = true
	}

	cache := NewLRUCache(10, &LRUCacheOptions{
		EvictionCallback:  cb,
		EvictionBatchSize: 5,
		IdleTimeout:       0,
	})

	for i := 0; i < 11; i++ {
		cache.set(i, i)
	}

	if !called {
		t.Fatalf("expected the batch eviction callback to be called")
	}

	// we added a new element and removed the last 5
	if cache.GetSize() != 6 {
		t.Fatalf("expected cache size to be 6 but got %d", cache.GetSize())
	}
}

func TestLRUCache_Eviction(t *testing.T) {

	cache := datastructure.NewLRUCache(100, &datastructure.LRUCacheOptions{
		EvictionCallback: func(key interface{}, value interface{}) {
			evictedKey := key.(string)
			evictedObj := value.(int)
			println(fmt.Sprintf("Evicted Key: %s, Value: %d", evictedKey, evictedObj))
		},
	})

	for i := 0; i < 110; i++ {
		cache.Set(fmt.Sprintf("%d", i), i)
	}

	for i := 0; i < 110; i++ {
		println(fmt.Sprintf("Contains: %d, %v", i, cache.Contains(fmt.Sprintf("%d", i))))
	}

	for i := 0; i < 110; i++ {
		var result int
		if cacheResult := cache.ComputeIfAbsent(fmt.Sprintf("%d", i), func() interface{} {
			return i
		}); !typeutils.IsInterfaceNil(cacheResult) {
			result = cacheResult.(int)
		}

		println(fmt.Sprintf("Key: %d, Value: %d", i, result))
	}
}
