# Alchemorsel v3 Cache Infrastructure

This package provides a comprehensive cache-first architecture implementation for Alchemorsel v3, designed to optimize performance and achieve the 14KB first packet goal outlined in ADR-0006.

## Architecture Overview

The cache infrastructure follows ADR-0007 (Redis Caching Strategy) and implements a multi-layer caching approach:

```
┌─────────────────────────────────────────────────────────┐
│                   Application Layer                    │
├─────────────────────────────────────────────────────────┤
│                  Cache Services                        │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐       │
│  │   Recipe    │ │   Session   │ │   Template  │       │
│  │   Cache     │ │   Cache     │ │   Cache     │  ...  │
│  └─────────────┘ └─────────────┘ └─────────────┘       │
├─────────────────────────────────────────────────────────┤
│                Core Cache Service                      │
│  ┌─────────────────────────────────────────────────────┐ │
│  │             Cache-First Pattern                    │ │
│  │  L1 (Local Memory) → L2 (Redis) → Source          │ │
│  └─────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────┤
│                Redis Infrastructure                    │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐       │
│  │   Client    │ │   Health    │ │   Metrics   │       │
│  │   Manager   │ │   Monitor   │ │   Collector │       │
│  └─────────────┘ └─────────────┘ └─────────────┘       │
└─────────────────────────────────────────────────────────┘
```

## Key Components

### 1. Core Infrastructure

- **RedisClient**: Redis connection management with circuit breaker, health monitoring, and performance metrics
- **CacheService**: Multi-layer cache service implementing cache-first pattern with L1 (local) and L2 (Redis) caching
- **LocalCache**: In-memory LRU cache for fastest access to frequently used data

### 2. Specialized Services

- **RecipeCacheService**: Recipe-specific caching with search optimization
- **SessionCacheService**: User session and preference caching with security features
- **AICacheService**: AI response caching for cost optimization and performance
- **TemplateCacheService**: HTMX template caching optimized for 14KB first packet delivery

### 3. HTTP Integration

- **HTTPCacheMiddleware**: HTTP response caching middleware with 14KB optimization
- Supports HTMX-specific caching patterns
- Implements ETags, conditional requests, and cache control headers

### 4. Monitoring & Metrics

- **CacheMonitor**: Comprehensive monitoring with alerting
- Performance metrics collection and export
- Health status reporting and circuit breaker integration

## Performance Targets (ADR-0007)

| Metric | Target | Implementation |
|--------|--------|----------------|
| Cache Hit Ratio | 95%+ | Multi-layer caching with intelligent TTL |
| Cached Response Time | <50ms | Local memory L1 cache |
| Cache Miss Response | <200ms | Optimized Redis operations |
| Concurrent Users | 1000+ | Connection pooling and async operations |
| First Packet Size | 14KB | Template optimization and critical CSS |

## Usage Examples

### Basic Cache Operations

```go
// Initialize cache container
container, err := cache.NewContainer(config, logger)
if err != nil {
    log.Fatal("Failed to initialize cache:", err)
}
defer container.Close()

// Use cache service directly
ctx := context.Background()
data := []byte("cached data")
err = container.CacheService.Set(ctx, "my-key", data, time.Hour)

// Retrieve from cache
cachedData, err := container.CacheService.Get(ctx, "my-key")
```

### Recipe Caching

```go
// Cache a recipe with automatic tagging
recipe := &recipe.Recipe{
    ID: uuid.New(),
    Title: "Chocolate Cake",
    // ... other fields
}

err := container.RecipeCache.CacheRecipe(ctx, recipe)

// Retrieve recipe with fallback to database
recipe, err := container.RecipeCache.GetRecipe(ctx, recipeID, func(ctx context.Context, id uuid.UUID) (*recipe.Recipe, error) {
    return repository.FindByID(ctx, id)
})
```

### Session Management

```go
// Create a user session
session, err := container.SessionCache.CreateSession(ctx, userID, sessionID, deviceInfo, 24*time.Hour)

// Retrieve session
session, err := container.SessionCache.GetSession(ctx, sessionID)

// Update session data
err := container.SessionCache.UpdateSession(ctx, sessionID, map[string]interface{}{
    "last_activity": time.Now(),
    "page_views": 15,
})
```

### HTTP Middleware

```go
// Add cache middleware to HTTP server
cacheMiddleware := container.HTTPMiddleware.Middleware()

// Apply to routes
router.Use(cacheMiddleware)

// Or apply to specific routes
router.Handle("/api/recipes", cacheMiddleware(recipesHandler))
```

### Template Caching with HTMX

```go
// Cache rendered template
templateData := map[string]interface{}{
    "user": user,
    "recipes": recipes,
}

err := container.TemplateCache.CacheTemplate(ctx, "recipe-list", templateData, renderedHTML, request)

// Retrieve with automatic rendering fallback
html, metrics, err := container.TemplateCache.GetTemplate(ctx, "recipe-list", templateData, request, func(name string, data map[string]interface{}) (string, error) {
    return templateEngine.Render(name, data)
})
```

## Configuration

### Redis Configuration

```yaml
redis:
  host: localhost
  port: 6379
  database: 0
  max_retries: 3
  pool_size: 50
  dial_timeout: 5s
  read_timeout: 3s
  write_timeout: 3s
  enable_cluster: false
```

### Cache TTL Configuration

```yaml
cache:
  default_ttl: 1h
  recipe_ttl: 2h
  user_ttl: 30m
  session_ttl: 24h
  search_ttl: 15m
  ai_ttl: 1h
  template_ttl: 6h
```

### HTTP Cache Configuration

```yaml
http_cache:
  default_ttl: 5m
  api_ttl: 2m
  static_ttl: 24h
  htmx_ttl: 10m
  first_packet_optimization: true
  first_packet_target: 14336  # 14KB
  compression_enabled: true
```

## Monitoring & Alerts

### Health Check Endpoint

```http
GET /health/cache
```

Response:
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "overall": "healthy",
  "services": {
    "redis": {
      "status": "healthy",
      "message": "Redis connection active"
    },
    "cache": {
      "status": "healthy",
      "message": "Hit ratio: 96.5%"
    }
  }
}
```

### Metrics Endpoint

```http
GET /metrics/cache
```

Response:
```json
{
  "cache": {
    "total_operations": 1250000,
    "total_hits": 1200000,
    "total_misses": 50000,
    "hit_ratio": 0.96,
    "avg_read_time": "2.5ms",
    "l1_hits": 800000,
    "l2_hits": 400000
  },
  "redis": {
    "total_commands": 500000,
    "successful_ops": 499950,
    "failed_ops": 50,
    "avg_response_time": "1.2ms",
    "cache_hits": 400000,
    "cache_misses": 50000
  }
}
```

## Performance Optimization

### 14KB First Packet Optimization

The cache system implements several strategies to achieve the 14KB first packet goal:

1. **Critical CSS Inlining**: Inline critical CSS in template cache
2. **Above-the-fold Content Prioritization**: Prioritize visible content
3. **Lazy Loading**: Load non-critical content asynchronously
4. **HTMX Partial Caching**: Cache small, focused HTMX responses
5. **Template Fragmentation**: Break large templates into cacheable fragments

### Cache Key Strategy

Cache keys follow a hierarchical pattern:
```
alchemorsel:v3:{service}:{type}:{identifier}:{context}
```

Examples:
- `alchemorsel:v3:recipe:item:123e4567-e89b-12d3-a456-426614174000`
- `alchemorsel:v3:search:results:q8a7b9c2:f4d3e1a2`
- `alchemorsel:v3:template:recipe-card:d5f6g7h8:user:123`

### Invalidation Strategies

1. **Tag-based Invalidation**: Group related cache entries with tags
2. **Pattern-based Invalidation**: Use wildcards to invalidate related keys
3. **Cascade Invalidation**: Automatically invalidate dependent caches
4. **Time-based Expiration**: TTL-based automatic cleanup

## Best Practices

### 1. Cache-First Pattern

Always check cache before accessing source data:

```go
// ✅ Good: Cache-first with fallback
data, err := cache.Get(ctx, key)
if err == cache.ErrKeyNotFound {
    data, err = loadFromDatabase(ctx, id)
    if err == nil {
        cache.Set(ctx, key, data, ttl)
    }
}

// ❌ Bad: Direct database access
data, err := loadFromDatabase(ctx, id)
```

### 2. Appropriate TTL Selection

Choose TTL based on data volatility:

- **Static/Reference Data**: 24+ hours
- **User Profiles**: 30 minutes - 1 hour  
- **Session Data**: Session lifetime
- **Search Results**: 5-15 minutes
- **AI Responses**: 1-2 hours
- **Templates**: 30 minutes - 6 hours

### 3. Key Design

Use consistent, hierarchical key patterns:

```go
// ✅ Good: Structured, predictable
key := keyBuilder.BuildRecipeKey(recipeID)
// "alchemorsel:v3:recipe:123e4567-e89b-12d3-a456-426614174000"

// ❌ Bad: Inconsistent, hard to manage
key := fmt.Sprintf("recipe_%s_%d", recipeID, time.Now().Unix())
```

### 4. Error Handling

Handle cache errors gracefully:

```go
// ✅ Good: Graceful degradation
data, err := cache.Get(ctx, key)
if err != nil {
    // Log error but continue with fallback
    logger.Warn("Cache error", zap.Error(err))
    return loadFromSource(ctx, id)
}
```

### 5. Monitoring

Monitor key metrics:

- Hit ratio (target: >95%)
- Response times (target: <50ms cached, <200ms uncached)
- Error rates (target: <1%)
- Memory usage
- Connection pool health

## Testing

### Unit Tests

```bash
go test ./internal/infrastructure/cache/
```

### Performance Tests

```bash
go test -bench=. ./internal/infrastructure/cache/
```

### Load Testing

```bash
go test -run=TestCachePerformanceUnderLoad ./internal/infrastructure/cache/
```

## Troubleshooting

### Common Issues

1. **Low Hit Ratio**
   - Check TTL configuration
   - Verify key consistency
   - Review invalidation patterns

2. **High Response Times**
   - Check Redis connection health
   - Review connection pool settings
   - Monitor network latency

3. **Memory Issues**
   - Configure Redis memory limits
   - Implement eviction policies
   - Monitor key expiration

4. **Connection Errors**
   - Check Redis server health
   - Review connection pool configuration
   - Verify network connectivity

### Debug Mode

Enable debug logging for detailed cache operations:

```go
logger := zap.NewDevelopment()
container, err := cache.NewContainer(config, logger)
```

## Future Enhancements

1. **Distributed Caching**: Redis Cluster support for horizontal scaling
2. **Cache Warming**: Proactive cache population based on usage patterns
3. **Advanced Analytics**: ML-based cache optimization recommendations
4. **Edge Caching**: CDN integration for global performance
5. **Compression**: Advanced compression algorithms for large responses

## Contributing

When contributing to the cache infrastructure:

1. Follow the established patterns and interfaces
2. Add comprehensive tests for new features
3. Update metrics and monitoring for new cache types
4. Document performance implications
5. Ensure backward compatibility

## License

This cache infrastructure is part of Alchemorsel v3 and follows the project's licensing terms.