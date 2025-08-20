// Package healthcheck dependency management tests
// Tests for dependency graph validation, topological sorting, and dependency health checks
package healthcheck

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewDependencyManager(t *testing.T) {
	logger := zap.NewNop()
	dm := NewDependencyManager(logger)

	assert.NotNil(t, dm)
	assert.Equal(t, logger, dm.logger)
	assert.NotNil(t, dm.dependencies)
	assert.NotNil(t, dm.graph)
}

func TestNewDependencyGraph(t *testing.T) {
	dg := NewDependencyGraph()

	assert.NotNil(t, dg)
	assert.NotNil(t, dg.nodes)
}

func TestDependencyManager_Register(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	checker := NewMockChecker("db").WithStatus(StatusHealthy)
	dep := CreateTestDependency("database", DependencyTypeDatabase, true, []string{}, checker)

	dm.Register(dep)

	deps := dm.GetDependencies()
	assert.Len(t, deps, 1)
	assert.Contains(t, deps, "database")
	assert.Equal(t, dep, deps["database"])
}

func TestDependencyManager_RegisterWithDependencies(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	
	// Register database first
	dbChecker := NewMockChecker("db").WithStatus(StatusHealthy)
	dbDep := CreateTestDependency("database", DependencyTypeDatabase, true, []string{}, dbChecker)
	dm.Register(dbDep)

	// Register cache that depends on database
	cacheChecker := NewMockChecker("cache").WithStatus(StatusHealthy)
	cacheDep := CreateTestDependency("cache", DependencyTypeCache, false, []string{"database"}, cacheChecker)
	dm.Register(cacheDep)

	deps := dm.GetDependencies()
	assert.Len(t, deps, 2)
	assert.Contains(t, deps, "database")
	assert.Contains(t, deps, "cache")
}

func TestDependencyManager_Unregister(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	checker := NewMockChecker("db").WithStatus(StatusHealthy)
	dep := CreateTestDependency("database", DependencyTypeDatabase, true, []string{}, checker)

	dm.Register(dep)
	assert.Len(t, dm.GetDependencies(), 1)

	dm.Unregister("database")
	assert.Len(t, dm.GetDependencies(), 0)
}

func TestDependencyManager_CheckAll_NoDependencies(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	ctx := context.Background()

	results := dm.CheckAll(ctx)

	assert.Empty(t, results)
}

func TestDependencyManager_CheckAll_SingleDependency(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	ctx := context.Background()
	
	checker := NewMockChecker("db").WithStatus(StatusHealthy).WithMessage("Connected")
	dep := CreateTestDependency("database", DependencyTypeDatabase, true, []string{}, checker)
	dm.Register(dep)

	results := dm.CheckAll(ctx)

	require.Len(t, results, 1)
	result := results[0]
	assert.Equal(t, "database", result.Name)
	assert.Equal(t, DependencyTypeDatabase, result.Type)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Equal(t, "Connected", result.Message)
	assert.True(t, result.Critical)
	assert.Empty(t, result.Dependencies)
}

func TestDependencyManager_CheckAll_MultipleDependencies(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	ctx := context.Background()
	
	// Register dependencies
	dbChecker := NewMockChecker("db").WithStatus(StatusHealthy)
	dbDep := CreateTestDependency("database", DependencyTypeDatabase, true, []string{}, dbChecker)
	dm.Register(dbDep)

	cacheChecker := NewMockChecker("cache").WithStatus(StatusHealthy)
	cacheDep := CreateTestDependency("cache", DependencyTypeCache, false, []string{}, cacheChecker)
	dm.Register(cacheDep)

	results := dm.CheckAll(ctx)

	assert.Len(t, results, 2)
	
	// Results should contain both dependencies
	names := make(map[string]bool)
	for _, result := range results {
		names[result.Name] = true
	}
	assert.True(t, names["database"])
	assert.True(t, names["cache"])
}

func TestDependencyManager_CheckAll_TopologicalOrder(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	ctx := context.Background()
	
	// Create dependency chain: database -> cache -> api
	dbChecker := NewMockChecker("db").WithStatus(StatusHealthy)
	dbDep := CreateTestDependency("database", DependencyTypeDatabase, true, []string{}, dbChecker)
	dm.Register(dbDep)

	cacheChecker := NewMockChecker("cache").WithStatus(StatusHealthy)
	cacheDep := CreateTestDependency("cache", DependencyTypeCache, false, []string{"database"}, cacheChecker)
	dm.Register(cacheDep)

	apiChecker := NewMockChecker("api").WithStatus(StatusHealthy)
	apiDep := CreateTestDependency("external_api", DependencyTypeExternalAPI, false, []string{"cache"}, apiChecker)
	dm.Register(apiDep)

	results := dm.CheckAll(ctx)

	require.Len(t, results, 3)
	
	// Verify topological order: database should come before cache, cache before api
	AssertDependencyOrder(t, results, []string{"database", "cache", "external_api"})
}

func TestDependencyManager_CheckAll_DependencyFailureImpact(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	ctx := context.Background()
	
	// Database is unhealthy and critical
	dbChecker := NewMockChecker("db").WithStatus(StatusUnhealthy).WithMessage("Connection failed")
	dbDep := CreateTestDependency("database", DependencyTypeDatabase, true, []string{}, dbChecker)
	dm.Register(dbDep)

	// Cache depends on database
	cacheChecker := NewMockChecker("cache").WithStatus(StatusHealthy)
	cacheDep := CreateTestDependency("cache", DependencyTypeCache, false, []string{"database"}, cacheChecker)
	dm.Register(cacheDep)

	results := dm.CheckAll(ctx)

	require.Len(t, results, 2)
	
	// Database should be unhealthy
	dbResult := findDependencyByName(results, "database")
	require.NotNil(t, dbResult)
	assert.Equal(t, StatusUnhealthy, dbResult.Status)
	assert.Equal(t, "Connection failed", dbResult.Message)

	// Cache should be degraded due to database failure
	cacheResult := findDependencyByName(results, "cache")
	require.NotNil(t, cacheResult)
	assert.Equal(t, StatusDegraded, cacheResult.Status)
	assert.Contains(t, cacheResult.Message, "Dependency 'database' is unhealthy")
}

func TestDependencyManager_CheckAll_NonCriticalDependencyFailure(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	ctx := context.Background()
	
	// Cache is unhealthy but not critical
	cacheChecker := NewMockChecker("cache").WithStatus(StatusUnhealthy)
	cacheDep := CreateTestDependency("cache", DependencyTypeCache, false, []string{}, cacheChecker)
	dm.Register(cacheDep)

	// Service depends on cache
	serviceChecker := NewMockChecker("service").WithStatus(StatusHealthy)
	serviceDep := CreateTestDependency("external_service", DependencyTypeService, false, []string{"cache"}, serviceChecker)
	dm.Register(serviceDep)

	results := dm.CheckAll(ctx)

	require.Len(t, results, 2)
	
	// Cache should be unhealthy
	cacheResult := findDependencyByName(results, "cache")
	require.NotNil(t, cacheResult)
	assert.Equal(t, StatusUnhealthy, cacheResult.Status)

	// Service should remain healthy since cache is not critical
	serviceResult := findDependencyByName(results, "external_service")
	require.NotNil(t, serviceResult)
	assert.Equal(t, StatusHealthy, serviceResult.Status)
}

func TestDependencyManager_CheckDependency(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	ctx := context.Background()
	
	checker := NewMockChecker("db").WithStatus(StatusHealthy).WithMessage("Connected")
	dep := CreateTestDependency("database", DependencyTypeDatabase, true, []string{}, checker)
	dm.Register(dep)

	result, err := dm.CheckDependency(ctx, "database")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "database", result.Name)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Equal(t, "Connected", result.Message)
}

func TestDependencyManager_CheckDependency_NotFound(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	ctx := context.Background()

	result, err := dm.CheckDependency(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "dependency 'nonexistent' not found")
}

func TestDependencyManager_GetCriticalDependencies(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	
	// Register critical and non-critical dependencies
	dbChecker := NewMockChecker("db").WithStatus(StatusHealthy)
	dbDep := CreateTestDependency("database", DependencyTypeDatabase, true, []string{}, dbChecker)
	dm.Register(dbDep)

	cacheChecker := NewMockChecker("cache").WithStatus(StatusHealthy)
	cacheDep := CreateTestDependency("cache", DependencyTypeCache, false, []string{}, cacheChecker)
	dm.Register(cacheDep)

	queueChecker := NewMockChecker("queue").WithStatus(StatusHealthy)
	queueDep := CreateTestDependency("message_queue", DependencyTypeQueue, true, []string{}, queueChecker)
	dm.Register(queueDep)

	criticalDeps := dm.GetCriticalDependencies()

	assert.Len(t, criticalDeps, 2)
	assert.Contains(t, criticalDeps, "database")
	assert.Contains(t, criticalDeps, "message_queue")
	assert.NotContains(t, criticalDeps, "cache")
}

func TestDependencyManager_ValidateGraph_NoCycles(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	
	// Create valid dependency chain
	dbChecker := NewMockChecker("db").WithStatus(StatusHealthy)
	dbDep := CreateTestDependency("database", DependencyTypeDatabase, true, []string{}, dbChecker)
	dm.Register(dbDep)

	cacheChecker := NewMockChecker("cache").WithStatus(StatusHealthy)
	cacheDep := CreateTestDependency("cache", DependencyTypeCache, false, []string{"database"}, cacheChecker)
	dm.Register(cacheDep)

	err := dm.ValidateGraph()

	assert.NoError(t, err)
}

func TestDependencyManager_ValidateGraph_WithCycles(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	
	// Create circular dependency: A -> B -> C -> A
	checkerA := NewMockChecker("a").WithStatus(StatusHealthy)
	depA := CreateTestDependency("service_a", DependencyTypeService, false, []string{"service_c"}, checkerA)
	dm.Register(depA)

	checkerB := NewMockChecker("b").WithStatus(StatusHealthy)
	depB := CreateTestDependency("service_b", DependencyTypeService, false, []string{"service_a"}, checkerB)
	dm.Register(depB)

	checkerC := NewMockChecker("c").WithStatus(StatusHealthy)
	depC := CreateTestDependency("service_c", DependencyTypeService, false, []string{"service_b"}, checkerC)
	dm.Register(depC)

	err := dm.ValidateGraph()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency detected")
}

func TestDependencyGraph_AddNode(t *testing.T) {
	dg := NewDependencyGraph()

	dg.AddNode("database", []string{})

	assert.Len(t, dg.nodes, 1)
	assert.Contains(t, dg.nodes, "database")
	
	node := dg.nodes["database"]
	assert.Equal(t, "database", node.Name)
	assert.Empty(t, node.Dependencies)
	assert.Empty(t, node.Dependents)
}

func TestDependencyGraph_AddNode_WithDependencies(t *testing.T) {
	dg := NewDependencyGraph()

	dg.AddNode("cache", []string{"database"})

	assert.Len(t, dg.nodes, 2) // cache and placeholder database
	
	cacheNode := dg.nodes["cache"]
	assert.Equal(t, "cache", cacheNode.Name)
	assert.Equal(t, []string{"database"}, cacheNode.Dependencies)
	assert.Empty(t, cacheNode.Dependents)

	dbNode := dg.nodes["database"]
	assert.Equal(t, "database", dbNode.Name)
	assert.Empty(t, dbNode.Dependencies)
	assert.Equal(t, []string{"cache"}, dbNode.Dependents)
}

func TestDependencyGraph_RemoveNode(t *testing.T) {
	dg := NewDependencyGraph()

	// Add nodes with dependencies
	dg.AddNode("database", []string{})
	dg.AddNode("cache", []string{"database"})

	assert.Len(t, dg.nodes, 2)

	// Remove cache node
	dg.RemoveNode("cache")

	assert.Len(t, dg.nodes, 1)
	assert.Contains(t, dg.nodes, "database")
	assert.NotContains(t, dg.nodes, "cache")

	// Database should no longer have cache as dependent
	dbNode := dg.nodes["database"]
	assert.Empty(t, dbNode.Dependents)
}

func TestDependencyGraph_TopologicalSort_SimpleChain(t *testing.T) {
	dg := NewDependencyGraph()

	// Create chain: database -> cache -> api
	dg.AddNode("database", []string{})
	dg.AddNode("cache", []string{"database"})
	dg.AddNode("api", []string{"cache"})

	order := dg.TopologicalSort()

	assert.Equal(t, []string{"database", "cache", "api"}, order)
}

func TestDependencyGraph_TopologicalSort_ComplexGraph(t *testing.T) {
	dg := NewDependencyGraph()

	// Create more complex dependency graph
	dg.AddNode("database", []string{})
	dg.AddNode("cache", []string{"database"})
	dg.AddNode("queue", []string{"database"})
	dg.AddNode("api", []string{"cache", "queue"})
	dg.AddNode("worker", []string{"queue"})

	order := dg.TopologicalSort()

	require.Len(t, order, 5)
	
	// Verify constraints
	dbIdx := findIndex(order, "database")
	cacheIdx := findIndex(order, "cache")
	queueIdx := findIndex(order, "queue")
	apiIdx := findIndex(order, "api")
	workerIdx := findIndex(order, "worker")

	// Database should come before cache and queue
	assert.Less(t, dbIdx, cacheIdx)
	assert.Less(t, dbIdx, queueIdx)
	
	// Cache and queue should come before api
	assert.Less(t, cacheIdx, apiIdx)
	assert.Less(t, queueIdx, apiIdx)
	
	// Queue should come before worker
	assert.Less(t, queueIdx, workerIdx)
}

func TestDependencyGraph_TopologicalSort_WithCycle(t *testing.T) {
	dg := NewDependencyGraph()

	// Create circular dependency
	dg.AddNode("a", []string{"c"})
	dg.AddNode("b", []string{"a"})
	dg.AddNode("c", []string{"b"})

	order := dg.TopologicalSort()

	// Should return empty slice for cyclic graph
	assert.Empty(t, order)
}

func TestDependencyGraph_ValidateCycles(t *testing.T) {
	dg := NewDependencyGraph()

	// Create valid graph
	dg.AddNode("database", []string{})
	dg.AddNode("cache", []string{"database"})

	err := dg.ValidateCycles()
	assert.NoError(t, err)

	// Add circular dependency
	dg.AddNode("api", []string{"cache"})
	dg.AddNode("service", []string{"api", "database"})
	
	// Still valid
	err = dg.ValidateCycles()
	assert.NoError(t, err)

	// Now create cycle
	dg.RemoveNode("database")
	dg.AddNode("database", []string{"service"})

	err = dg.ValidateCycles()
	assert.Error(t, err)
}

func TestBasicDependency_Interface(t *testing.T) {
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	dep := NewBasicDependency("test_dep", DependencyTypeDatabase, true, []string{"other"}, checker)

	assert.Equal(t, "test_dep", dep.GetName())
	assert.Equal(t, DependencyTypeDatabase, dep.GetType())
	assert.True(t, dep.IsCritical())
	assert.Equal(t, []string{"other"}, dep.GetDependencies())
}

func TestBasicDependency_Check(t *testing.T) {
	ctx := context.Background()
	metadata := map[string]interface{}{"version": "1.0"}
	
	checker := NewMockChecker("test").
		WithStatus(StatusHealthy).
		WithMessage("OK").
		WithMetadata(metadata)
	
	dep := NewBasicDependency("test_dep", DependencyTypeDatabase, true, []string{}, checker)

	result := dep.Check(ctx)

	assert.Equal(t, "test_dep", result.Name)
	assert.Equal(t, DependencyTypeDatabase, result.Type)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Equal(t, "OK", result.Message)
	assert.True(t, result.Critical)
	assert.Equal(t, metadata, result.Metadata)
	assert.Empty(t, result.Dependencies)
}

func TestDependencyConstructors(t *testing.T) {
	checker := NewMockChecker("test").WithStatus(StatusHealthy)

	// Test DatabaseDependency
	dbDep := DatabaseDependency("postgres", true, checker)
	assert.Equal(t, "postgres", dbDep.GetName())
	assert.Equal(t, DependencyTypeDatabase, dbDep.GetType())
	assert.True(t, dbDep.IsCritical())
	assert.Empty(t, dbDep.GetDependencies())

	// Test CacheDependency
	cacheDep := CacheDependency("redis", false, checker)
	assert.Equal(t, "redis", cacheDep.GetName())
	assert.Equal(t, DependencyTypeCache, cacheDep.GetType())
	assert.False(t, cacheDep.IsCritical())

	// Test ExternalAPIDependency
	apiDep := ExternalAPIDependency("payment_service", true, []string{"database"}, checker)
	assert.Equal(t, "payment_service", apiDep.GetName())
	assert.Equal(t, DependencyTypeExternalAPI, apiDep.GetType())
	assert.True(t, apiDep.IsCritical())
	assert.Equal(t, []string{"database"}, apiDep.GetDependencies())

	// Test ServiceDependency
	serviceDep := ServiceDependency("user_service", false, []string{"database", "cache"}, checker)
	assert.Equal(t, "user_service", serviceDep.GetName())
	assert.Equal(t, DependencyTypeService, serviceDep.GetType())
	assert.False(t, serviceDep.IsCritical())
	assert.Equal(t, []string{"database", "cache"}, serviceDep.GetDependencies())
}

// Test concurrent access to dependency manager
func TestDependencyManager_ConcurrentAccess(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	ctx := context.Background()

	// Register initial dependencies
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("dep_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy)
		dep := CreateTestDependency(name, DependencyTypeService, false, []string{}, checker)
		dm.Register(dep)
	}

	// Run concurrent operations
	numGoroutines := 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			if id%3 == 0 {
				// Check all dependencies
				dm.CheckAll(ctx)
			} else if id%3 == 1 {
				// Get dependencies
				dm.GetDependencies()
			} else {
				// Check individual dependency
				dm.CheckDependency(ctx, "dep_0")
			}
		}(i)
	}

	wg.Wait()

	// Verify state is still consistent
	deps := dm.GetDependencies()
	assert.Len(t, deps, 5)
}

// Benchmark tests
func BenchmarkDependencyManager_CheckAll_SingleDependency(b *testing.B) {
	dm := NewDependencyManager(zap.NewNop())
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	dep := CreateTestDependency("database", DependencyTypeDatabase, true, []string{}, checker)
	dm.Register(dep)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dm.CheckAll(ctx)
	}
}

func BenchmarkDependencyManager_CheckAll_MultipleDependencies(b *testing.B) {
	dm := NewDependencyManager(zap.NewNop())

	// Register 10 dependencies
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("dep_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy)
		dep := CreateTestDependency(name, DependencyTypeService, false, []string{}, checker)
		dm.Register(dep)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dm.CheckAll(ctx)
	}
}

func BenchmarkDependencyGraph_TopologicalSort(b *testing.B) {
	dg := NewDependencyGraph()

	// Create complex dependency graph
	dg.AddNode("database", []string{})
	dg.AddNode("cache", []string{"database"})
	dg.AddNode("queue", []string{"database"})
	dg.AddNode("api", []string{"cache", "queue"})
	dg.AddNode("worker", []string{"queue"})
	dg.AddNode("scheduler", []string{"database", "queue"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dg.TopologicalSort()
	}
}

// Helper functions
func findDependencyByName(dependencies []DependencyStatus, name string) *DependencyStatus {
	for _, dep := range dependencies {
		if dep.Name == name {
			return &dep
		}
	}
	return nil
}

func findIndex(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}