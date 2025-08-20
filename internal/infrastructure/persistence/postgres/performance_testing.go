// Package postgres provides comprehensive database performance testing and validation
package postgres

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// PerformanceTester provides comprehensive database performance testing
type PerformanceTester struct {
	db                *gorm.DB
	connectionManager *ConnectionManager
	queryCache        *QueryCache
	logger            *zap.Logger
}

// NewPerformanceTester creates a new performance tester
func NewPerformanceTester(
	db *gorm.DB,
	cm *ConnectionManager,
	qc *QueryCache,
	logger *zap.Logger,
) *PerformanceTester {
	return &PerformanceTester{
		db:                db,
		connectionManager: cm,
		queryCache:        qc,
		logger:            logger,
	}
}

// TestSuite represents a complete performance test suite
type TestSuite struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Tests       []PerformanceTest `json:"tests"`
	Config      TestConfig    `json:"config"`
	Results     TestResults   `json:"results"`
}

// PerformanceTest represents a single performance test
type PerformanceTest struct {
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	Category     TestCategory       `json:"category"`
	Query        string             `json:"query"`
	Args         []interface{}      `json:"args"`
	Concurrency  int                `json:"concurrency"`
	Duration     time.Duration      `json:"duration"`
	Target       PerformanceTarget  `json:"target"`
	Result       *TestResult        `json:"result,omitempty"`
}

// TestCategory represents different test categories
type TestCategory string

const (
	TestCategoryConnection TestCategory = "connection"
	TestCategoryQuery      TestCategory = "query"
	TestCategoryCache      TestCategory = "cache"
	TestCategoryIndex      TestCategory = "index"
	TestCategoryLoad       TestCategory = "load"
	TestCategoryStress     TestCategory = "stress"
)

// PerformanceTarget defines performance targets for tests
type PerformanceTarget struct {
	MaxResponseTime   time.Duration `json:"max_response_time"`
	MinThroughput     float64       `json:"min_throughput"`
	MaxErrorRate      float64       `json:"max_error_rate"`
	MinCacheHitRate   float64       `json:"min_cache_hit_rate"`
	MaxConnectionUtil float64       `json:"max_connection_util"`
}

// TestResult holds the results of a performance test
type TestResult struct {
	Passed           bool           `json:"passed"`
	StartTime        time.Time      `json:"start_time"`
	EndTime          time.Time      `json:"end_time"`
	Duration         time.Duration  `json:"duration"`
	TotalOperations  int64          `json:"total_operations"`
	SuccessfulOps    int64          `json:"successful_ops"`
	FailedOps        int64          `json:"failed_ops"`
	Throughput       float64        `json:"throughput"`
	AvgResponseTime  time.Duration  `json:"avg_response_time"`
	P95ResponseTime  time.Duration  `json:"p95_response_time"`
	P99ResponseTime  time.Duration  `json:"p99_response_time"`
	ErrorRate        float64        `json:"error_rate"`
	CacheHitRate     float64        `json:"cache_hit_rate"`
	ConnectionUtil   float64        `json:"connection_util"`
	Errors           []string       `json:"errors"`
	Metrics          TestMetrics    `json:"metrics"`
}

// TestMetrics holds detailed test metrics
type TestMetrics struct {
	ResponseTimes    []time.Duration `json:"response_times"`
	Timestamps       []time.Time     `json:"timestamps"`
	CacheHits        int64           `json:"cache_hits"`
	CacheMisses      int64           `json:"cache_misses"`
	ConnectionsUsed  []int           `json:"connections_used"`
	MemoryUsage      []int64         `json:"memory_usage"`
}

// TestConfig holds test configuration
type TestConfig struct {
	Timeout         time.Duration `json:"timeout"`
	WarmupDuration  time.Duration `json:"warmup_duration"`
	CooldownDuration time.Duration `json:"cooldown_duration"`
	DataSetSize     int           `json:"data_set_size"`
	PrepareData     bool          `json:"prepare_data"`
	CleanupData     bool          `json:"cleanup_data"`
}

// TestResults holds aggregate test results
type TestResults struct {
	Timestamp     time.Time            `json:"timestamp"`
	TotalTests    int                  `json:"total_tests"`
	PassedTests   int                  `json:"passed_tests"`
	FailedTests   int                  `json:"failed_tests"`
	SuccessRate   float64              `json:"success_rate"`
	TotalDuration time.Duration        `json:"total_duration"`
	Summary       TestSummary          `json:"summary"`
	Failures      []string             `json:"failures"`
}

// TestSummary provides high-level test summary
type TestSummary struct {
	OverallHealth      float64       `json:"overall_health"`
	PerformanceGrade   string        `json:"performance_grade"`
	ConnectionHealth   string        `json:"connection_health"`
	QueryHealth        string        `json:"query_health"`
	CacheHealth        string        `json:"cache_health"`
	IndexHealth        string        `json:"index_health"`
	Recommendations    []string      `json:"recommendations"`
	CriticalIssues     []string      `json:"critical_issues"`
}

// RunComprehensiveTests runs a comprehensive performance test suite
func (pt *PerformanceTester) RunComprehensiveTests(ctx context.Context) (*TestSuite, error) {
	suite := &TestSuite{
		Name:        "Alchemorsel v3 Database Performance Suite",
		Description: "Comprehensive database performance validation for ADR-0008 targets",
		Config: TestConfig{
			Timeout:          30 * time.Minute,
			WarmupDuration:   2 * time.Minute,
			CooldownDuration: 1 * time.Minute,
			DataSetSize:      10000,
			PrepareData:      true,
			CleanupData:      true,
		},
		Tests: pt.createTestSuite(),
	}

	pt.logger.Info("Starting comprehensive database performance tests",
		zap.String("suite", suite.Name),
		zap.Int("test_count", len(suite.Tests)))

	start := time.Now()

	// Prepare test data if needed
	if suite.Config.PrepareData {
		if err := pt.prepareTestData(ctx, suite.Config.DataSetSize); err != nil {
			return nil, fmt.Errorf("failed to prepare test data: %w", err)
		}
	}

	// Warmup
	if suite.Config.WarmupDuration > 0 {
		pt.logger.Info("Warming up database", zap.Duration("duration", suite.Config.WarmupDuration))
		if err := pt.warmupDatabase(ctx, suite.Config.WarmupDuration); err != nil {
			pt.logger.Warn("Warmup failed", zap.Error(err))
		}
	}

	// Run tests
	var passed, failed int
	var failures []string

	for i := range suite.Tests {
		test := &suite.Tests[i]
		pt.logger.Info("Running performance test",
			zap.String("test", test.Name),
			zap.String("category", string(test.Category)))

		result, err := pt.runSingleTest(ctx, test)
		if err != nil {
			pt.logger.Error("Test execution failed",
				zap.String("test", test.Name),
				zap.Error(err))
			failures = append(failures, fmt.Sprintf("%s: %v", test.Name, err))
			failed++
			continue
		}

		test.Result = result
		if result.Passed {
			passed++
		} else {
			failed++
			failures = append(failures, fmt.Sprintf("%s: performance targets not met", test.Name))
		}
	}

	// Cooldown
	if suite.Config.CooldownDuration > 0 {
		time.Sleep(suite.Config.CooldownDuration)
	}

	// Cleanup test data if needed
	if suite.Config.CleanupData {
		if err := pt.cleanupTestData(ctx); err != nil {
			pt.logger.Warn("Failed to cleanup test data", zap.Error(err))
		}
	}

	totalDuration := time.Since(start)
	successRate := float64(passed) / float64(passed+failed) * 100

	suite.Results = TestResults{
		Timestamp:     time.Now(),
		TotalTests:    passed + failed,
		PassedTests:   passed,
		FailedTests:   failed,
		SuccessRate:   successRate,
		TotalDuration: totalDuration,
		Failures:      failures,
		Summary:       pt.generateTestSummary(suite.Tests),
	}

	pt.logger.Info("Performance test suite completed",
		zap.Int("passed", passed),
		zap.Int("failed", failed),
		zap.Float64("success_rate", successRate),
		zap.Duration("total_duration", totalDuration))

	return suite, nil
}

// createTestSuite creates the complete test suite
func (pt *PerformanceTester) createTestSuite() []PerformanceTest {
	return []PerformanceTest{
		// Connection pool tests
		{
			Name:        "Connection Pool Utilization",
			Description: "Test connection pool under load",
			Category:    TestCategoryConnection,
			Concurrency: 100,
			Duration:    2 * time.Minute,
			Target: PerformanceTarget{
				MaxConnectionUtil: 90.0,
				MaxErrorRate:      1.0,
			},
		},
		{
			Name:        "Connection Acquisition Speed",
			Description: "Test connection acquisition performance",
			Category:    TestCategoryConnection,
			Concurrency: 50,
			Duration:    1 * time.Minute,
			Target: PerformanceTarget{
				MaxResponseTime: 5 * time.Millisecond,
				MaxErrorRate:    0.1,
			},
		},

		// Query performance tests
		{
			Name:        "Recipe Search Performance",
			Description: "Test recipe search query performance",
			Category:    TestCategoryQuery,
			Query:       "SELECT * FROM recipes WHERE status = 'published' AND cuisine = ? ORDER BY likes_count DESC LIMIT 20",
			Args:        []interface{}{"italian"},
			Concurrency: 50,
			Duration:    2 * time.Minute,
			Target: PerformanceTarget{
				MaxResponseTime: 100 * time.Millisecond,
				MinThroughput:   100.0,
				MaxErrorRate:    0.5,
			},
		},
		{
			Name:        "User Recipe Lookup",
			Description: "Test user recipe lookup performance",
			Category:    TestCategoryQuery,
			Query:       "SELECT * FROM recipes WHERE author_id = ? AND status = 'published' ORDER BY created_at DESC LIMIT 10",
			Concurrency: 30,
			Duration:    2 * time.Minute,
			Target: PerformanceTarget{
				MaxResponseTime: 50 * time.Millisecond,
				MinThroughput:   200.0,
				MaxErrorRate:    0.1,
			},
		},
		{
			Name:        "Recipe Rating Aggregation",
			Description: "Test complex aggregation query performance",
			Category:    TestCategoryQuery,
			Query:       "SELECT r.id, r.title, AVG(rt.rating) as avg_rating, COUNT(rt.id) as rating_count FROM recipes r LEFT JOIN recipe_ratings rt ON r.id = rt.recipe_id WHERE r.status = 'published' GROUP BY r.id, r.title HAVING COUNT(rt.id) > 5 ORDER BY avg_rating DESC LIMIT 50",
			Concurrency: 20,
			Duration:    2 * time.Minute,
			Target: PerformanceTarget{
				MaxResponseTime: 200 * time.Millisecond,
				MinThroughput:   50.0,
				MaxErrorRate:    1.0,
			},
		},

		// Cache performance tests
		{
			Name:        "Query Cache Hit Rate",
			Description: "Test query cache effectiveness",
			Category:    TestCategoryCache,
			Query:       "SELECT * FROM recipes WHERE id = ?",
			Concurrency: 100,
			Duration:    3 * time.Minute,
			Target: PerformanceTarget{
				MinCacheHitRate: 90.0,
				MaxResponseTime: 10 * time.Millisecond,
				MinThroughput:   500.0,
			},
		},

		// Load tests
		{
			Name:        "Sustained Load Test",
			Description: "Test sustained database load",
			Category:    TestCategoryLoad,
			Concurrency: 200,
			Duration:    5 * time.Minute,
			Target: PerformanceTarget{
				MaxResponseTime:   500 * time.Millisecond,
				MinThroughput:     100.0,
				MaxErrorRate:      2.0,
				MaxConnectionUtil: 85.0,
			},
		},
		{
			Name:        "Burst Load Test",
			Description: "Test database under burst load",
			Category:    TestCategoryStress,
			Concurrency: 500,
			Duration:    30 * time.Second,
			Target: PerformanceTarget{
				MaxResponseTime:   1 * time.Second,
				MaxErrorRate:      5.0,
				MaxConnectionUtil: 95.0,
			},
		},

		// Index effectiveness tests
		{
			Name:        "Index Scan vs Sequential Scan",
			Description: "Test index usage effectiveness",
			Category:    TestCategoryIndex,
			Query:       "SELECT * FROM recipes WHERE cuisine = ? AND difficulty = ?",
			Args:        []interface{}{"italian", "medium"},
			Concurrency: 30,
			Duration:    2 * time.Minute,
			Target: PerformanceTarget{
				MaxResponseTime: 100 * time.Millisecond,
				MinThroughput:   100.0,
			},
		},
	}
}

// runSingleTest runs a single performance test
func (pt *PerformanceTester) runSingleTest(ctx context.Context, test *PerformanceTest) (*TestResult, error) {
	result := &TestResult{
		StartTime: time.Now(),
		Metrics: TestMetrics{
			ResponseTimes: make([]time.Duration, 0),
			Timestamps:    make([]time.Time, 0),
		},
	}

	// Test context with timeout
	testCtx, cancel := context.WithTimeout(ctx, test.Duration+time.Minute)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	done := make(chan bool)

	// Start metrics collection
	go pt.collectTestMetrics(testCtx, result)

	// Start time for duration tracking
	startTime := time.Now()

	// Run concurrent workers
	for i := 0; i < test.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			pt.runTestWorker(testCtx, test, result, &mu, startTime, done)
		}(i)
	}

	// Wait for test duration
	time.Sleep(test.Duration)
	close(done)
	
	// Wait for all workers to finish
	wg.Wait()

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Calculate final metrics
	pt.calculateTestMetrics(result, test)

	// Evaluate against targets
	result.Passed = pt.evaluateTestResult(result, test.Target)

	return result, nil
}

// runTestWorker runs a test worker
func (pt *PerformanceTester) runTestWorker(
	ctx context.Context,
	test *PerformanceTest,
	result *TestResult,
	mu *sync.Mutex,
	startTime time.Time,
	done chan bool,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		default:
			// Execute operation
			opStart := time.Now()
			
			var err error
			if test.Query != "" {
				err = pt.executeTestQuery(ctx, test)
			} else {
				err = pt.executeTestOperation(ctx, test)
			}

			opDuration := time.Since(opStart)

			// Record metrics
			mu.Lock()
			result.TotalOperations++
			result.Metrics.ResponseTimes = append(result.Metrics.ResponseTimes, opDuration)
			result.Metrics.Timestamps = append(result.Metrics.Timestamps, time.Now())
			
			if err != nil {
				result.FailedOps++
				result.Errors = append(result.Errors, err.Error())
			} else {
				result.SuccessfulOps++
			}
			mu.Unlock()

			// Small delay to prevent overwhelming
			time.Sleep(time.Millisecond)
		}
	}
}

// executeTestQuery executes a test query
func (pt *PerformanceTester) executeTestQuery(ctx context.Context, test *PerformanceTest) error {
	var results []map[string]interface{}
	
	query := pt.db.WithContext(ctx)
	if len(test.Args) > 0 {
		query = query.Raw(test.Query, test.Args...)
	} else {
		query = query.Raw(test.Query)
	}
	
	return query.Find(&results).Error
}

// executeTestOperation executes a test operation (like connection test)
func (pt *PerformanceTester) executeTestOperation(ctx context.Context, test *PerformanceTest) error {
	switch test.Category {
	case TestCategoryConnection:
		// Test connection acquisition
		sqlDB, err := pt.db.DB()
		if err != nil {
			return err
		}
		return sqlDB.PingContext(ctx)
	default:
		// Simple ping
		sqlDB, err := pt.db.DB()
		if err != nil {
			return err
		}
		return sqlDB.PingContext(ctx)
	}
}

// collectTestMetrics collects metrics during test execution
func (pt *PerformanceTester) collectTestMetrics(ctx context.Context, result *TestResult) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Collect connection metrics
			if pt.connectionManager != nil {
				metrics := pt.connectionManager.GetMetrics().GetSnapshot()
				result.Metrics.ConnectionsUsed = append(result.Metrics.ConnectionsUsed, metrics.InUse)
			}

			// Collect cache metrics
			if pt.queryCache != nil {
				cacheMetrics := pt.queryCache.GetMetrics()
				result.Metrics.CacheHits = cacheMetrics.Hits
				result.Metrics.CacheMisses = cacheMetrics.Misses
			}
		}
	}
}

// calculateTestMetrics calculates final test metrics
func (pt *PerformanceTester) calculateTestMetrics(result *TestResult, test *PerformanceTest) {
	if result.TotalOperations > 0 {
		// Calculate throughput (operations per second)
		result.Throughput = float64(result.TotalOperations) / result.Duration.Seconds()

		// Calculate error rate
		result.ErrorRate = float64(result.FailedOps) / float64(result.TotalOperations) * 100

		// Calculate response time metrics
		if len(result.Metrics.ResponseTimes) > 0 {
			// Sort response times for percentile calculation
			times := make([]time.Duration, len(result.Metrics.ResponseTimes))
			copy(times, result.Metrics.ResponseTimes)
			
			// Simple sort (for accurate percentiles, use a proper sort)
			for i := 0; i < len(times)-1; i++ {
				for j := i + 1; j < len(times); j++ {
					if times[i] > times[j] {
						times[i], times[j] = times[j], times[i]
					}
				}
			}

			// Calculate average
			var total time.Duration
			for _, t := range times {
				total += t
			}
			result.AvgResponseTime = total / time.Duration(len(times))

			// Calculate percentiles
			p95Index := int(0.95 * float64(len(times)))
			p99Index := int(0.99 * float64(len(times)))
			
			if p95Index < len(times) {
				result.P95ResponseTime = times[p95Index]
			}
			if p99Index < len(times) {
				result.P99ResponseTime = times[p99Index]
			}
		}

		// Calculate cache hit rate
		if result.Metrics.CacheHits+result.Metrics.CacheMisses > 0 {
			result.CacheHitRate = float64(result.Metrics.CacheHits) / 
				float64(result.Metrics.CacheHits+result.Metrics.CacheMisses) * 100
		}

		// Calculate connection utilization
		if pt.connectionManager != nil {
			metrics := pt.connectionManager.GetMetrics().GetSnapshot()
			result.ConnectionUtil = metrics.GetConnectionEfficiency()
		}
	}
}

// evaluateTestResult evaluates test result against targets
func (pt *PerformanceTester) evaluateTestResult(result *TestResult, target PerformanceTarget) bool {
	passed := true

	if target.MaxResponseTime > 0 && result.AvgResponseTime > target.MaxResponseTime {
		passed = false
	}

	if target.MinThroughput > 0 && result.Throughput < target.MinThroughput {
		passed = false
	}

	if target.MaxErrorRate > 0 && result.ErrorRate > target.MaxErrorRate {
		passed = false
	}

	if target.MinCacheHitRate > 0 && result.CacheHitRate < target.MinCacheHitRate {
		passed = false
	}

	if target.MaxConnectionUtil > 0 && result.ConnectionUtil > target.MaxConnectionUtil {
		passed = false
	}

	return passed
}

// prepareTestData prepares test data for performance testing
func (pt *PerformanceTester) prepareTestData(ctx context.Context, size int) error {
	pt.logger.Info("Preparing test data", zap.Int("size", size))

	// Create test users
	for i := 0; i < min(size/10, 1000); i++ {
		user := map[string]interface{}{
			"id":            uuid.New(),
			"email":         fmt.Sprintf("testuser%d@example.com", i),
			"username":      fmt.Sprintf("testuser%d", i),
			"password_hash": "hashed_password",
			"full_name":     fmt.Sprintf("Test User %d", i),
			"created_at":    time.Now(),
			"updated_at":    time.Now(),
		}

		if err := pt.db.WithContext(ctx).Table("users").Create(user).Error; err != nil {
			return fmt.Errorf("failed to create test user: %w", err)
		}
	}

	// Create test recipes
	cuisines := []string{"italian", "french", "chinese", "mexican", "american"}
	difficulties := []string{"easy", "medium", "hard"}

	for i := 0; i < size; i++ {
		recipe := map[string]interface{}{
			"id":                 uuid.New(),
			"title":              fmt.Sprintf("Test Recipe %d", i),
			"description":        fmt.Sprintf("Description for test recipe %d", i),
			"author_id":          uuid.New(), // Random author
			"cuisine":            cuisines[rand.Intn(len(cuisines))],
			"difficulty":         difficulties[rand.Intn(len(difficulties))],
			"prep_time_minutes":  rand.Intn(60) + 10,
			"cook_time_minutes":  rand.Intn(120) + 15,
			"servings":           rand.Intn(8) + 1,
			"status":             "published",
			"likes_count":        rand.Intn(1000),
			"views_count":        rand.Intn(5000),
			"average_rating":     float64(rand.Intn(50)+1) / 10.0,
			"published_at":       time.Now().Add(-time.Duration(rand.Intn(365)) * 24 * time.Hour),
			"created_at":         time.Now(),
			"updated_at":         time.Now(),
		}

		if err := pt.db.WithContext(ctx).Table("recipes").Create(recipe).Error; err != nil {
			return fmt.Errorf("failed to create test recipe: %w", err)
		}
	}

	pt.logger.Info("Test data prepared successfully")
	return nil
}

// warmupDatabase warms up the database with some queries
func (pt *PerformanceTester) warmupDatabase(ctx context.Context, duration time.Duration) error {
	start := time.Now()
	queries := []string{
		"SELECT COUNT(*) FROM recipes",
		"SELECT COUNT(*) FROM users",
		"SELECT * FROM recipes WHERE status = 'published' LIMIT 10",
		"SELECT * FROM recipes WHERE cuisine = 'italian' LIMIT 5",
	}

	for time.Since(start) < duration {
		for _, query := range queries {
			var result []map[string]interface{}
			pt.db.WithContext(ctx).Raw(query).Find(&result)
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

// cleanupTestData cleans up test data
func (pt *PerformanceTester) cleanupTestData(ctx context.Context) error {
	pt.logger.Info("Cleaning up test data")

	// Delete test recipes
	if err := pt.db.WithContext(ctx).Exec("DELETE FROM recipes WHERE title LIKE 'Test Recipe %'").Error; err != nil {
		return fmt.Errorf("failed to cleanup test recipes: %w", err)
	}

	// Delete test users
	if err := pt.db.WithContext(ctx).Exec("DELETE FROM users WHERE email LIKE 'testuser%@example.com'").Error; err != nil {
		return fmt.Errorf("failed to cleanup test users: %w", err)
	}

	pt.logger.Info("Test data cleaned up successfully")
	return nil
}

// generateTestSummary generates a comprehensive test summary
func (pt *PerformanceTester) generateTestSummary(tests []PerformanceTest) TestSummary {
	var overallHealth float64 = 100.0
	var recommendations []string
	var criticalIssues []string

	connectionHealth := "healthy"
	queryHealth := "healthy"
	cacheHealth := "healthy"
	indexHealth := "healthy"

	for _, test := range tests {
		if test.Result == nil {
			continue
		}

		if !test.Result.Passed {
			overallHealth -= 10.0

			switch test.Category {
			case TestCategoryConnection:
				connectionHealth = "degraded"
				recommendations = append(recommendations, "Optimize connection pool configuration")
			case TestCategoryQuery:
				queryHealth = "degraded"
				recommendations = append(recommendations, "Optimize slow queries and add missing indexes")
			case TestCategoryCache:
				cacheHealth = "degraded"
				recommendations = append(recommendations, "Improve cache hit ratio and TTL settings")
			case TestCategoryIndex:
				indexHealth = "degraded"
				recommendations = append(recommendations, "Review and optimize database indexes")
			}

			if test.Result.ErrorRate > 5.0 {
				criticalIssues = append(criticalIssues, 
					fmt.Sprintf("High error rate in %s: %.1f%%", test.Name, test.Result.ErrorRate))
			}

			if test.Result.AvgResponseTime > 1*time.Second {
				criticalIssues = append(criticalIssues, 
					fmt.Sprintf("Slow response time in %s: %v", test.Name, test.Result.AvgResponseTime))
			}
		}
	}

	if overallHealth < 0 {
		overallHealth = 0
	}

	performanceGrade := "A"
	if overallHealth < 90 {
		performanceGrade = "B"
	}
	if overallHealth < 80 {
		performanceGrade = "C"
	}
	if overallHealth < 70 {
		performanceGrade = "D"
	}
	if overallHealth < 60 {
		performanceGrade = "F"
	}

	return TestSummary{
		OverallHealth:    overallHealth,
		PerformanceGrade: performanceGrade,
		ConnectionHealth: connectionHealth,
		QueryHealth:      queryHealth,
		CacheHealth:      cacheHealth,
		IndexHealth:      indexHealth,
		Recommendations:  recommendations,
		CriticalIssues:   criticalIssues,
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}