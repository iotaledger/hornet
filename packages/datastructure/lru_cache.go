package datastructure

import (
	"github.com/gohornet/hornet/packages/syncutils"
	"github.com/gohornet/hornet/packages/typeutils"
)

type lruCacheElement struct {
	key   interface{}
	value interface{}
}

type LRUCache struct {
	directory        map[interface{}]*DoublyLinkedListEntry
	doublyLinkedList *DoublyLinkedList
	capacity         int
	size             int
	options          *LRUCacheOptions
	mutex            syncutils.RWMutex
	krwMutex         KRWMutex
}

func NewLRUCache(capacity int, options ...*LRUCacheOptions) *LRUCache {
	var currentOptions *LRUCacheOptions
	if len(options) < 1 || options[0] == nil {
		currentOptions = DEFAULT_OPTIONS
	} else {
		currentOptions = options[0]
	}

	if currentOptions.EvictionBatchSize > 1 {
		if int(currentOptions.EvictionBatchSize) > capacity {
			panic("eviction batch size must be equal or lower than the capacity")
		}
	} else if currentOptions.EvictionBatchSize == 0 {
		currentOptions.EvictionBatchSize = 1
	}

	return &LRUCache{
		directory:        make(map[interface{}]*DoublyLinkedListEntry, capacity),
		doublyLinkedList: &DoublyLinkedList{},
		capacity:         capacity,
		options:          currentOptions,
		krwMutex:         KRWMutex{keyMutexConsumers: make(map[interface{}]int), keyMutexes: make(map[interface{}]*syncutils.RWMutex)},
	}
}

func (cache *LRUCache) Set(key interface{}, value interface{}) {
	keyMutex := cache.krwMutex.Register(key)
	keyMutex.Lock()

	cache.mutex.Lock()
	cache.set(key, value)
	cache.mutex.Unlock()

	keyMutex.Unlock()
	cache.krwMutex.Free(key)
}

func (cache *LRUCache) set(key interface{}, value interface{}) {
	directory := cache.directory

	if element, exists := directory[key]; exists {
		element.value.(*lruCacheElement).value = value
		cache.promoteElement(element)
		return
	}

	linkedListEntry := &DoublyLinkedListEntry{value: &lruCacheElement{key: key, value: value}}
	cache.doublyLinkedList.addFirstEntry(linkedListEntry)
	directory[key] = linkedListEntry
	cache.size++

	if cache.size <= cache.capacity {
		return
	}

	// gather the elements we want to evict
	elemsToEvict := []interface{}{}
	elemsKeys := []interface{}{}
	for i := 0; i < int(cache.options.EvictionBatchSize) && cache.doublyLinkedList.GetSize() > 0; i++ {
		element, err := cache.doublyLinkedList.removeLastEntry()
		if err != nil {
			panic(err)
		}
		lruCacheElement := element.value.(*lruCacheElement)

		// we don't need to hold any element locks because we are holding
		// the entire cache's write lock
		elemsToEvict = append(elemsToEvict, lruCacheElement.value)
		elemsKeys = append(elemsKeys, lruCacheElement.key)
	}

	if cache.options.EvictionCallback != nil {
		if cache.options.EvictionBatchSize == 1 && len(elemsKeys) == 1 {
			cache.options.EvictionCallback(elemsKeys[0], elemsToEvict[0])
		} else {
			cache.options.EvictionCallback(elemsKeys, elemsToEvict)
		}
	}
	// remove the elements from the cache
	for i := range elemsKeys {
		delete(directory, elemsKeys[i])
	}
	cache.size -= len(elemsToEvict)
}

func (cache *LRUCache) ComputeIfAbsent(key interface{}, callback func() interface{}) (result interface{}) {
	keyMutex := cache.krwMutex.Register(key)

	keyMutex.Lock()
	cache.mutex.RLock()
	if element, exists := cache.directory[key]; exists {
		cache.mutex.RUnlock()
		cache.mutex.Lock()
		cache.promoteElement(element)
		cache.mutex.Unlock()

		result = element.GetValue().(*lruCacheElement).value

		keyMutex.Unlock()
	} else {
		cache.mutex.RUnlock()
		if result = callback(); !typeutils.IsInterfaceNil(result) {
			cache.mutex.Lock()
			cache.set(key, result)
			cache.mutex.Unlock()
		}
		keyMutex.Unlock()
	}

	cache.krwMutex.Free(key)

	return
}

// Calls the callback if an entry with the given key exists.
// The result of the callback is written back into the cache.
// If the callback returns nil the entry is removed from the cache.
// Returns the updated entry.
func (cache *LRUCache) ComputeIfPresent(key interface{}, callback func(value interface{}) interface{}) (result interface{}) {
	keyMutex := cache.krwMutex.Register(key)

	keyMutex.RLock()
	cache.mutex.RLock()
	if entry, exists := cache.directory[key]; exists {
		cache.mutex.RUnlock()
		keyMutex.RUnlock()
		keyMutex.Lock()

		result = entry.GetValue().(*lruCacheElement).value

		if callbackResult := callback(result); !typeutils.IsInterfaceNil(callbackResult) {
			result = callbackResult

			cache.mutex.Lock()
			cache.set(key, callbackResult)
			cache.mutex.Unlock()

			keyMutex.Unlock()
		} else {
			cache.mutex.Lock()
			if err := cache.doublyLinkedList.removeEntry(entry); err != nil {
				panic(err)
			}
			delete(cache.directory, key)
			cache.size--
			cache.mutex.Unlock()

			keyMutex.Unlock()

			if cache.options.EvictionCallback != nil {
				cache.options.EvictionCallback(key, result)
			}
		}
	} else {
		cache.mutex.RUnlock()
		keyMutex.RUnlock()
	}

	cache.krwMutex.Free(key)

	return
}

func (cache *LRUCache) Contains(key interface{}) (result bool) {
	keyMutex := cache.krwMutex.Register(key)

	keyMutex.RLock()
	cache.mutex.RLock()
	if element, exists := cache.directory[key]; exists {
		cache.mutex.RUnlock()
		keyMutex.RUnlock()

		cache.mutex.Lock()
		cache.promoteElement(element)
		cache.mutex.Unlock()

		result = true
	} else {
		cache.mutex.RUnlock()
		keyMutex.RUnlock()

		result = false
	}

	cache.krwMutex.Free(key)

	return
}

func (cache *LRUCache) Get(key interface{}) (result interface{}) {
	keyMutex := cache.krwMutex.Register(key)

	keyMutex.RLock()
	cache.mutex.RLock()
	if element, exists := cache.directory[key]; exists {
		cache.mutex.RUnlock()
		cache.mutex.Lock()
		cache.promoteElement(element)
		cache.mutex.Unlock()

		result = element.GetValue().(*lruCacheElement).value

	} else {
		cache.mutex.RUnlock()
	}

	keyMutex.RUnlock()
	cache.krwMutex.Free(key)

	return
}

func (cache *LRUCache) GetCapacity() int {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	return cache.capacity
}

func (cache *LRUCache) GetSize() int {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	return cache.size
}

func (cache *LRUCache) DeleteWithoutEviction(key interface{}) (existed bool, keyToEvict interface{}, valueToEvict interface{}) {
	keyMutex := cache.krwMutex.Register(key)
	keyMutex.Lock()

	cache.mutex.RLock()

	entry, exists := cache.directory[key]
	if exists {
		cache.mutex.RUnlock()
		cache.mutex.Lock()

		if err := cache.doublyLinkedList.removeEntry(entry); err != nil {
			panic(err)
		}
		delete(cache.directory, key)

		cache.size--
		cache.mutex.Unlock()
		keyMutex.Unlock()

		cache.krwMutex.Free(key)

		return true, key, entry.GetValue().(*lruCacheElement).value
	}

	cache.mutex.RUnlock()

	keyMutex.Unlock()
	cache.krwMutex.Free(key)

	return false, nil, nil
}

func (cache *LRUCache) Delete(key interface{}) bool {
	deleted, keyToEvict, valueToEvict := cache.DeleteWithoutEviction(key)

	if deleted {
		if cache.options.EvictionCallback != nil {
			cache.options.EvictionCallback(keyToEvict, valueToEvict)
		}
	}

	return deleted
}

func (cache *LRUCache) promoteElement(element *DoublyLinkedListEntry) {
	if err := cache.doublyLinkedList.removeEntry(element); err != nil {
		panic(err)
	}
	cache.doublyLinkedList.addFirstEntry(element)
}

func (cache *LRUCache) DeleteAll() {
	cache.mutex.Lock()

	// gather the elements we want to evict
	elemsToEvict := []interface{}{}
	elemsKeys := []interface{}{}
	evictedItemsCnt := 0

	for key, entry := range cache.directory {
		if err := cache.doublyLinkedList.removeEntry(entry); err != nil {
			panic(err)
		}
		delete(cache.directory, key)

		cache.size--

		// we don't need to hold any element locks because we are holding
		// the entire cache's write lock
		lruCacheElement := entry.GetValue().(*lruCacheElement)
		elemsToEvict = append(elemsToEvict, lruCacheElement.value)
		elemsKeys = append(elemsKeys, lruCacheElement.key)
		evictedItemsCnt++
	}

	if cache.options.EvictionCallback != nil && evictedItemsCnt > 0 {
		for i := 0; i < evictedItemsCnt; {
			startIndex := i
			endIndex := startIndex + int(cache.options.EvictionBatchSize)
			if endIndex > evictedItemsCnt {
				endIndex = evictedItemsCnt
			}

			if cache.options.EvictionBatchSize == 1 {
				cache.options.EvictionCallback(elemsKeys[startIndex], elemsToEvict[startIndex])
			} else {
				cache.options.EvictionCallback(elemsKeys[startIndex:endIndex], elemsToEvict[startIndex:endIndex])
			}

			i = endIndex
		}
	}

	cache.mutex.Unlock()
}
