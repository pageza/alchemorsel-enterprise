package network

import (
	"sync"
	"time"
)

// RoutingTable manages routing decisions and caching
type RoutingTable struct {
	entries map[string]*RoutingEntry
	mu      sync.RWMutex
	metrics *RoutingMetrics
}

// RoutingEntry represents a cached routing decision
type RoutingEntry struct {
	ClientIP       string
	SelectedRegion string
	Decision       *RoutingDecision
	CreatedAt      time.Time
	LastUsed       time.Time
	UsageCount     int
	TTL            time.Duration
}

// RoutingMetrics tracks routing table performance
type RoutingMetrics struct {
	CacheHits      int64
	CacheMisses    int64
	TotalLookups   int64
	Evictions      int64
	mu             sync.RWMutex
}

// NewRoutingTable creates a new routing table
func NewRoutingTable() *RoutingTable {
	rt := &RoutingTable{
		entries: make(map[string]*RoutingEntry),
		metrics: &RoutingMetrics{},
	}
	
	// Start cleanup goroutine
	go rt.cleanupExpired()
	
	return rt
}

// Get retrieves a routing decision from cache
func (rt *RoutingTable) Get(clientIP string) (*RoutingDecision, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	rt.metrics.mu.Lock()
	rt.metrics.TotalLookups++
	rt.metrics.mu.Unlock()

	entry, exists := rt.entries[clientIP]
	if !exists {
		rt.metrics.mu.Lock()
		rt.metrics.CacheMisses++
		rt.metrics.mu.Unlock()
		return nil, false
	}

	// Check if entry has expired
	if time.Since(entry.CreatedAt) > entry.TTL {
		delete(rt.entries, clientIP)
		rt.metrics.mu.Lock()
		rt.metrics.CacheMisses++
		rt.metrics.Evictions++
		rt.metrics.mu.Unlock()
		return nil, false
	}

	// Update usage statistics
	entry.LastUsed = time.Now()
	entry.UsageCount++

	rt.metrics.mu.Lock()
	rt.metrics.CacheHits++
	rt.metrics.mu.Unlock()

	return entry.Decision, true
}

// Set stores a routing decision in cache
func (rt *RoutingTable) Set(clientIP string, decision *RoutingDecision) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	entry := &RoutingEntry{
		ClientIP:       clientIP,
		SelectedRegion: decision.SelectedRegion.Name,
		Decision:       decision,
		CreatedAt:      time.Now(),
		LastUsed:       time.Now(),
		UsageCount:     1,
		TTL:            decision.TTL,
	}

	rt.entries[clientIP] = entry
}

// Delete removes a routing entry
func (rt *RoutingTable) Delete(clientIP string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if _, exists := rt.entries[clientIP]; exists {
		delete(rt.entries, clientIP)
		rt.metrics.mu.Lock()
		rt.metrics.Evictions++
		rt.metrics.mu.Unlock()
	}
}

// GetStats returns routing table statistics
func (rt *RoutingTable) GetStats() RoutingTableStats {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	rt.metrics.mu.RLock()
	defer rt.metrics.mu.RUnlock()

	hitRatio := float64(0)
	if rt.metrics.TotalLookups > 0 {
		hitRatio = float64(rt.metrics.CacheHits) / float64(rt.metrics.TotalLookups)
	}

	return RoutingTableStats{
		TotalEntries:  len(rt.entries),
		CacheHits:     rt.metrics.CacheHits,
		CacheMisses:   rt.metrics.CacheMisses,
		TotalLookups:  rt.metrics.TotalLookups,
		HitRatio:      hitRatio,
		Evictions:     rt.metrics.Evictions,
	}
}

// GetRegionDistribution returns distribution of cached decisions by region
func (rt *RoutingTable) GetRegionDistribution() map[string]int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	distribution := make(map[string]int)
	for _, entry := range rt.entries {
		distribution[entry.SelectedRegion]++
	}

	return distribution
}

// cleanupExpired removes expired entries
func (rt *RoutingTable) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rt.performCleanup()
	}
}

// performCleanup removes expired and least recently used entries
func (rt *RoutingTable) performCleanup() {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	// Find expired entries
	for clientIP, entry := range rt.entries {
		if now.Sub(entry.CreatedAt) > entry.TTL {
			expiredKeys = append(expiredKeys, clientIP)
		}
	}

	// Remove expired entries
	for _, key := range expiredKeys {
		delete(rt.entries, key)
		rt.metrics.mu.Lock()
		rt.metrics.Evictions++
		rt.metrics.mu.Unlock()
	}

	// If still too many entries, remove least recently used
	maxEntries := 10000
	if len(rt.entries) > maxEntries {
		rt.evictLRU(len(rt.entries) - maxEntries)
	}
}

// evictLRU removes the least recently used entries
func (rt *RoutingTable) evictLRU(count int) {
	if count <= 0 {
		return
	}

	// Create slice of entries sorted by last used time
	type entryWithKey struct {
		key   string
		entry *RoutingEntry
	}

	entries := make([]entryWithKey, 0, len(rt.entries))
	for key, entry := range rt.entries {
		entries = append(entries, entryWithKey{key: key, entry: entry})
	}

	// Sort by last used time (oldest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].entry.LastUsed.Before(entries[j].entry.LastUsed)
	})

	// Remove oldest entries
	evicted := 0
	for i := 0; i < len(entries) && evicted < count; i++ {
		delete(rt.entries, entries[i].key)
		evicted++
	}

	rt.metrics.mu.Lock()
	rt.metrics.Evictions += int64(evicted)
	rt.metrics.mu.Unlock()
}

// GetTopClients returns the most active clients
func (rt *RoutingTable) GetTopClients(limit int) []ClientActivity {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	activities := make([]ClientActivity, 0, len(rt.entries))
	for clientIP, entry := range rt.entries {
		activities = append(activities, ClientActivity{
			ClientIP:    clientIP,
			Region:      entry.SelectedRegion,
			UsageCount:  entry.UsageCount,
			LastUsed:    entry.LastUsed,
		})
	}

	// Sort by usage count (descending)
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].UsageCount > activities[j].UsageCount
	})

	if limit > 0 && limit < len(activities) {
		activities = activities[:limit]
	}

	return activities
}

// InvalidateRegion removes all entries for a specific region
func (rt *RoutingTable) InvalidateRegion(regionName string) int {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	keysToDelete := make([]string, 0)
	for clientIP, entry := range rt.entries {
		if entry.SelectedRegion == regionName {
			keysToDelete = append(keysToDelete, clientIP)
		}
	}

	for _, key := range keysToDelete {
		delete(rt.entries, key)
	}

	rt.metrics.mu.Lock()
	rt.metrics.Evictions += int64(len(keysToDelete))
	rt.metrics.mu.Unlock()

	return len(keysToDelete)
}

// UpdateDecision updates an existing routing decision
func (rt *RoutingTable) UpdateDecision(clientIP string, decision *RoutingDecision) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	entry, exists := rt.entries[clientIP]
	if !exists {
		return false
	}

	entry.Decision = decision
	entry.SelectedRegion = decision.SelectedRegion.Name
	entry.LastUsed = time.Now()

	return true
}

// GetActiveRegions returns list of regions currently being used
func (rt *RoutingTable) GetActiveRegions() []string {
	distribution := rt.GetRegionDistribution()
	regions := make([]string, 0, len(distribution))
	
	for region := range distribution {
		regions = append(regions, region)
	}
	
	sort.Strings(regions)
	return regions
}

// Data structures
type RoutingTableStats struct {
	TotalEntries int64
	CacheHits    int64
	CacheMisses  int64
	TotalLookups int64
	HitRatio     float64
	Evictions    int64
}

type ClientActivity struct {
	ClientIP   string
	Region     string
	UsageCount int
	LastUsed   time.Time
}

// NetworkMetrics tracks various network performance metrics
type NetworkMetrics struct {
	routingDecisionTimes []time.Duration
	regionPerformance    map[string]*RegionPerformanceMetrics
	mu                   sync.RWMutex
}

type RegionPerformanceMetrics struct {
	SuccessRate         float64
	AverageResponseTime time.Duration
	Throughput          float64
	ErrorCount          int64
	RequestCount        int64
	LastUpdated         time.Time
}

// NewNetworkMetrics creates a new network metrics instance
func NewNetworkMetrics() *NetworkMetrics {
	return &NetworkMetrics{
		routingDecisionTimes: make([]time.Duration, 0),
		regionPerformance:    make(map[string]*RegionPerformanceMetrics),
	}
}

// RecordRoutingDecisionTime records the time taken to make a routing decision
func (nm *NetworkMetrics) RecordRoutingDecisionTime(duration time.Duration) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.routingDecisionTimes = append(nm.routingDecisionTimes, duration)
	
	// Keep only recent measurements
	maxSamples := 1000
	if len(nm.routingDecisionTimes) > maxSamples {
		nm.routingDecisionTimes = nm.routingDecisionTimes[1:]
	}
}

// GetRegionPerformance returns performance metrics for a region
func (nm *NetworkMetrics) GetRegionPerformance(regionName, requestType string) *RegionPerformanceMetrics {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	key := fmt.Sprintf("%s_%s", regionName, requestType)
	if metrics, exists := nm.regionPerformance[key]; exists {
		return metrics
	}

	// Return default metrics if not found
	return &RegionPerformanceMetrics{
		SuccessRate:         0.99,
		AverageResponseTime: 100 * time.Millisecond,
		Throughput:          100.0,
		LastUpdated:         time.Now(),
	}
}

// UpdateRegionPerformance updates performance metrics for a region
func (nm *NetworkMetrics) UpdateRegionPerformance(regionName, requestType string, 
	successRate float64, avgResponseTime time.Duration, throughput float64) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	key := fmt.Sprintf("%s_%s", regionName, requestType)
	nm.regionPerformance[key] = &RegionPerformanceMetrics{
		SuccessRate:         successRate,
		AverageResponseTime: avgResponseTime,
		Throughput:          throughput,
		LastUpdated:         time.Now(),
	}
}

// GetAverageRoutingDecisionTime returns the average time to make routing decisions
func (nm *NetworkMetrics) GetAverageRoutingDecisionTime() time.Duration {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	if len(nm.routingDecisionTimes) == 0 {
		return 0
	}

	var total time.Duration
	for _, duration := range nm.routingDecisionTimes {
		total += duration
	}

	return total / time.Duration(len(nm.routingDecisionTimes))
}