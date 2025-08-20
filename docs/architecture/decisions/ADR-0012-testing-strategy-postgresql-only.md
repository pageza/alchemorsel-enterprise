# ADR-0012: Testing Strategy (PostgreSQL-Only)

## Status
Accepted

## Context
Alchemorsel v3's PostgreSQL-only database strategy (ADR-0002) requires a comprehensive testing approach that accurately reflects production behavior while maintaining fast test execution. Testing against SQLite or in-memory databases can miss PostgreSQL-specific features and behaviors, leading to production issues.

Testing challenges:
- PostgreSQL-specific SQL features and data types
- Transaction isolation and concurrency behavior
- Performance characteristics under load
- Database migration testing
- Integration testing with real database constraints

Testing requirements:
- Fast feedback loop for developers
- Accurate representation of production behavior
- Parallel test execution support
- Comprehensive coverage including edge cases
- CI/CD pipeline integration

## Decision
We will implement a PostgreSQL-only testing strategy using containerized test databases with parallel execution and comprehensive test categorization.

**Testing Architecture:**

**Test Database Strategy:**
- Dockerized PostgreSQL for all database tests
- Separate test database per test suite for parallel execution
- Database schema migrations tested against PostgreSQL
- Test data seeding using PostgreSQL-compatible scripts

**Test Categories:**

**Unit Tests:**
- Business logic without database dependencies
- Mocked database interfaces for pure function testing
- Fast execution (<1s per test suite)
- No external dependencies required

**Integration Tests:**
- Database operations with real PostgreSQL instances
- API endpoint testing with test database
- Service integration with all dependencies
- Medium execution time (<30s per test suite)

**End-to-End Tests:**
- Full application workflow testing
- Docker Compose test environment
- Real browser testing with Playwright/Selenium
- Slower execution (1-5 minutes per test suite)

**Database-Specific Testing:**
```go
// Example test structure
func TestUserRepository_PostgreSQL(t *testing.T) {
    db := setupTestDB(t) // Creates isolated PostgreSQL container
    defer teardownTestDB(t, db)
    
    repo := NewUserRepository(db)
    
    t.Run("complex_query_with_jsonb", func(t *testing.T) {
        // Test PostgreSQL-specific JSONB functionality
    })
    
    t.Run("concurrent_transactions", func(t *testing.T) {
        // Test PostgreSQL transaction isolation
    })
}
```

**CI/CD Integration:**
- GitHub Actions with PostgreSQL service containers
- Parallel test execution with database isolation
- Test result reporting and coverage tracking
- Performance regression detection

**Test Data Management:**
- Factory pattern for test data generation
- Database fixtures for complex test scenarios
- Cleanup procedures for test isolation
- Consistent test data across environments

## Consequences

### Positive
- Tests accurately reflect production PostgreSQL behavior
- Early detection of database-specific issues
- Comprehensive coverage of PostgreSQL features
- Reliable CI/CD pipeline with consistent results
- Parallel test execution reduces overall test time

### Negative
- Slower test execution compared to in-memory databases
- Additional infrastructure complexity with Docker containers
- Requires PostgreSQL knowledge for test development
- Higher resource usage for test environments

### Neutral
- Testing approach aligns with production architecture
- Industry standard for database-specific testing
- Compatible with modern CI/CD platforms