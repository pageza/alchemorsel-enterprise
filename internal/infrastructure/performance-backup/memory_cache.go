package performance

import (
	"container/list"
	"strings"
	"sync"
	"time"
)

// MemoryCache implements an in-memory LRU cache with TTL support
type MemoryCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*cacheItem
	lru      *list.List
}

type cacheItem struct {
	key       string
	value     []byte
	expiredAt time.Time
	element   *list.Element
}

// NewMemoryCache creates a new memory cache with the given capacity
func NewMemoryCache(capacity int) *MemoryCache {
	return &MemoryCache{
		capacity: capacity,
		items:    make(map[string]*cacheItem),
		lru:      list.New(),
	}
}

// Get retrieves a value from the cache
func (c *MemoryCache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// Check if item has expired
	if time.Now().After(item.expiredAt) {
		c.removeItem(item)
		return nil, false
	}

	// Move to front (most recently used)
	c.lru.MoveToFront(item.element)
	
	return item.value, true
}

// Set stores a value in the cache
func (c *MemoryCache) Set(key string, value []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiredAt := time.Now().Add(ttl)

	// If key already exists, update it
	if item, exists := c.items[key]; exists {
		item.value = value
		item.expiredAt = expiredAt
		c.lru.MoveToFront(item.element)
		return
	}

	// Create new item
	item := &cacheItem{
		key:       key,
		value:     value,
		expiredAt: expiredAt,
	}

	// Add to front of LRU list
	item.element = c.lru.PushFront(item)
	c.items[key] = item

	// Remove oldest items if capacity exceeded
	for len(c.items) > c.capacity {
		oldest := c.lru.Back()
		if oldest != nil {
			c.removeItem(oldest.Value.(*cacheItem))
		}
	}
}

// Delete removes a key from the cache
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, exists := c.items[key]; exists {
		c.removeItem(item)
	}
}

// Clear removes all items from the cache
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*cacheItem)
	c.lru = list.New()
}

// Size returns the current number of items in the cache
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Keys returns all keys in the cache
func (c *MemoryCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	for key := range c.items {
		keys = append(keys, key)
	}
	return keys
}

// InvalidatePattern removes all keys matching the given pattern (supports * wildcard)
func (c *MemoryCache) InvalidatePattern(pattern string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	keysToRemove := make([]*cacheItem, 0)
	
	for key, item := range c.items {
		if c.matchPattern(key, pattern) {
			keysToRemove = append(keysToRemove, item)
		}
	}

	for _, item := range keysToRemove {
		c.removeItem(item)
	}
}

// CleanupExpired removes all expired items from the cache
func (c *MemoryCache) CleanupExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiredItems := make([]*cacheItem, 0)

	for _, item := range c.items {
		if now.After(item.expiredAt) {
			expiredItems = append(expiredItems, item)
		}
	}

	for _, item := range expiredItems {
		c.removeItem(item)
	}

	return len(expiredItems)
}

// Stats returns cache statistics
func (c *MemoryCache) Stats() MemoryCacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	expired := 0
	
	for _, item := range c.items {
		if now.After(item.expiredAt) {
			expired++
		}
	}

	return MemoryCacheStats{
		Size:        len(c.items),
		Capacity:    c.capacity,
		ExpiredKeys: expired,
	}
}

// StartCleanupWorker starts a background goroutine to periodically clean up expired items
func (c *MemoryCache) StartCleanupWorker(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			c.CleanupExpired()
		}
	}()
}

// removeItem removes an item from both the map and LRU list
func (c *MemoryCache) removeItem(item *cacheItem) {
	delete(c.items, item.key)
	c.lru.Remove(item.element)
}

// matchPattern checks if a key matches a pattern (supports * wildcard)
func (c *MemoryCache) matchPattern(key, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if !strings.Contains(pattern, "*") {
		return key == pattern
	}

	// Simple wildcard matching
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return key == pattern
	}

	// Check if key starts with first part
	if len(parts[0]) > 0 && !strings.HasPrefix(key, parts[0]) {
		return false
	}

	// Check if key ends with last part
	if len(parts[len(parts)-1]) > 0 && !strings.HasSuffix(key, parts[len(parts)-1]) {
		return false
	}

	// Check middle parts
	searchKey := key
	if len(parts[0]) > 0 {
		searchKey = searchKey[len(parts[0]):]
	}
	if len(parts[len(parts)-1]) > 0 {
		searchKey = searchKey[:len(searchKey)-len(parts[len(parts)-1])]
	}

	for i := 1; i < len(parts)-1; i++ {
		if len(parts[i]) == 0 {
			continue
		}
		index := strings.Index(searchKey, parts[i])
		if index == -1 {
			return false
		}
		searchKey = searchKey[index+len(parts[i]):]
	}

	return true
}

// MemoryCacheStats represents memory cache statistics
type MemoryCacheStats struct {
	Size        int `json:"size"`
	Capacity    int `json:"capacity"`
	ExpiredKeys int `json:"expired_keys"`
}

// CacheWarmer handles cache warming strategies
type CacheWarmer struct {
	cache   *CacheManager
	logger  *zap.Logger
	recipes RecipeRepository
	users   UserRepository
}

// NewCacheWarmer creates a new cache warmer
func NewCacheWarmer(cache *CacheManager, logger *zap.Logger, recipes RecipeRepository, users UserRepository) *CacheWarmer {
	return &CacheWarmer{
		cache:   cache,
		logger:  logger,
		recipes: recipes,
		users:   users,
	}
}

// WarmPopularRecipes preloads popular recipes into cache
func (w *CacheWarmer) WarmPopularRecipes(ctx context.Context, limit int) error {
	w.logger.Info("Warming popular recipes cache", zap.Int("limit", limit))

	recipes, err := w.recipes.GetPopularRecipes(ctx, limit)
	if err != nil {
		return err
	}

	for _, recipe := range recipes {
		err := w.cache.SetRecipe(ctx, &recipe)
		if err != nil {
			w.logger.Error("Failed to warm recipe cache", 
				zap.String("recipe_id", recipe.ID), 
				zap.Error(err))
		}
	}

	w.logger.Info("Completed warming popular recipes cache", 
		zap.Int("cached_recipes", len(recipes)))

	return nil
}

// WarmRecentRecipes preloads recently created recipes
func (w *CacheWarmer) WarmRecentRecipes(ctx context.Context, limit int) error {
	recipes, err := w.recipes.GetRecentRecipes(ctx, limit)
	if err != nil {
		return err
	}

	for _, recipe := range recipes {
		err := w.cache.SetRecipe(ctx, &recipe)
		if err != nil {
			w.logger.Error("Failed to warm recent recipe cache", 
				zap.String("recipe_id", recipe.ID), 
				zap.Error(err))
		}
	}

	return nil
}

// WarmUserProfiles preloads active user profiles
func (w *CacheWarmer) WarmUserProfiles(ctx context.Context, userIDs []string) error {
	for _, userID := range userIDs {
		profile, err := w.users.GetProfile(ctx, userID)
		if err != nil {
			w.logger.Error("Failed to get user profile for warming", 
				zap.String("user_id", userID), 
				zap.Error(err))
			continue
		}

		err = w.cache.SetUserProfile(ctx, profile)
		if err != nil {
			w.logger.Error("Failed to warm user profile cache", 
				zap.String("user_id", userID), 
				zap.Error(err))
		}
	}

	return nil
}

// StartPeriodicWarming starts background cache warming
func (w *CacheWarmer) StartPeriodicWarming(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.WarmPopularRecipes(ctx, 100)
				w.WarmRecentRecipes(ctx, 50)
			}
		}
	}()
}

// Repository interfaces for cache warming
type RecipeRepository interface {
	GetPopularRecipes(ctx context.Context, limit int) ([]Recipe, error)
	GetRecentRecipes(ctx context.Context, limit int) ([]Recipe, error)
}

type UserRepository interface {
	GetProfile(ctx context.Context, userID string) (*UserProfile, error)
}