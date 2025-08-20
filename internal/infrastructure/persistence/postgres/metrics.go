// Package postgres provides database metrics and monitoring
package postgres

import (
	"database/sql"
	"sync"
	"time"
)

// ConnectionMetrics tracks database connection and performance metrics
type ConnectionMetrics struct {
	mu sync.RWMutex
	
	// Connection Pool Metrics
	OpenConnections     int `json:"open_connections"`
	InUse              int `json:"in_use"`
	Idle               int `json:"idle"`
	MaxOpenConnections int `json:"max_open_connections"`
	
	// Connection Wait Metrics
	WaitCount         int64         `json:"wait_count"`
	WaitDuration      time.Duration `json:"wait_duration"`
	MaxIdleClosed     int64         `json:"max_idle_closed"`
	MaxIdleTimeClosed int64         `json:"max_idle_time_closed"`
	MaxLifetimeClosed int64         `json:"max_lifetime_closed"`
	
	// Query Performance Metrics
	TotalQueries       int64         `json:"total_queries"`
	SlowQueries        int64         `json:"slow_queries"`
	FailedQueries      int64         `json:"failed_queries"`
	AverageQueryTime   time.Duration `json:"average_query_time"`
	
	// Cache Metrics
	CacheHits          int64 `json:"cache_hits"`
	CacheMisses        int64 `json:"cache_misses"`
	CacheHitRatio      float64 `json:"cache_hit_ratio"`
	
	// Index Usage Metrics
	IndexScans         int64   `json:"index_scans"`
	SeqScans          int64   `json:"seq_scans"`
	IndexUsageRatio   float64 `json:"index_usage_ratio"`
	
	LastUpdated       time.Time `json:"last_updated"`
}

// NewConnectionMetrics creates a new metrics instance
func NewConnectionMetrics() *ConnectionMetrics {
	return &ConnectionMetrics{
		LastUpdated: time.Now(),
	}
}

// UpdateConnectionStats updates connection pool statistics
func (m *ConnectionMetrics) UpdateConnectionStats(stats sql.DBStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.OpenConnections = stats.OpenConnections
	m.InUse = stats.InUse
	m.Idle = stats.Idle
	m.MaxOpenConnections = stats.MaxOpenConnections
	m.WaitCount = stats.WaitCount
	m.WaitDuration = stats.WaitDuration
	m.MaxIdleClosed = stats.MaxIdleClosed
	m.MaxIdleTimeClosed = stats.MaxIdleTimeClosed
	m.MaxLifetimeClosed = stats.MaxLifetimeClosed
	m.LastUpdated = time.Now()
}

// UpdateQueryStats updates query performance statistics
func (m *ConnectionMetrics) UpdateQueryStats(stats QueryStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.TotalQueries = stats.TotalQueries
	m.SlowQueries = stats.SlowQueries
	m.FailedQueries = stats.FailedQueries
	m.AverageQueryTime = stats.AverageQueryTime
	m.LastUpdated = time.Now()
}

// UpdateCacheStats updates cache performance statistics
func (m *ConnectionMetrics) UpdateCacheStats(hits, misses int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.CacheHits = hits
	m.CacheMisses = misses
	
	total := hits + misses
	if total > 0 {
		m.CacheHitRatio = float64(hits) / float64(total) * 100
	}
	
	m.LastUpdated = time.Now()
}

// UpdateIndexStats updates index usage statistics
func (m *ConnectionMetrics) UpdateIndexStats(indexScans, seqScans int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.IndexScans = indexScans
	m.SeqScans = seqScans
	
	total := indexScans + seqScans
	if total > 0 {
		m.IndexUsageRatio = float64(indexScans) / float64(total) * 100
	}
	
	m.LastUpdated = time.Now()
}

// GetSnapshot returns a snapshot of current metrics
func (m *ConnectionMetrics) GetSnapshot() ConnectionMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return *m
}

// GetConnectionEfficiency returns connection pool efficiency percentage
func (m *ConnectionMetrics) GetConnectionEfficiency() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.MaxOpenConnections == 0 {
		return 0
	}
	
	utilization := float64(m.InUse) / float64(m.MaxOpenConnections) * 100
	return utilization
}

// GetQuerySuccessRate returns query success rate percentage
func (m *ConnectionMetrics) GetQuerySuccessRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.TotalQueries == 0 {
		return 100
	}
	
	successQueries := m.TotalQueries - m.FailedQueries
	return float64(successQueries) / float64(m.TotalQueries) * 100
}

// GetSlowQueryRatio returns slow query ratio percentage
func (m *ConnectionMetrics) GetSlowQueryRatio() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.TotalQueries == 0 {
		return 0
	}
	
	return float64(m.SlowQueries) / float64(m.TotalQueries) * 100
}

// IsHealthy returns true if all metrics are within healthy thresholds
func (m *ConnectionMetrics) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Check connection pool health
	if m.GetConnectionEfficiency() > 90 {
		return false // Pool is overutilized
	}
	
	// Check query performance
	if m.GetSlowQueryRatio() > 5 { // More than 5% slow queries
		return false
	}
	
	// Check query success rate
	if m.GetQuerySuccessRate() < 99 { // Less than 99% success rate
		return false
	}
	
	// Check cache hit ratio
	if m.CacheHitRatio < 90 { // Less than 90% cache hit ratio
		return false
	}
	
	// Check index usage
	if m.IndexUsageRatio < 95 { // Less than 95% index usage
		return false
	}
	
	return true
}

// GetRecommendations returns performance optimization recommendations
func (m *ConnectionMetrics) GetRecommendations() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var recommendations []string
	
	// Connection pool recommendations
	efficiency := m.GetConnectionEfficiency()
	if efficiency > 90 {
		recommendations = append(recommendations, "Consider increasing max_open_conns - connection pool is heavily utilized")
	} else if efficiency < 10 {
		recommendations = append(recommendations, "Consider decreasing max_open_conns - connection pool is underutilized")
	}
	
	// Query performance recommendations
	slowRatio := m.GetSlowQueryRatio()
	if slowRatio > 10 {
		recommendations = append(recommendations, "High slow query ratio detected - review and optimize slow queries")
	} else if slowRatio > 5 {
		recommendations = append(recommendations, "Moderate slow query ratio - consider query optimization")
	}
	
	// Cache recommendations
	if m.CacheHitRatio < 90 && m.CacheHits+m.CacheMisses > 100 {
		recommendations = append(recommendations, "Low cache hit ratio - consider increasing cache TTL or size")
	}
	
	// Index recommendations
	if m.IndexUsageRatio < 95 && m.IndexScans+m.SeqScans > 100 {
		recommendations = append(recommendations, "Low index usage ratio - review queries for missing indexes")
	}
	
	// Connection wait recommendations
	if m.WaitCount > 100 {
		recommendations = append(recommendations, "High connection wait count - consider increasing connection pool size")
	}
	
	return recommendations
}

// MetricsReport provides a comprehensive metrics report
type MetricsReport struct {
	Timestamp     time.Time            `json:"timestamp"`
	Metrics       ConnectionMetrics    `json:"metrics"`
	Health        HealthStatus         `json:"health"`
	Recommendations []string           `json:"recommendations"`
}

// HealthStatus represents overall database health
type HealthStatus struct {
	IsHealthy           bool    `json:"is_healthy"`
	ConnectionHealth    string  `json:"connection_health"`
	QueryHealth         string  `json:"query_health"`
	CacheHealth         string  `json:"cache_health"`
	IndexHealth         string  `json:"index_health"`
	OverallScore        float64 `json:"overall_score"`
}

// GenerateReport generates a comprehensive metrics report
func (m *ConnectionMetrics) GenerateReport() MetricsReport {
	snapshot := m.GetSnapshot()
	health := m.calculateHealth()
	recommendations := m.GetRecommendations()
	
	return MetricsReport{
		Timestamp:       time.Now(),
		Metrics:         snapshot,
		Health:          health,
		Recommendations: recommendations,
	}
}

// calculateHealth calculates overall health status
func (m *ConnectionMetrics) calculateHealth() HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var scores []float64
	
	// Connection health (0-100)
	connEfficiency := m.GetConnectionEfficiency()
	connHealth := "healthy"
	connScore := 100.0
	
	if connEfficiency > 90 {
		connHealth = "critical"
		connScore = 20.0
	} else if connEfficiency > 80 {
		connHealth = "warning"
		connScore = 60.0
	} else if connEfficiency < 10 {
		connHealth = "underutilized"
		connScore = 80.0
	}
	scores = append(scores, connScore)
	
	// Query health (0-100)
	slowRatio := m.GetSlowQueryRatio()
	successRate := m.GetQuerySuccessRate()
	queryHealth := "healthy"
	queryScore := 100.0
	
	if successRate < 95 {
		queryHealth = "critical"
		queryScore = 20.0
	} else if slowRatio > 10 {
		queryHealth = "critical"
		queryScore = 30.0
	} else if slowRatio > 5 {
		queryHealth = "warning"
		queryScore = 70.0
	}
	scores = append(scores, queryScore)
	
	// Cache health (0-100)
	cacheHealth := "healthy"
	cacheScore := 100.0
	
	if m.CacheHitRatio < 80 && m.CacheHits+m.CacheMisses > 100 {
		cacheHealth = "warning"
		cacheScore = 60.0
	} else if m.CacheHitRatio < 70 {
		cacheHealth = "critical"
		cacheScore = 30.0
	}
	scores = append(scores, cacheScore)
	
	// Index health (0-100)
	indexHealth := "healthy"
	indexScore := 100.0
	
	if m.IndexUsageRatio < 90 && m.IndexScans+m.SeqScans > 100 {
		indexHealth = "warning"
		indexScore = 60.0
	} else if m.IndexUsageRatio < 80 {
		indexHealth = "critical"
		indexScore = 30.0
	}
	scores = append(scores, indexScore)
	
	// Calculate overall score
	var totalScore float64
	for _, score := range scores {
		totalScore += score
	}
	overallScore := totalScore / float64(len(scores))
	
	return HealthStatus{
		IsHealthy:        m.IsHealthy(),
		ConnectionHealth: connHealth,
		QueryHealth:      queryHealth,
		CacheHealth:      cacheHealth,
		IndexHealth:      indexHealth,
		OverallScore:     overallScore,
	}
}