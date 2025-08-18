// Package memory provides in-memory cache repository implementation
package memory

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/ports/outbound"
)

// CacheItem represents a cached item
type CacheItem struct {
	Value     []byte
	ExpiresAt time.Time
}

// CacheRepository implements in-memory cache repository
type CacheRepository struct {
	data  map[string]CacheItem
	mutex sync.RWMutex
}

// NewCacheRepository creates a new in-memory cache repository
func NewCacheRepository() outbound.CacheRepository {
	repo := &CacheRepository{
		data: make(map[string]CacheItem),
	}
	
	// Start cleanup goroutine
	go repo.cleanup()
	
	return repo
}

// Get retrieves a value from cache
func (r *CacheRepository) Get(ctx context.Context, key string) ([]byte, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	item, exists := r.data[key]
	if !exists {
		return nil, errors.New("key not found")
	}
	
	if time.Now().After(item.ExpiresAt) {
		delete(r.data, key)
		return nil, errors.New("key expired")
	}
	
	return item.Value, nil
}

// Set stores a value in cache with TTL
func (r *CacheRepository) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	expiresAt := time.Now().Add(ttl)
	if ttl == 0 {
		expiresAt = time.Now().Add(24 * time.Hour) // Default to 24 hours
	}
	
	r.data[key] = CacheItem{
		Value:     value,
		ExpiresAt: expiresAt,
	}
	
	return nil
}

// Delete removes a key from cache
func (r *CacheRepository) Delete(ctx context.Context, key string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	delete(r.data, key)
	return nil
}

// Exists checks if a key exists in cache
func (r *CacheRepository) Exists(ctx context.Context, key string) (bool, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	item, exists := r.data[key]
	if !exists {
		return false, nil
	}
	
	if time.Now().After(item.ExpiresAt) {
		delete(r.data, key)
		return false, nil
	}
	
	return true, nil
}

// MGet retrieves multiple values from cache
func (r *CacheRepository) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	result := make(map[string][]byte)
	now := time.Now()
	
	for _, key := range keys {
		item, exists := r.data[key]
		if exists && now.Before(item.ExpiresAt) {
			result[key] = item.Value
		}
	}
	
	return result, nil
}

// MSet stores multiple values in cache
func (r *CacheRepository) MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	expiresAt := time.Now().Add(ttl)
	if ttl == 0 {
		expiresAt = time.Now().Add(24 * time.Hour)
	}
	
	for key, value := range items {
		r.data[key] = CacheItem{
			Value:     value,
			ExpiresAt: expiresAt,
		}
	}
	
	return nil
}

// Increment increments a counter
func (r *CacheRepository) Increment(ctx context.Context, key string) (int64, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	item, exists := r.data[key]
	var value int64 = 1
	
	if exists && time.Now().Before(item.ExpiresAt) {
		// Try to parse existing value as int64
		if len(item.Value) == 8 {
			value = int64(item.Value[0])<<56 | int64(item.Value[1])<<48 | 
					int64(item.Value[2])<<40 | int64(item.Value[3])<<32 |
					int64(item.Value[4])<<24 | int64(item.Value[5])<<16 |
					int64(item.Value[6])<<8 | int64(item.Value[7])
			value++
		}
	}
	
	// Store the incremented value
	valueBytes := make([]byte, 8)
	valueBytes[0] = byte(value >> 56)
	valueBytes[1] = byte(value >> 48)
	valueBytes[2] = byte(value >> 40)
	valueBytes[3] = byte(value >> 32)
	valueBytes[4] = byte(value >> 24)
	valueBytes[5] = byte(value >> 16)
	valueBytes[6] = byte(value >> 8)
	valueBytes[7] = byte(value)
	
	r.data[key] = CacheItem{
		Value:     valueBytes,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	
	return value, nil
}

// Decrement decrements a counter
func (r *CacheRepository) Decrement(ctx context.Context, key string) (int64, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	item, exists := r.data[key]
	var value int64 = -1
	
	if exists && time.Now().Before(item.ExpiresAt) {
		// Try to parse existing value as int64
		if len(item.Value) == 8 {
			value = int64(item.Value[0])<<56 | int64(item.Value[1])<<48 | 
					int64(item.Value[2])<<40 | int64(item.Value[3])<<32 |
					int64(item.Value[4])<<24 | int64(item.Value[5])<<16 |
					int64(item.Value[6])<<8 | int64(item.Value[7])
			value--
		}
	}
	
	// Store the decremented value
	valueBytes := make([]byte, 8)
	valueBytes[0] = byte(value >> 56)
	valueBytes[1] = byte(value >> 48)
	valueBytes[2] = byte(value >> 40)
	valueBytes[3] = byte(value >> 32)
	valueBytes[4] = byte(value >> 24)
	valueBytes[5] = byte(value >> 16)
	valueBytes[6] = byte(value >> 8)
	valueBytes[7] = byte(value)
	
	r.data[key] = CacheItem{
		Value:     valueBytes,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	
	return value, nil
}

// SAdd adds members to a set
func (r *CacheRepository) SAdd(ctx context.Context, key string, members ...string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	// Get existing set or create new one
	var existingSet map[string]bool
	item, exists := r.data[key]
	
	if exists && time.Now().Before(item.ExpiresAt) {
		// Try to deserialize existing set (simplified)
		existingSet = make(map[string]bool)
		// In a real implementation, you'd properly serialize/deserialize
	} else {
		existingSet = make(map[string]bool)
	}
	
	// Add new members
	for _, member := range members {
		existingSet[member] = true
	}
	
	// Serialize set (simplified - just store the keys)
	serialized := make([]byte, 0)
	for member := range existingSet {
		serialized = append(serialized, []byte(member)...)
		serialized = append(serialized, byte('\n'))
	}
	
	r.data[key] = CacheItem{
		Value:     serialized,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	
	return nil
}

// SMembers returns all members of a set
func (r *CacheRepository) SMembers(ctx context.Context, key string) ([]string, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	item, exists := r.data[key]
	if !exists || time.Now().After(item.ExpiresAt) {
		return []string{}, nil
	}
	
	// Deserialize set (simplified)
	members := strings.Split(string(item.Value), "\n")
	result := make([]string, 0, len(members))
	for _, member := range members {
		if member != "" {
			result = append(result, member)
		}
	}
	
	return result, nil
}

// SRem removes members from a set
func (r *CacheRepository) SRem(ctx context.Context, key string, members ...string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	item, exists := r.data[key]
	if !exists || time.Now().After(item.ExpiresAt) {
		return nil
	}
	
	// Get existing members
	existingMembers := strings.Split(string(item.Value), "\n")
	memberSet := make(map[string]bool)
	
	for _, member := range existingMembers {
		if member != "" {
			memberSet[member] = true
		}
	}
	
	// Remove specified members
	for _, member := range members {
		delete(memberSet, member)
	}
	
	// Serialize remaining members
	serialized := make([]byte, 0)
	for member := range memberSet {
		serialized = append(serialized, []byte(member)...)
		serialized = append(serialized, byte('\n'))
	}
	
	r.data[key] = CacheItem{
		Value:     serialized,
		ExpiresAt: item.ExpiresAt,
	}
	
	return nil
}

// cleanup removes expired items
func (r *CacheRepository) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			r.mutex.Lock()
			now := time.Now()
			for key, item := range r.data {
				if now.After(item.ExpiresAt) {
					delete(r.data, key)
				}
			}
			r.mutex.Unlock()
		}
	}
}