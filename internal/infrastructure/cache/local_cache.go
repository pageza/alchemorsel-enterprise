// Package cache provides local in-memory cache implementation
package cache

import (
	"sync"
	"time"
)

// LocalCache provides thread-safe in-memory caching with LRU eviction
type LocalCache struct {
	items    map[string]*localCacheItem
	lruList  *lruList
	maxSize  int
	mu       sync.RWMutex
}

// localCacheItem represents a cached item with TTL and LRU tracking
type localCacheItem struct {
	data      interface{}
	expiresAt time.Time
	lruNode   *lruNode
}

// lruList implements a doubly-linked list for LRU tracking
type lruList struct {
	head *lruNode
	tail *lruNode
	size int
}

// lruNode represents a node in the LRU list
type lruNode struct {
	key  string
	prev *lruNode
	next *lruNode
}

// NewLocalCache creates a new local cache with specified maximum size
func NewLocalCache(maxSize int) *LocalCache {
	if maxSize <= 0 {
		maxSize = 1000 // Default size
	}

	lru := &lruList{}
	lru.head = &lruNode{}
	lru.tail = &lruNode{}
	lru.head.next = lru.tail
	lru.tail.prev = lru.head

	return &LocalCache{
		items:   make(map[string]*localCacheItem),
		lruList: lru,
		maxSize: maxSize,
	}
}

// Get retrieves an item from the cache
func (lc *LocalCache) Get(key string) (interface{}, bool) {
	lc.mu.RLock()
	item, exists := lc.items[key]
	lc.mu.RUnlock()

	if !exists {
		return nil, false
	}

	// Check if item has expired
	if time.Now().After(item.expiresAt) {
		lc.mu.Lock()
		lc.deleteItem(key, item)
		lc.mu.Unlock()
		return nil, false
	}

	// Move to front of LRU list (mark as recently used)
	lc.mu.Lock()
	lc.moveToFront(item.lruNode)
	lc.mu.Unlock()

	return item.data, true
}

// Set stores an item in the cache with TTL
func (lc *LocalCache) Set(key string, data interface{}, ttl time.Duration) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	expiresAt := time.Now().Add(ttl)

	// If item already exists, update it
	if existingItem, exists := lc.items[key]; exists {
		existingItem.data = data
		existingItem.expiresAt = expiresAt
		lc.moveToFront(existingItem.lruNode)
		return
	}

	// Create new LRU node
	node := &lruNode{key: key}

	// Create new cache item
	item := &localCacheItem{
		data:      data,
		expiresAt: expiresAt,
		lruNode:   node,
	}

	// Add to cache
	lc.items[key] = item
	lc.addToFront(node)

	// Evict if necessary
	lc.evictIfNecessary()
}

// Delete removes an item from the cache
func (lc *LocalCache) Delete(key string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if item, exists := lc.items[key]; exists {
		lc.deleteItem(key, item)
	}
}

// Exists checks if a key exists in the cache (and is not expired)
func (lc *LocalCache) Exists(key string) bool {
	lc.mu.RLock()
	item, exists := lc.items[key]
	lc.mu.RUnlock()

	if !exists {
		return false
	}

	// Check if item has expired
	if time.Now().After(item.expiresAt) {
		lc.mu.Lock()
		lc.deleteItem(key, item)
		lc.mu.Unlock()
		return false
	}

	return true
}

// Size returns the current number of items in the cache
func (lc *LocalCache) Size() int {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return len(lc.items)
}

// Clear removes all items from the cache
func (lc *LocalCache) Clear() {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	lc.items = make(map[string]*localCacheItem)
	lc.lruList = &lruList{}
	lc.lruList.head = &lruNode{}
	lc.lruList.tail = &lruNode{}
	lc.lruList.head.next = lc.lruList.tail
	lc.lruList.tail.prev = lc.lruList.head
}

// InvalidatePattern removes all keys matching a pattern (simple prefix matching)
func (lc *LocalCache) InvalidatePattern(pattern string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// For simplicity, we'll support prefix matching
	// In production, you might want to use a more sophisticated pattern matching
	keysToDelete := make([]string, 0)

	for key := range lc.items {
		if matchesPattern(key, pattern) {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		if item, exists := lc.items[key]; exists {
			lc.deleteItem(key, item)
		}
	}
}

// CleanupExpired removes all expired items from the cache
func (lc *LocalCache) CleanupExpired() int {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	for key, item := range lc.items {
		if now.After(item.expiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		if item, exists := lc.items[key]; exists {
			lc.deleteItem(key, item)
		}
	}

	return len(expiredKeys)
}

// GetStats returns cache statistics
func (lc *LocalCache) GetStats() LocalCacheStats {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	now := time.Now()
	expired := 0

	for _, item := range lc.items {
		if now.After(item.expiresAt) {
			expired++
		}
	}

	return LocalCacheStats{
		Size:        len(lc.items),
		MaxSize:     lc.maxSize,
		ExpiredItems: expired,
		UtilizationRatio: float64(len(lc.items)) / float64(lc.maxSize),
	}
}

// Internal helper methods

func (lc *LocalCache) deleteItem(key string, item *localCacheItem) {
	delete(lc.items, key)
	lc.removeFromList(item.lruNode)
}

func (lc *LocalCache) evictIfNecessary() {
	for len(lc.items) > lc.maxSize {
		// Remove least recently used item
		if lc.lruList.tail.prev != lc.lruList.head {
			lru := lc.lruList.tail.prev
			lc.deleteItem(lru.key, lc.items[lru.key])
		}
	}
}

func (lc *LocalCache) addToFront(node *lruNode) {
	node.prev = lc.lruList.head
	node.next = lc.lruList.head.next
	lc.lruList.head.next.prev = node
	lc.lruList.head.next = node
	lc.lruList.size++
}

func (lc *LocalCache) removeFromList(node *lruNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
	lc.lruList.size--
}

func (lc *LocalCache) moveToFront(node *lruNode) {
	lc.removeFromList(node)
	lc.addToFront(node)
}

// matchesPattern performs simple pattern matching (supports * wildcard at the end)
func matchesPattern(key, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// Simple prefix matching for patterns ending with *
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix
	}

	// Exact match
	return key == pattern
}

// LocalCacheStats represents local cache statistics
type LocalCacheStats struct {
	Size             int     `json:"size"`
	MaxSize          int     `json:"max_size"`
	ExpiredItems     int     `json:"expired_items"`
	UtilizationRatio float64 `json:"utilization_ratio"`
}

// AutoCleanup starts a goroutine that periodically cleans up expired items
func (lc *LocalCache) AutoCleanup(interval time.Duration) chan struct{} {
	stopChan := make(chan struct{})
	
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				lc.CleanupExpired()
			case <-stopChan:
				return
			}
		}
	}()

	return stopChan
}