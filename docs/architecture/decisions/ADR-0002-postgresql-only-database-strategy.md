# ADR-0002: PostgreSQL-Only Database Strategy

## Status
Accepted

## Context
Alchemorsel v3 requires a robust, ACID-compliant database system that can handle complex queries, provide strong consistency, and scale effectively. Previous versions used multiple database technologies, creating operational complexity and inconsistent data patterns.

Key requirements:
- ACID compliance for financial and user data
- Complex query support for analytics and reporting
- Strong typing and data validation
- Excellent Go ecosystem integration
- Proven production scalability
- Comprehensive backup and recovery options

## Decision
We will use PostgreSQL as the sole database technology for all Alchemorsel v3 data storage needs.

**Implementation Requirements:**
- All data persistence must use PostgreSQL 15+
- No other database technologies (MySQL, MongoDB, etc.) permitted
- Database interactions must use `github.com/lib/pq` or `pgx` drivers
- All schema changes must use database migrations
- Local development must use Docker PostgreSQL containers
- Production deployments must use managed PostgreSQL services

**Prohibited:**
- SQLite for any production use cases
- NoSQL databases for primary data storage
- File-based storage for structured data
- Mixed database architectures

## Consequences

### Positive
- Single database technology reduces operational complexity
- Excellent ACID guarantees for all data operations
- Rich query capabilities with SQL and PostgreSQL extensions
- Strong Go ecosystem support with mature drivers
- Comprehensive tooling for monitoring, backup, and recovery
- JSON/JSONB support for flexible schema requirements
- Proven scalability patterns (read replicas, partitioning)

### Negative
- Learning curve for team members unfamiliar with PostgreSQL
- Single point of failure if not properly configured
- May be overkill for simple key-value storage needs
- Requires PostgreSQL-specific optimization knowledge

### Neutral
- Database costs predictable with single technology
- Monitoring and alerting simplified with unified metrics
- Documentation and training efforts consolidated