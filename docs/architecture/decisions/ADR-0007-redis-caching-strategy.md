# ADR-0007: Redis Caching Strategy

## Status
Accepted

## Context
Alchemorsel v3 handles computationally expensive operations including AI model inference, complex database queries, and frequently accessed user data. Without proper caching, these operations create performance bottlenecks and increased infrastructure costs.

Performance requirements:
- Sub-100ms response times for cached data
- Reduced database load for read-heavy operations
- Session storage for user authentication
- Temporary storage for AI model results
- Rate limiting and request throttling

Caching needs identified:
- User session data
- AI model inference results
- Frequently queried database results
- API response caching
- Rate limiting counters
- Background job queues

## Decision
We will implement Redis as a comprehensive caching layer with specific patterns for different data types and use cases.

**Redis Architecture:**
- Single Redis instance for development
- Redis Cluster for production high availability
- Redis Sentinel for automatic failover
- Separate logical databases for different concerns

**Caching Patterns:**

**Session Storage:**
- TTL: 24 hours for active sessions
- Key pattern: `session:{session_id}`
- JSON serialization for session objects

**AI Model Results:**
- TTL: 1 hour for inference results
- Key pattern: `ai:{model}:{input_hash}`
- Compressed storage for large responses

**Database Query Cache:**
- TTL: 5-15 minutes based on data volatility
- Key pattern: `db:{table}:{query_hash}`
- Invalidation on related data updates

**API Response Cache:**
- TTL: 1-60 minutes based on endpoint
- Key pattern: `api:{endpoint}:{params_hash}`
- HTTP cache headers integration

**Rate Limiting:**
- TTL: Sliding window (1 minute, 1 hour, 1 day)
- Key pattern: `rate:{user_id}:{endpoint}:{window}`
- Counter-based implementation

## Consequences

### Positive
- Dramatic performance improvements for repeated operations
- Reduced database load and cost optimization
- Scalable session management across multiple application instances
- Flexible TTL policies for different data types
- Built-in pub/sub for real-time features if needed
- Comprehensive monitoring and debugging tools

### Negative
- Additional infrastructure complexity and costs
- Data consistency challenges with cache invalidation
- Memory usage requires monitoring and optimization
- Cache warming strategies needed for critical paths
- Potential single point of failure without proper HA setup

### Neutral
- Industry standard caching solution with extensive documentation
- Compatible with Go ecosystem through mature libraries
- Migration path available to other caching solutions if needed