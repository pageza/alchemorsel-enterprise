// Package cache provides performance tests and benchmarks for cache infrastructure
package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"go.uber.org/zap"
)

// BenchmarkCacheOperations benchmarks basic cache operations
func BenchmarkCacheOperations(b *testing.B) {
	cache, cleanup := setupBenchmarkCache(b)
	defer cleanup()

	ctx := context.Background()
	
	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("benchmark_key_%d", i)
			data := []byte(fmt.Sprintf("benchmark_data_%d", i))
			cache.Set(ctx, key, data, time.Hour)
		}
	})

	b.Run("Get", func(b *testing.B) {
		// Setup data
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("get_benchmark_key_%d", i)
			data := []byte(fmt.Sprintf("get_benchmark_data_%d", i))
			cache.Set(ctx, key, data, time.Hour)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("get_benchmark_key_%d", i%1000)
			cache.Get(ctx, key)
		}
	})

	b.Run("GetSet", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("getset_key_%d", i%100) // Reuse keys for realistic scenario
			
			// Try to get first
			cache.Get(ctx, key)
			
			// Set if not found (simulating cache-first pattern)
			data := []byte(fmt.Sprintf("getset_data_%d", i))
			cache.Set(ctx, key, data, time.Hour)
		}
	})
}

// BenchmarkConcurrentAccess benchmarks concurrent cache access
func BenchmarkConcurrentAccess(b *testing.B) {
	cache, cleanup := setupBenchmarkCache(b)
	defer cleanup()

	ctx := context.Background()
	
	// Setup initial data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("concurrent_key_%d", i)
		data := []byte(fmt.Sprintf("concurrent_data_%d", i))
		cache.Set(ctx, key, data, time.Hour)
	}

	b.Run("ConcurrentReads", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := fmt.Sprintf("concurrent_key_%d", i%1000)
				cache.Get(ctx, key)
				i++
			}
		})
	})

	b.Run("ConcurrentWrites", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := fmt.Sprintf("write_key_%d", i)
				data := []byte(fmt.Sprintf("write_data_%d", i))
				cache.Set(ctx, key, data, time.Hour)
				i++
			}
		})
	})

	b.Run("ConcurrentMixed", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if i%3 == 0 {
					// Write operation
					key := fmt.Sprintf("mixed_write_key_%d", i)
					data := []byte(fmt.Sprintf("mixed_write_data_%d", i))
					cache.Set(ctx, key, data, time.Hour)
				} else {
					// Read operation
					key := fmt.Sprintf("concurrent_key_%d", i%1000)
					cache.Get(ctx, key)
				}
				i++
			}
		})
	})
}

// BenchmarkCacheSize benchmarks cache operations with different data sizes
func BenchmarkCacheSize(b *testing.B) {
	cache, cleanup := setupBenchmarkCache(b)
	defer cleanup()

	ctx := context.Background()
	
	sizes := []int{
		100,      // 100B
		1024,     // 1KB
		10240,    // 10KB
		102400,   // 100KB
		1048576,  // 1MB
	}

	for _, size := range sizes {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i % 256)
		}

		b.Run(fmt.Sprintf("Set_%dB", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("size_key_%d_%d", size, i)
				cache.Set(ctx, key, data, time.Hour)
			}
			b.SetBytes(int64(size))
		})

		// Setup data for get benchmark
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("get_size_key_%d_%d", size, i)
			cache.Set(ctx, key, data, time.Hour)
		}

		b.Run(fmt.Sprintf("Get_%dB", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("get_size_key_%d_%d", size, i%100)
				cache.Get(ctx, key)
			}
			b.SetBytes(int64(size))
		})
	}
}

// BenchmarkSpecializedServices benchmarks specialized cache services
func BenchmarkSpecializedServices(b *testing.B) {
	container, cleanup := setupBenchmarkContainer(b)
	defer cleanup()

	ctx := context.Background()

	b.Run("RecipeCache", func(b *testing.B) {
		// This would benchmark recipe caching operations
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("recipe_%d", i)
			data := []byte(fmt.Sprintf(`{"id":"%s","title":"Recipe %d","ingredients":["ingredient1","ingredient2"]}`, key, i))
			container.CacheService.Set(ctx, key, data, time.Hour)
		}
	})

	b.Run("SessionCache", func(b *testing.B) {
		// This would benchmark session caching operations
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sessionID := fmt.Sprintf("session_%d", i)
			// Simulate session creation/retrieval
			container.SessionCache.GetSession(ctx, sessionID)
		}
	})

	b.Run("TemplateCache", func(b *testing.B) {
		// This would benchmark template caching operations
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			templateName := fmt.Sprintf("template_%d", i%10) // Reuse template names
			data := map[string]interface{}{
				"user_id": fmt.Sprintf("user_%d", i),
				"data":    fmt.Sprintf("template_data_%d", i),
			}
			
			key := container.TemplateCache.keyBuilder.BuildTemplateKey(templateName, data, nil, nil)
			container.CacheService.Get(ctx, key)
		}
	})
}

// TestCachePerformanceUnderLoad tests cache performance under sustained load
func TestCachePerformanceUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	cache, cleanup := setupBenchmarkCache(t)
	defer cleanup()

	ctx := context.Background()
	
	// Test parameters
	duration := 30 * time.Second
	numWorkers := 50
	targetOpsPerSec := 10000

	var (
		totalOps    int64
		totalErrors int64
		wg          sync.WaitGroup
		start       = time.Now()
		stop        = make(chan struct{})
	)

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			ops := 0
			errors := 0
			
			for {
				select {
				case <-stop:
					t.Logf("Worker %d: %d ops, %d errors", workerID, ops, errors)
					return
				default:
					// Perform cache operation
					key := fmt.Sprintf("load_test_key_%d_%d", workerID, ops)
					data := []byte(fmt.Sprintf("load_test_data_%d_%d", workerID, ops))
					
					if ops%3 == 0 {
						// Write operation
						if err := cache.Set(ctx, key, data, time.Hour); err != nil {
							errors++
						}
					} else {
						// Read operation
						if _, err := cache.Get(ctx, key); err != nil {
							// Cache miss is not an error in this context
							if err != ErrKeyNotFound {
								errors++
							}
						}
					}
					
					ops++
					
					// Small delay to control rate
					time.Sleep(time.Duration(1000000000/(targetOpsPerSec/numWorkers)) * time.Nanosecond)
				}
			}
		}(i)
	}

	// Stop after duration
	time.AfterFunc(duration, func() {
		close(stop)
	})

	// Wait for workers to finish
	wg.Wait()
	elapsed := time.Since(start)

	// Calculate final stats
	stats := cache.GetStats()
	
	t.Logf("Load test completed in %v", elapsed)
	t.Logf("Total operations: %d", stats.TotalOperations)
	t.Logf("Hit ratio: %.2f%%", stats.HitRatio*100)
	t.Logf("Average response time: %v", stats.AvgReadTime)
	t.Logf("Operations per second: %.2f", float64(stats.TotalOperations)/elapsed.Seconds())

	// Performance assertions
	if stats.HitRatio < 0.5 {
		t.Errorf("Hit ratio too low: %.2f%% (expected > 50%%)", stats.HitRatio*100)
	}

	if stats.AvgReadTime > 10*time.Millisecond {
		t.Errorf("Average response time too high: %v (expected < 10ms)", stats.AvgReadTime)
	}

	opsPerSec := float64(stats.TotalOperations) / elapsed.Seconds()
	if opsPerSec < float64(targetOpsPerSec)*0.8 {
		t.Errorf("Operations per second too low: %.2f (expected > %.2f)", opsPerSec, float64(targetOpsPerSec)*0.8)
	}
}

// TestMemoryUsage tests memory usage patterns
func TestMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	cache, cleanup := setupBenchmarkCache(t)
	defer cleanup()

	ctx := context.Background()
	
	// Test memory usage with large dataset
	dataSize := 1024 // 1KB per item
	numItems := 10000 // 10MB total
	
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte(i % 256)
	}

	start := time.Now()
	
	// Fill cache
	for i := 0; i < numItems; i++ {
		key := fmt.Sprintf("memory_test_key_%d", i)
		if err := cache.Set(ctx, key, data, time.Hour); err != nil {
			t.Fatalf("Failed to set key %s: %v", key, err)
		}
		
		// Log progress
		if i%1000 == 0 {
			t.Logf("Stored %d items", i)
		}
	}
	
	elapsed := time.Since(start)
	t.Logf("Stored %d items (%dMB) in %v", numItems, (numItems*dataSize)/(1024*1024), elapsed)

	// Test retrieval performance
	start = time.Now()
	hits := 0
	
	for i := 0; i < numItems; i++ {
		key := fmt.Sprintf("memory_test_key_%d", i)
		if _, err := cache.Get(ctx, key); err == nil {
			hits++
		}
	}
	
	elapsed = time.Since(start)
	hitRatio := float64(hits) / float64(numItems)
	
	t.Logf("Retrieved %d/%d items (%.2f%%) in %v", hits, numItems, hitRatio*100, elapsed)
	t.Logf("Average retrieval time: %v", elapsed/time.Duration(numItems))

	// Verify hit ratio
	if hitRatio < 0.99 {
		t.Errorf("Hit ratio too low: %.2f%% (expected > 99%%)", hitRatio*100)
	}
}

// Helper functions

func setupBenchmarkCache(tb testing.TB) (*CacheService, func()) {
	// Create test Redis config
	redisConfig := &config.RedisConfig{
		Host:         "localhost",
		Port:         6379,
		Database:     1, // Use test database
		MaxRetries:   3,
		PoolSize:     50,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	logger := zap.NewNop() // No-op logger for benchmarks

	// Create Redis client
	redisClient, err := NewRedisClient(redisConfig, logger)
	if err != nil {
		tb.Fatalf("Failed to create Redis client: %v", err)
	}

	// Create cache service
	cacheConfig := DefaultCacheConfig()
	cache := NewCacheService(redisClient, cacheConfig, logger)

	cleanup := func() {
		// Clean up test data
		ctx := context.Background()
		cache.InvalidateByPattern(ctx, "*benchmark*")
		cache.InvalidateByPattern(ctx, "*test*")
		redisClient.Close()
	}

	return cache, cleanup
}

func setupBenchmarkContainer(tb testing.TB) (*Container, func()) {
	// Create test config
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:         "localhost",
			Port:         6379,
			Database:     1, // Use test database
			MaxRetries:   3,
			PoolSize:     50,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
		Monitoring: config.MonitoringConfig{
			EnableMetrics: false, // Disable monitoring for benchmarks
		},
	}

	logger := zap.NewNop() // No-op logger for benchmarks

	// Create container
	container, err := NewContainer(cfg, logger)
	if err != nil {
		tb.Fatalf("Failed to create cache container: %v", err)
	}

	cleanup := func() {
		// Clean up test data
		container.InvalidateAll()
		container.Close()
	}

	return container, cleanup
}

// Benchmark results reporting
func BenchmarkResultSummary(b *testing.B) {
	// This function can be used to generate performance reports
	b.Logf("=== Cache Performance Benchmark Summary ===")
	b.Logf("Run 'go test -bench=. -benchmem' to see detailed results")
	b.Logf("Key metrics to monitor:")
	b.Logf("- ns/op: Lower is better (target: < 1ms for cache ops)")
	b.Logf("- allocs/op: Lower is better (target: < 10 allocs per op)")
	b.Logf("- MB/s: Higher is better for data transfer")
	b.Logf("- Hit ratio: Higher is better (target: > 95%%)")
}

// Performance test configuration
type PerformanceTestConfig struct {
	RedisHost          string
	RedisPort          int
	TestDuration       time.Duration
	ConcurrentWorkers  int
	TargetOpsPerSecond int
	DataSizeBytes      int
	NumTestItems       int
}

func DefaultPerformanceTestConfig() *PerformanceTestConfig {
	return &PerformanceTestConfig{
		RedisHost:          "localhost",
		RedisPort:          6379,
		TestDuration:       30 * time.Second,
		ConcurrentWorkers:  10,
		TargetOpsPerSecond: 5000,
		DataSizeBytes:      1024,
		NumTestItems:       10000,
	}
}