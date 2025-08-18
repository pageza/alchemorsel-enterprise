# Alchemorsel v3 Testing Strategy

## Executive Summary

This document outlines the comprehensive enterprise-grade testing strategy for Alchemorsel v3, designed to demonstrate advanced QA engineering practices that startup CTOs value for building reliable, scalable products. Our approach balances rigorous quality assurance with development velocity, implementing the right tests at the right levels.

## Testing Philosophy

### Core Principles

1. **Test Pyramid Implementation**: 70% Unit, 20% Integration, 10% E2E
2. **Critical Path Focus**: Avoid testing hell by focusing on business value
3. **Quality Gates**: Automated quality enforcement in CI/CD
4. **Shift Left**: Catch issues as early as possible
5. **Performance-Driven**: Test performance as a feature, not an afterthought

### Testing Levels

```
    /\        E2E Tests (10%)
   /  \       User journeys, browser automation
  /____\      HTMX interactions, accessibility
 /      \     
/________\     Integration Tests (20%)
|        |     API contracts, database interactions
|        |     External service integration
|________|     
|        |     Unit Tests (70%)
|        |     Domain logic, business rules
|        |     Security functions, utilities
|________|     
```

## Testing Framework Selection

### Go Backend Testing Stack

| Framework | Purpose | Rationale |
|-----------|---------|-----------|
| **testify/suite** | Test organization | Structured test setup/teardown, readable assertions |
| **testcontainers-go** | Integration testing | Real database testing with isolated containers |
| **gomock** | Mocking | Type-safe mock generation for interfaces |
| **Ginkgo + Gomega** | BDD testing | Readable specs for complex business logic |
| **Go Benchmark** | Performance testing | Built-in benchmarking with regression detection |
| **httptest** | HTTP testing | Fast HTTP handler testing without network |

### Frontend Testing Stack

| Framework | Purpose | Rationale |
|-----------|---------|-----------|
| **Playwright** | E2E testing | Reliable browser automation with modern features |
| **HTMX Test Helpers** | HTMX testing | Custom utilities for HTMX interaction testing |
| **axe-core** | Accessibility | Automated accessibility testing |
| **WebPageTest API** | Performance | Real-world performance monitoring |

## Test Categories

### 1. Unit Tests (70% of total tests)

**Target Coverage**: 85%+ for domain logic, 95%+ for critical security functions

#### Domain Logic Tests
- **Recipe Entity**: Business rule validation, state transitions
- **User Entity**: Authentication, authorization, profile management
- **Value Objects**: Immutability, validation, equality
- **Domain Services**: Complex business logic, calculations

#### Infrastructure Tests
- **Security Functions**: Password hashing, JWT validation, encryption
- **Utilities**: Validation, formatting, calculations
- **Configuration**: Environment-specific settings validation

#### Characteristics
- **Fast**: < 10ms execution time per test
- **Isolated**: No external dependencies
- **Deterministic**: Same result every time
- **Focused**: Single responsibility testing

### 2. Integration Tests (20% of total tests)

**Target Coverage**: 100% of critical integration points

#### Database Integration
- **Repository Layer**: CRUD operations, complex queries
- **Transaction Management**: ACID compliance, rollback scenarios
- **Migration Testing**: Schema changes, data integrity
- **Performance**: Query optimization, index effectiveness

#### External Service Integration
- **AI Services**: OpenAI API integration, fallback scenarios
- **Email Services**: Template rendering, delivery confirmation
- **Storage Services**: File upload, CDN integration
- **Payment Services**: Subscription management, webhook handling

#### Message Bus Integration
- **Event Publishing**: Domain event dispatch
- **Event Handling**: Reliable message processing
- **Dead Letter Queues**: Error handling, retry logic

#### Characteristics
- **Realistic**: Use real databases and services
- **Isolated**: Each test runs in clean environment
- **Comprehensive**: Cover all integration scenarios
- **Monitored**: Track performance and reliability

### 3. End-to-End Tests (10% of total tests)

**Target Coverage**: 100% of critical user journeys

#### Critical User Journeys
- **User Registration**: Sign up, email verification, onboarding
- **Recipe Creation**: AI generation, manual creation, publishing
- **Recipe Discovery**: Search, filtering, recommendations
- **Social Features**: Likes, comments, sharing
- **Premium Features**: Subscription flow, feature access

#### HTMX-Specific Testing
- **Progressive Enhancement**: Functionality without JavaScript
- **Dynamic Updates**: Partial page updates, state management
- **Form Handling**: Validation, submission, error display
- **Real-time Features**: Live updates, notifications

#### Cross-Browser Testing
- **Desktop**: Chrome, Firefox, Safari, Edge
- **Mobile**: iOS Safari, Android Chrome
- **Accessibility**: Screen readers, keyboard navigation

## Performance Testing Strategy

### Benchmark Categories

#### 1. Application Benchmarks
```go
// Example: Recipe creation performance
func BenchmarkRecipeCreation(b *testing.B) {
    for i := 0; i < b.N; i++ {
        recipe, err := domain.NewRecipe(
            "Test Recipe", 
            "Description", 
            uuid.New(),
        )
        require.NoError(b, err)
        require.NotNil(b, recipe)
    }
}
```

#### 2. Database Benchmarks
- **Query Performance**: Complex searches, aggregations
- **Connection Pooling**: Concurrent access patterns
- **Migration Speed**: Schema change performance

#### 3. API Benchmarks
- **Endpoint Performance**: Response time under load
- **Concurrent Users**: Throughput testing
- **Memory Usage**: Resource consumption patterns

#### 4. Frontend Performance
- **First Contentful Paint**: < 1.5s target
- **Largest Contentful Paint**: < 2.5s target
- **Time to Interactive**: < 3.5s target
- **14KB First Packet**: Critical request optimization

### Performance Regression Detection

```yaml
# GitHub Actions performance gate
- name: Performance Regression Check
  run: |
    go test -bench=. -benchmem ./... | tee benchmark.txt
    benchstat baseline.txt benchmark.txt || exit 1
```

## Security Testing Framework

### 1. Authentication Testing
- **JWT Security**: Token validation, expiration, revocation
- **Session Management**: Concurrent sessions, hijacking prevention
- **Password Security**: Hashing, complexity validation
- **MFA Testing**: TOTP generation, backup codes

### 2. Authorization Testing
- **RBAC**: Role-based access control validation
- **Resource Protection**: Owner-only access verification
- **Privilege Escalation**: Unauthorized access attempts

### 3. Input Validation Testing
- **SQL Injection**: Parameterized query verification
- **XSS Protection**: Input sanitization, output encoding
- **CSRF Protection**: Token validation, SameSite cookies
- **File Upload**: Type validation, size limits

### 4. Rate Limiting Testing
- **API Rate Limits**: Request throttling validation
- **Authentication Limits**: Brute force protection
- **Resource Limits**: Memory and CPU protection

### 5. Data Protection Testing
- **Encryption**: Data at rest and in transit
- **PII Handling**: GDPR compliance, data anonymization
- **Audit Logging**: Security event tracking

## Test Infrastructure

### Test Database Management

```go
// testcontainers setup for isolated database testing
func setupTestDB(t *testing.T) *sql.DB {
    ctx := context.Background()
    
    postgres, err := testcontainers.GenericContainer(ctx, 
        testcontainers.GenericContainerRequest{
            ContainerRequest: testcontainers.ContainerRequest{
                Image:        "postgres:15-alpine",
                ExposedPorts: []string{"5432/tcp"},
                Env: map[string]string{
                    "POSTGRES_DB":       "alchemorsel_test",
                    "POSTGRES_USER":     "test",
                    "POSTGRES_PASSWORD": "test",
                },
                WaitingFor: wait.ForLog("database system is ready"),
            },
            Started: true,
        })
    require.NoError(t, err)
    
    // Cleanup
    t.Cleanup(func() {
        postgres.Terminate(ctx)
    })
    
    // Return database connection
    return connectToTestDB(t, postgres)
}
```

### Mock Services

#### AI Service Mocking
```go
type MockAIService struct {
    mock.Mock
}

func (m *MockAIService) GenerateRecipe(ctx context.Context, prompt string) (*Recipe, error) {
    args := m.Called(ctx, prompt)
    return args.Get(0).(*Recipe), args.Error(1)
}

// Predefined responses for consistent testing
func (m *MockAIService) SetupStandardResponses() {
    m.On("GenerateRecipe", mock.Anything, "italian pasta").
        Return(&Recipe{
            Title: "Spaghetti Carbonara",
            // ... predefined recipe data
        }, nil)
}
```

### Test Data Factories

```go
// Recipe factory for consistent test data
type RecipeFactory struct {
    faker *gofakeit.Faker
}

func (rf *RecipeFactory) CreateValidRecipe() *domain.Recipe {
    recipe, _ := domain.NewRecipe(
        rf.faker.Sentence(3),
        rf.faker.Paragraph(2, 3, 5, " "),
        uuid.New(),
    )
    
    // Add ingredients
    recipe.AddIngredient(domain.Ingredient{
        Name:     rf.faker.Food(),
        Quantity: rf.faker.Float32Range(0.5, 2.0),
        Unit:     "cups",
    })
    
    return recipe
}
```

### Parallel Test Execution

```go
// Parallel test execution with proper isolation
func TestRecipeOperations(t *testing.T) {
    tests := []struct {
        name string
        test func(*testing.T, *sql.DB)
    }{
        {"Create Recipe", testCreateRecipe},
        {"Update Recipe", testUpdateRecipe},
        {"Delete Recipe", testDeleteRecipe},
    }
    
    for _, tt := range tests {
        tt := tt // capture range variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            db := setupTestDB(t)
            tt.test(t, db)
        })
    }
}
```

## Quality Assurance Tools

### Code Coverage

```makefile
# Coverage reporting with thresholds
.PHONY: test-coverage
test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | grep total | awk '{if($$3 < 80.0) exit 1}'
```

### Static Analysis

```yaml
# golangci-lint configuration
linters:
  enable:
    - gosec        # Security analyzer
    - govet        # Go vet
    - errcheck     # Error checking
    - staticcheck  # Staticcheck
    - unused       # Unused code
    - gosimple     # Simplification
    - ineffassign  # Ineffectual assignments
    - misspell     # Misspellings
    - gocyclo      # Cyclomatic complexity
    - dupl         # Code duplication
    - goconst      # Repeated strings
    - gofmt        # Formatting
    - goimports    # Import formatting
```

### Mutation Testing

```go
// Critical function for mutation testing
func (r *Recipe) Publish() error {
    if r.status != RecipeStatusDraft {  // Mutate != to ==
        return ErrInvalidStatusTransition
    }
    
    if len(r.ingredients) == 0 {        // Mutate == to !=
        return ErrNoIngredients
    }
    
    // Mutation testing ensures these conditions are properly tested
    return nil
}
```

### Security Scanning

```yaml
# Security scanning with gosec
security-scan:
  stage: test
  script:
    - gosec -fmt json -out gosec-report.json ./...
    - govulncheck ./...
    - nancy sleuth --loud
```

## Enterprise Testing Practices

### Test Documentation

#### Living Documentation with Ginkgo
```go
var _ = Describe("Recipe Publishing", func() {
    Context("When a recipe is in draft status", func() {
        It("Should publish successfully with valid data", func() {
            recipe := RecipeFactory.CreateValidRecipe()
            
            err := recipe.Publish()
            
            Expect(err).ToNot(HaveOccurred())
            Expect(recipe.Status()).To(Equal(RecipeStatusPublished))
        })
        
        It("Should fail when ingredients are missing", func() {
            recipe := RecipeFactory.CreateRecipeWithoutIngredients()
            
            err := recipe.Publish()
            
            Expect(err).To(Equal(ErrNoIngredients))
        })
    })
})
```

### Test Environment Management

```yaml
# Docker Compose for test environment
version: '3.8'
services:
  postgres-test:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: alchemorsel_test
      POSTGRES_USER: test
      POSTGRES_PASSWORD: test
    ports:
      - "5433:5432"
    tmpfs:
      - /var/lib/postgresql/data  # In-memory for speed
      
  redis-test:
    image: redis:7-alpine
    ports:
      - "6380:6379"
    command: redis-server --save ""  # No persistence
```

### Flaky Test Detection

```go
// Flaky test detection with retry logic
func TestWithRetry(t *testing.T) {
    const maxRetries = 3
    
    for i := 0; i < maxRetries; i++ {
        t.Run(fmt.Sprintf("attempt_%d", i+1), func(t *testing.T) {
            if err := runTest(); err != nil {
                if i == maxRetries-1 {
                    t.Fatalf("Test failed after %d attempts: %v", maxRetries, err)
                }
                t.Skipf("Retrying test, attempt %d failed: %v", i+1, err)
            }
        })
    }
}
```

### Test Metrics and Reporting

```go
// Test metrics collection
type TestMetrics struct {
    TestDuration    time.Duration
    TestCount       int
    FailureCount    int
    CoveragePercent float64
    PerformanceData map[string]time.Duration
}

func (tm *TestMetrics) Report() {
    log.Printf("Test Summary:")
    log.Printf("  Tests Run: %d", tm.TestCount)
    log.Printf("  Failures: %d", tm.FailureCount)
    log.Printf("  Success Rate: %.2f%%", float64(tm.TestCount-tm.FailureCount)/float64(tm.TestCount)*100)
    log.Printf("  Coverage: %.2f%%", tm.CoveragePercent)
    log.Printf("  Duration: %v", tm.TestDuration)
}
```

## CI/CD Integration

### Quality Gates

```yaml
# GitHub Actions quality gates
name: Quality Gates
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v3
        with:
          go-version: 1.22
          
      - name: Unit Tests
        run: |
          go test -race -coverprofile=coverage.out ./...
          go tool cover -func=coverage.out | grep total | awk '{if($3 < 80.0) exit 1}'
          
      - name: Integration Tests
        run: go test -tags=integration ./test/integration/...
        
      - name: Security Scan
        run: |
          go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
          gosec ./...
          
      - name: Performance Benchmarks
        run: |
          go test -bench=. -benchmem ./... | tee benchmark.txt
          benchstat baseline.txt benchmark.txt
          
      - name: E2E Tests
        run: |
          docker-compose -f docker-compose.test.yml up -d
          npm test -- --config=playwright.config.js
          docker-compose -f docker-compose.test.yml down
```

### Quality Dashboard

```go
// Quality dashboard data structure
type QualityDashboard struct {
    Timestamp       time.Time `json:"timestamp"`
    BuildNumber     string    `json:"build_number"`
    TestResults     TestMetrics `json:"test_results"`
    SecurityScore   int       `json:"security_score"`
    PerformanceData map[string]Benchmark `json:"performance_data"`
    CoverageReport  CoverageReport `json:"coverage_report"`
}

type Benchmark struct {
    Name            string        `json:"name"`
    AverageTime     time.Duration `json:"average_time"`
    MemoryUsage     int64         `json:"memory_usage"`
    RegressionCheck bool          `json:"regression_check"`
}
```

## Testing Best Practices

### 1. Test Organization
- **Package Structure**: Mirror production code structure
- **Test Files**: Use `_test.go` suffix consistently
- **Helper Functions**: Create reusable test utilities
- **Test Data**: Use factories and builders for complex objects

### 2. Test Naming Conventions
```go
// Pattern: TestUnitOfWork_StateUnderTest_ExpectedBehavior
func TestRecipe_Publish_WhenDraft_ShouldSetPublishedStatus(t *testing.T) {}
func TestRecipe_Publish_WhenMissingIngredients_ShouldReturnError(t *testing.T) {}
func TestRecipe_AddRating_WhenValidRating_ShouldUpdateAverage(t *testing.T) {}
```

### 3. Assertion Strategies
```go
// Use specific assertions for better error messages
assert.Equal(t, RecipeStatusPublished, recipe.Status())
assert.Contains(t, recipe.Tags(), "italian")
assert.InDelta(t, 4.5, recipe.AverageRating(), 0.01)
assert.NoError(t, err)
assert.NotNil(t, recipe.PublishedAt())
```

### 4. Test Data Management
```go
// Use builders for complex test data
recipe := NewRecipeBuilder().
    WithTitle("Spaghetti Carbonara").
    WithIngredients([]Ingredient{
        {Name: "Spaghetti", Quantity: 1, Unit: "lb"},
        {Name: "Eggs", Quantity: 4, Unit: "pieces"},
    }).
    WithInstructions([]Instruction{
        {Step: 1, Description: "Boil water"},
        {Step: 2, Description: "Cook pasta"},
    }).
    Build()
```

## Conclusion

This comprehensive testing strategy demonstrates enterprise-grade QA practices that balance thorough quality assurance with development velocity. By implementing the right tests at the right levels, we ensure:

1. **Rapid Feedback**: Unit tests catch issues immediately
2. **Integration Confidence**: Integration tests verify system interactions
3. **User Experience**: E2E tests validate complete user journeys
4. **Performance Assurance**: Benchmarks prevent performance regressions
5. **Security Validation**: Security tests protect against vulnerabilities
6. **Maintainability**: Well-structured tests support long-term maintenance

The strategy showcases advanced testing knowledge while maintaining practical engineering judgment—exactly what startup CTOs value for building reliable, scalable products.