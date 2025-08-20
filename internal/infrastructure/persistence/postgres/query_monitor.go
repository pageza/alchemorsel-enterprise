// Package postgres provides query performance monitoring
package postgres

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// QueryMonitor tracks and analyzes query performance
type QueryMonitor struct {
	logger      *zap.Logger
	mu          sync.RWMutex
	stats       QueryStats
	slowQueries []SlowQuery
	maxSlowLogs int
}

// QueryStats holds aggregated query statistics
type QueryStats struct {
	TotalQueries     int64         `json:"total_queries"`
	SlowQueries      int64         `json:"slow_queries"`
	FailedQueries    int64         `json:"failed_queries"`
	AverageQueryTime time.Duration `json:"average_query_time"`
	TotalQueryTime   time.Duration `json:"total_query_time"`
	LastReset        time.Time     `json:"last_reset"`
}

// SlowQuery represents a slow query with context
type SlowQuery struct {
	SQL        string        `json:"sql"`
	Duration   time.Duration `json:"duration"`
	Timestamp  time.Time     `json:"timestamp"`
	Error      string        `json:"error,omitempty"`
	StackTrace string        `json:"stack_trace,omitempty"`
}

// QueryContext holds query execution context
type QueryContext struct {
	StartTime time.Time
	SQL       string
}

// NewQueryMonitor creates a new query monitor
func NewQueryMonitor(logger *zap.Logger) *QueryMonitor {
	return &QueryMonitor{
		logger:      logger,
		stats:       QueryStats{LastReset: time.Now()},
		slowQueries: make([]SlowQuery, 0),
		maxSlowLogs: 1000, // Keep last 1000 slow queries
	}
}

// BeforeQuery is called before query execution
func (qm *QueryMonitor) BeforeQuery(db *gorm.DB) {
	if db.Statement == nil {
		return
	}
	
	ctx := &QueryContext{
		StartTime: time.Now(),
		SQL:       db.Statement.SQL.String(),
	}
	
	db.InstanceSet("query_monitor_context", ctx)
}

// AfterQuery is called after query execution
func (qm *QueryMonitor) AfterQuery(db *gorm.DB) {
	if db.Statement == nil {
		return
	}
	
	ctxInterface, exists := db.InstanceGet("query_monitor_context")
	if !exists {
		return
	}
	
	ctx, ok := ctxInterface.(*QueryContext)
	if !ok {
		return
	}
	
	duration := time.Since(ctx.StartTime)
	
	qm.recordQuery(ctx.SQL, duration, db.Error)
}

// recordQuery records query execution metrics
func (qm *QueryMonitor) recordQuery(sql string, duration time.Duration, err error) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	
	qm.stats.TotalQueries++
	qm.stats.TotalQueryTime += duration
	qm.stats.AverageQueryTime = qm.stats.TotalQueryTime / time.Duration(qm.stats.TotalQueries)
	
	if err != nil {
		qm.stats.FailedQueries++
	}
	
	// Check for slow queries (threshold: 100ms)
	if duration > 100*time.Millisecond {
		qm.stats.SlowQueries++
		
		slowQuery := SlowQuery{
			SQL:       qm.sanitizeSQL(sql),
			Duration:  duration,
			Timestamp: time.Now(),
		}
		
		if err != nil {
			slowQuery.Error = err.Error()
		}
		
		qm.recordSlowQuery(slowQuery)
		
		// Log slow query
		qm.logger.Warn("Slow query detected",
			zap.Duration("duration", duration),
			zap.String("sql", slowQuery.SQL),
			zap.Error(err),
		)
	}
}

// recordSlowQuery records a slow query with circular buffer
func (qm *QueryMonitor) recordSlowQuery(query SlowQuery) {
	if len(qm.slowQueries) >= qm.maxSlowLogs {
		// Remove oldest entry
		qm.slowQueries = qm.slowQueries[1:]
	}
	
	qm.slowQueries = append(qm.slowQueries, query)
}

// sanitizeSQL removes sensitive data from SQL for logging
func (qm *QueryMonitor) sanitizeSQL(sql string) string {
	// Replace potential sensitive values with placeholders
	sanitized := strings.ReplaceAll(sql, "'", "?")
	
	// Limit length for readability
	if len(sanitized) > 500 {
		sanitized = sanitized[:500] + "..."
	}
	
	return sanitized
}

// GetStats returns current query statistics
func (qm *QueryMonitor) GetStats() QueryStats {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	
	return qm.stats
}

// GetSlowQueries returns recent slow queries
func (qm *QueryMonitor) GetSlowQueries(limit int) []SlowQuery {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	
	if limit == 0 || limit > len(qm.slowQueries) {
		limit = len(qm.slowQueries)
	}
	
	// Return most recent queries
	start := len(qm.slowQueries) - limit
	if start < 0 {
		start = 0
	}
	
	result := make([]SlowQuery, limit)
	copy(result, qm.slowQueries[start:])
	
	return result
}

// ResetStats resets all statistics
func (qm *QueryMonitor) ResetStats() {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	
	qm.stats = QueryStats{LastReset: time.Now()}
	qm.slowQueries = make([]SlowQuery, 0)
}

// GetTopSlowQueries returns the slowest queries grouped by pattern
func (qm *QueryMonitor) GetTopSlowQueries(limit int) []QueryPattern {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	
	patterns := make(map[string]*QueryPattern)
	
	for _, query := range qm.slowQueries {
		pattern := qm.extractQueryPattern(query.SQL)
		
		if p, exists := patterns[pattern]; exists {
			p.Count++
			p.TotalDuration += query.Duration
			if query.Duration > p.MaxDuration {
				p.MaxDuration = query.Duration
			}
			if query.Duration < p.MinDuration {
				p.MinDuration = query.Duration
			}
		} else {
			patterns[pattern] = &QueryPattern{
				Pattern:       pattern,
				Count:         1,
				TotalDuration: query.Duration,
				MaxDuration:   query.Duration,
				MinDuration:   query.Duration,
				LastSeen:      query.Timestamp,
			}
		}
	}
	
	// Convert to slice and sort by total duration
	result := make([]QueryPattern, 0, len(patterns))
	for _, pattern := range patterns {
		pattern.AverageDuration = pattern.TotalDuration / time.Duration(pattern.Count)
		result = append(result, *pattern)
	}
	
	// Sort by total duration (highest first)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].TotalDuration < result[j].TotalDuration {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	
	return result
}

// QueryPattern represents a query pattern with aggregated metrics
type QueryPattern struct {
	Pattern         string        `json:"pattern"`
	Count           int           `json:"count"`
	TotalDuration   time.Duration `json:"total_duration"`
	AverageDuration time.Duration `json:"average_duration"`
	MaxDuration     time.Duration `json:"max_duration"`
	MinDuration     time.Duration `json:"min_duration"`
	LastSeen        time.Time     `json:"last_seen"`
}

// extractQueryPattern extracts a pattern from SQL query
func (qm *QueryMonitor) extractQueryPattern(sql string) string {
	// Normalize whitespace
	normalized := strings.Join(strings.Fields(sql), " ")
	
	// Extract query type and main table
	parts := strings.Split(strings.ToUpper(normalized), " ")
	if len(parts) == 0 {
		return "UNKNOWN"
	}
	
	queryType := parts[0]
	
	// Try to identify main table for common operations
	switch queryType {
	case "SELECT":
		if idx := findWordIndex(parts, "FROM"); idx != -1 && idx+1 < len(parts) {
			return fmt.Sprintf("SELECT FROM %s", parts[idx+1])
		}
	case "INSERT":
		if idx := findWordIndex(parts, "INTO"); idx != -1 && idx+1 < len(parts) {
			return fmt.Sprintf("INSERT INTO %s", parts[idx+1])
		}
	case "UPDATE":
		if len(parts) > 1 {
			return fmt.Sprintf("UPDATE %s", parts[1])
		}
	case "DELETE":
		if idx := findWordIndex(parts, "FROM"); idx != -1 && idx+1 < len(parts) {
			return fmt.Sprintf("DELETE FROM %s", parts[idx+1])
		}
	}
	
	return queryType
}

// findWordIndex finds the index of a word in a slice
func findWordIndex(words []string, target string) int {
	for i, word := range words {
		if word == target {
			return i
		}
	}
	return -1
}

// GetQueryAnalysis provides detailed query performance analysis
func (qm *QueryMonitor) GetQueryAnalysis() QueryAnalysis {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	
	analysis := QueryAnalysis{
		Timestamp:    time.Now(),
		TotalQueries: qm.stats.TotalQueries,
		SlowQueries:  qm.stats.SlowQueries,
		FailedQueries: qm.stats.FailedQueries,
		AverageQueryTime: qm.stats.AverageQueryTime,
	}
	
	if qm.stats.TotalQueries > 0 {
		analysis.SlowQueryRatio = float64(qm.stats.SlowQueries) / float64(qm.stats.TotalQueries) * 100
		analysis.FailureRate = float64(qm.stats.FailedQueries) / float64(qm.stats.TotalQueries) * 100
	}
	
	// Get query patterns
	analysis.TopSlowPatterns = qm.GetTopSlowQueries(10)
	
	// Generate recommendations
	analysis.Recommendations = qm.generateQueryRecommendations()
	
	return analysis
}

// QueryAnalysis provides comprehensive query performance analysis
type QueryAnalysis struct {
	Timestamp        time.Time      `json:"timestamp"`
	TotalQueries     int64          `json:"total_queries"`
	SlowQueries      int64          `json:"slow_queries"`
	FailedQueries    int64          `json:"failed_queries"`
	AverageQueryTime time.Duration  `json:"average_query_time"`
	SlowQueryRatio   float64        `json:"slow_query_ratio"`
	FailureRate      float64        `json:"failure_rate"`
	TopSlowPatterns  []QueryPattern `json:"top_slow_patterns"`
	Recommendations  []string       `json:"recommendations"`
}

// generateQueryRecommendations generates optimization recommendations
func (qm *QueryMonitor) generateQueryRecommendations() []string {
	var recommendations []string
	
	if qm.stats.TotalQueries == 0 {
		return recommendations
	}
	
	slowRatio := float64(qm.stats.SlowQueries) / float64(qm.stats.TotalQueries) * 100
	failureRate := float64(qm.stats.FailedQueries) / float64(qm.stats.TotalQueries) * 100
	
	if slowRatio > 10 {
		recommendations = append(recommendations, "High slow query ratio (>10%) - urgent optimization needed")
	} else if slowRatio > 5 {
		recommendations = append(recommendations, "Moderate slow query ratio (>5%) - consider query optimization")
	}
	
	if failureRate > 1 {
		recommendations = append(recommendations, "High query failure rate (>1%) - check for database connection issues")
	}
	
	if qm.stats.AverageQueryTime > 50*time.Millisecond {
		recommendations = append(recommendations, "High average query time - consider adding indexes or optimizing queries")
	}
	
	// Analyze patterns for specific recommendations
	patterns := qm.GetTopSlowQueries(5)
	for _, pattern := range patterns {
		if pattern.Count > 10 && pattern.AverageDuration > 200*time.Millisecond {
			recommendations = append(recommendations, 
				fmt.Sprintf("Pattern '%s' executed %d times with avg duration %v - optimize this query", 
					pattern.Pattern, pattern.Count, pattern.AverageDuration))
		}
	}
	
	return recommendations
}

// GORMLogWriter implements GORM's Writer interface for query logging
type GORMLogWriter struct {
	logger       *zap.Logger
	queryMonitor *QueryMonitor
}

// Printf implements the Writer interface
func (w *GORMLogWriter) Printf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	
	// Log based on content
	if strings.Contains(msg, "SLOW SQL") {
		w.logger.Warn("GORM slow query", zap.String("message", msg))
	} else if strings.Contains(msg, "ERROR") {
		w.logger.Error("GORM error", zap.String("message", msg))
	} else {
		w.logger.Debug("GORM log", zap.String("message", msg))
	}
}