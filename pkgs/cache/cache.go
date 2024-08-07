package cache

import (
	"sync"
	"time"
)

// Cache stores arbitrary data with expiration time and cost.
type Cache[K comparable, V any] struct {
	items        map[K]*element[K, *item[K, V]]
	evictionList *list[K, *item[K, V]]
	mutex        sync.RWMutex
	close        chan struct{}
	maxCost      int64
	currentCost  int64
}

// An item represents arbitrary data with expiration time and cost.
type item[K comparable, V any] struct {
	key     K
	data    V
	expires int64
	cost    int64
}

// New creates a new cache that asynchronously cleans expired entries after the given time passes.
func New[K comparable, V any](cleaningInterval time.Duration, maxCost int64) *Cache[K, V] {
	cache := &Cache[K, V]{
		items:        make(map[K]*element[K, *item[K, V]]),
		evictionList: newList[K, *item[K, V]](),
		close:        make(chan struct{}),
		maxCost:      maxCost,
	}

	go func() {
		ticker := time.NewTicker(cleaningInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cache.evictExpiredItems()
			case <-cache.close:
				return
			}
		}
	}()

	return cache
}

// Get gets the value for the given key.
func (cache *Cache[K, V]) Get(key K) (V, bool) {
	if cache == nil {
		var zeroValue V
		return zeroValue, false
	}

	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	element, exists := cache.items[key]
	if !exists {
		var zeroValue V
		return zeroValue, false
	}

	item := element.value
	if item.expires > 0 && time.Now().UnixNano() > item.expires {
		cache.mutex.RUnlock()
		cache.mutex.Lock()
		cache.removeElement(element)
		cache.mutex.Unlock()
		cache.mutex.RLock()
		var zeroValue V
		return zeroValue, false
	}

	cache.evictionList.moveToFront(element)

	return item.data, true
}

// Set sets a value for the given key with an expiration duration and a cost.
// If the duration is 0 or less, it will be stored forever.
func (cache *Cache[K, V]) Set(key K, value V, duration time.Duration, cost int64) {
	if cache == nil {
		return
	}

	var expires int64
	if duration > 0 {
		expires = time.Now().Add(duration).UnixNano()
	}

	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if element, exists := cache.items[key]; exists {
		cache.evictionList.moveToFront(element)
		item := element.value
		cache.currentCost -= item.cost
		item.data = value
		item.expires = expires
		item.cost = cost
		cache.currentCost += cost
	} else {
		newItem := &item[K, V]{key: key, data: value, expires: expires, cost: cost}
		element := cache.evictionList.pushFront(newItem)
		cache.items[key] = element
		cache.currentCost += cost
	}

	for cache.maxCost > 0 && cache.currentCost > cache.maxCost {
		element := cache.evictionList.back()
		if element != nil {
			cache.removeElement(element)
		}
	}
}

// Delete deletes the key and its value from the cache.
func (cache *Cache[K, V]) Delete(key K) {
	if cache == nil {
		return
	}

	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if element, exists := cache.items[key]; exists {
		cache.removeElement(element)
	}
}

// Clear removes all items from the cache.
func (cache *Cache[K, V]) Clear() {
	if cache == nil {
		return
	}

	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	cache.items = make(map[K]*element[K, *item[K, V]])
	cache.evictionList = newList[K, *item[K, V]]()
	cache.currentCost = 0
}

// Close closes the cache and frees up resources.
func (cache *Cache[K, V]) Close() {
	if cache == nil {
		return
	}

	close(cache.close)
	cache.Clear()
}

// evictExpiredItems removes all expired items from the cache.
func (cache *Cache[K, V]) evictExpiredItems() {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	now := time.Now().UnixNano()
	for element := cache.evictionList.back(); element != nil; {
		prev := element.prev
		item := element.value
		if item.expires > 0 && now > item.expires {
			cache.removeElement(element)
		}
		element = prev
	}
}

// removeElement removes the specified list element from the cache.
func (cache *Cache[K, V]) removeElement(element *element[K, *item[K, V]]) {
	cache.evictionList.remove(element)
	item := element.value
	delete(cache.items, item.key)
	cache.currentCost -= item.cost
}
