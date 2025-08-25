# Alchemorsel v3 Chat Interface - Comprehensive Test Suite

This comprehensive test suite provides >90% test coverage for the chat-centric conversational interface in Alchemorsel v3, ensuring robustness, performance, and security of the recipe creation and cooking assistance platform.

## Test Suite Overview

### Test Structure
```
test/
├── unit/                           # Unit tests for individual components
│   ├── websocket_manager_test.go   # WebSocket connection management
│   ├── ai_service_integration_test.go # AI service with mock providers
│   ├── chat_handlers_test.go       # HTTP and WebSocket handlers
│   ├── conversation_service_test.go # Core conversation logic
│   └── ai_service_test.go          # AI service component tests
├── integration/                    # Integration tests with real database
│   ├── conversation_repository_test.go # Database persistence layer
│   └── conversation_flow_test.go   # Full conversation flows
├── e2e/                           # End-to-end user workflow tests
│   └── chat_workflows_test.go     # Complete user journeys
├── performance/                   # Performance and scalability tests
│   └── websocket_scalability_test.go # Load and stress testing
├── security/                      # Security and vulnerability tests
│   └── chat_security_test.go      # Input validation, auth, XSS prevention
├── testutils/                     # Test utilities and helpers
│   ├── conversation_helpers.go    # Conversation test infrastructure
│   ├── conversation_mocks.go      # Mock implementations
│   ├── conversation_fixtures.go   # Test data fixtures
│   ├── database.go               # Database testing utilities
│   ├── assertions.go             # Custom test assertions
│   ├── factories.go              # Test data factories
│   └── mocks.go                  # General mocks
├── fixtures/                      # Static test data
└── README.md                      # This documentation
```

## Test Categories

### 1. Unit Tests (>90% Code Coverage)

#### WebSocket Manager Tests (`websocket_manager_test.go`)
- **Connection Management**: Basic connection establishment, multiple users, same-user multiple connections
- **Message Handling**: Sending/receiving messages, broadcast functionality, ping/pong heartbeat
- **Connection Cleanup**: Automatic cleanup of stale connections, resource management
- **Concurrency**: 50+ concurrent connections, message handling under load
- **Error Handling**: Invalid connections, message sending failures, protocol compliance
- **Security**: Connection limits, session isolation

#### AI Service Integration Tests (`ai_service_integration_test.go`)
- **Ollama Integration**: Primary AI service, health checks, response generation
- **OpenAI Fallback**: Automatic fallback when Ollama unavailable, error handling
- **System Prompts**: Intent-specific prompts, conversation context handling
- **Conversation History**: Context preservation, message truncation, multi-turn dialogues
- **Recipe Extraction**: NLP-based recipe component extraction, context analysis
- **Performance**: Response time measurement, concurrent request handling
- **Error Scenarios**: Timeout handling, invalid responses, service unavailability

#### Chat Handlers Tests (`chat_handlers_test.go`)
- **HTTP Endpoints**: Chat messages, conversation management, history retrieval
- **WebSocket Handlers**: Real-time messaging, connection upgrades, message processing
- **HTMX Integration**: Server-side rendering, progressive enhancement, mobile support
- **Authentication**: User context validation, unauthorized access prevention
- **Input Validation**: Message format validation, parameter checking
- **Error Responses**: Graceful error handling, appropriate HTTP status codes

#### Conversation Service Tests (`conversation_service_test.go`)
- **Intent Classification**: Recipe creation, cooking help, substitutions, meal planning
- **Context Management**: Conversation state, workflow progression, data persistence
- **Message Processing**: User input handling, AI response generation, metadata tracking
- **Lifecycle Management**: Creation, archiving, deletion, statistics
- **Edge Cases**: Empty messages, long conversations, concurrent operations

### 2. Integration Tests (Real Database)

#### Repository Layer Tests (`conversation_repository_test.go`)
- **CRUD Operations**: Create, read, update, delete for conversations, messages, contexts
- **Data Integrity**: Foreign key constraints, transaction handling, rollback scenarios
- **Query Performance**: Large datasets, pagination, indexing effectiveness
- **Concurrent Access**: Multiple users, database locking, race conditions
- **Data Types**: JSON metadata, Unicode content, large text handling
- **Migration**: Schema evolution, backward compatibility

#### Conversation Flow Tests (`conversation_flow_test.go`)
- **Complete Workflows**: Recipe creation from start to finish, multi-step conversations
- **Cross-User Isolation**: Data privacy, access control, conversation ownership
- **WebSocket Integration**: Real-time conversation through WebSocket connections
- **Context Persistence**: Conversation state maintenance across requests
- **Error Recovery**: Handling failures gracefully, conversation resumption
- **Multi-Device**: Conversation continuity across different connection types

### 3. End-to-End Tests (User Workflows)

#### Chat Workflows Tests (`chat_workflows_test.go`)
- **New User Journey**: First-time recipe creation, guided assistance
- **Expert Chef Workflows**: Complex recipes, advanced techniques, timing coordination
- **Cooking Help**: Technique assistance, troubleshooting, equipment guidance
- **Emergency Scenarios**: Ingredient substitutions, last-minute changes
- **Meal Planning**: Weekly planning, dietary restrictions, shopping lists
- **Multi-Device Continuity**: Mobile to desktop, conversation synchronization
- **Error Recovery**: Network failures, graceful degradation

### 4. Performance Tests (Scalability)

#### WebSocket Scalability Tests (`websocket_scalability_test.go`)
- **Connection Limits**: 10-250 concurrent connections, resource usage monitoring
- **Message Throughput**: 50-500 messages/second, latency measurement
- **Memory Usage**: Connection lifecycle, cleanup efficiency, memory leaks
- **Connection Stability**: Long-running connections, heartbeat mechanisms
- **Load Testing**: Stress testing under high concurrent load
- **Benchmarking**: Connection establishment, message sending performance

**Performance Requirements:**
- Connection success rate: ≥95%
- Average connection time: ≤1000ms
- Average response time: ≤100ms
- Message success rate: ≥98%
- Support 100+ concurrent users
- Memory efficient cleanup

### 5. Security Tests (Vulnerability Assessment)

#### Chat Security Tests (`chat_security_test.go`)
- **Input Validation**: XSS prevention, script injection, malformed input
- **Authentication**: User verification, session management, unauthorized access
- **Authorization**: Cross-user access prevention, conversation ownership
- **Rate Limiting**: DDoS protection, abuse prevention, fair usage
- **Data Protection**: PII handling, information disclosure prevention
- **Injection Attacks**: SQL, NoSQL, command, path traversal, XML injection
- **Protocol Security**: WebSocket security, CSRF protection, secure headers

**Security Test Cases:**
- Script tag injection: `<script>alert('xss')</script>`
- SQL injection: `'; DROP TABLE conversations; --`
- Path traversal: `../../../../etc/passwd`
- Large payload attacks: 10MB+ messages
- Unicode/encoding attacks: `\u003cscript\u003e`
- Rate limiting: 50+ requests/second

## Test Utilities and Infrastructure

### Mock Services
- **AI Service Mocks**: Ollama and OpenAI client mocks with configurable responses
- **Repository Mocks**: Database layer mocks for unit testing
- **WebSocket Mocks**: Connection and message handling simulation

### Test Fixtures
- **Conversation Scenarios**: Pre-built conversation flows for different intents
- **Message Templates**: Standard message patterns for testing
- **Context Data**: Realistic conversation context examples
- **User Profiles**: Different user types (beginner, expert, admin)

### Database Testing
- **SQLite**: In-memory database for unit tests (fast, isolated)
- **PostgreSQL**: Full database for integration tests (realistic)
- **Schema Migration**: Automatic schema setup and teardown
- **Data Seeding**: Realistic test data population
- **Transaction Rollback**: Clean test isolation

## Running Tests

### Prerequisites
```bash
# Install Go dependencies
go mod download

# For PostgreSQL integration tests (optional)
export TEST_DATABASE=postgres
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5432
export TEST_DB_USER=test
export TEST_DB_PASSWORD=test
export TEST_DB_NAME=alchemorsel_test
```

### Individual Test Categories
```bash
# Unit tests (fast, no external dependencies)
go test ./test/unit/... -v

# Integration tests (requires database)
go test ./test/integration/... -v

# End-to-end tests (full system)
go test ./test/e2e/... -v

# Performance tests (longer running)
go test ./test/performance/... -v

# Security tests (vulnerability scanning)
go test ./test/security/... -v
```

### Complete Test Suite
```bash
# Run all tests with coverage
go test ./test/... -v -cover -coverprofile=coverage.out

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html

# Run with race detection
go test ./test/... -race -v

# Run performance benchmarks
go test ./test/performance/... -bench=. -benchmem
```

### CI/CD Pipeline
```bash
# Quick tests for PR validation
make test-quick

# Full test suite for release
make test-full

# Security and performance tests
make test-security
make test-performance
```

## Test Configuration

### Environment Variables
- `TEST_DATABASE`: Database type (sqlite/postgres)
- `TEST_DB_*`: PostgreSQL connection settings
- `ENABLE_INTEGRATION_TESTS`: Enable integration tests
- `ENABLE_PERFORMANCE_TESTS`: Enable performance tests
- `TEST_TIMEOUT`: Test timeout duration

### Test Tags
```bash
# Run only unit tests
go test -tags unit ./test/...

# Run integration tests
go test -tags integration ./test/...

# Skip slow tests
go test -short ./test/...
```

## Coverage Requirements

### Minimum Coverage Targets
- **Overall Code Coverage**: >90%
- **Core Logic Coverage**: >95%
- **Handler Coverage**: >90%
- **Repository Coverage**: >95%
- **WebSocket Manager**: >90%

### Coverage Areas
1. **Conversation Service**: Intent classification, message processing, context management
2. **AI Integration**: Response generation, fallback handling, error scenarios
3. **WebSocket Management**: Connection lifecycle, message routing, cleanup
4. **HTTP Handlers**: Request processing, authentication, error handling
5. **Database Layer**: CRUD operations, transactions, data integrity

## Quality Assurance

### Test Quality Metrics
- **Test Reliability**: No flaky tests, consistent results
- **Test Speed**: Unit tests <1s, integration tests <10s per test
- **Test Maintenance**: Clear naming, good documentation, minimal duplication
- **Assertion Quality**: Meaningful assertions, proper error messages

### Best Practices Implemented
- **Isolation**: Each test is independent and can run in any order
- **Realistic Data**: Tests use realistic conversation scenarios
- **Error Scenarios**: Comprehensive error condition testing
- **Performance Monitoring**: Response time and resource usage tracking
- **Security Focus**: Proactive vulnerability testing

## Continuous Integration

### GitHub Actions Workflow
```yaml
name: Test Suite
on: [push, pull_request]
jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
      - run: go test ./test/unit/... -v -cover
  
  integration-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:14
        env:
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
          POSTGRES_DB: alchemorsel_test
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
      - run: go test ./test/integration/... -v
  
  security-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
      - run: go test ./test/security/... -v
```

## Troubleshooting

### Common Issues
1. **Database Connection**: Ensure PostgreSQL is running for integration tests
2. **WebSocket Tests**: May require increased timeouts in CI environments
3. **Performance Tests**: Results may vary based on hardware capabilities
4. **Race Conditions**: Run with `-race` flag to detect concurrency issues

### Debugging Tools
- `go test -v`: Verbose test output
- `go test -race`: Race condition detection
- `go test -cover`: Coverage analysis
- `go test -bench=.`: Performance benchmarking

## Maintenance

### Regular Tasks
- **Update Fixtures**: Keep test data current with application changes
- **Review Coverage**: Ensure new code has adequate test coverage
- **Performance Monitoring**: Track test execution time and resource usage
- **Security Updates**: Add tests for new security scenarios

### Test Evolution
- **New Features**: Add tests before implementing new functionality
- **Regression Prevention**: Add tests for every bug fix
- **Performance Baselines**: Update performance expectations as system evolves
- **Security Hardening**: Continuously add new attack vector tests

## Conclusion

This comprehensive test suite provides robust validation of the Alchemorsel v3 chat-centric conversational interface. With >90% code coverage across unit, integration, E2E, performance, and security tests, the system is thoroughly validated for:

- **Functionality**: All conversation workflows work correctly
- **Reliability**: System handles errors gracefully and recovers properly
- **Performance**: Meets scalability requirements under load
- **Security**: Resistant to common web application vulnerabilities
- **Maintainability**: Well-tested code is easier to modify and extend

The test suite ensures that users can reliably create recipes, get cooking help, find ingredient substitutions, and plan meals through natural conversational interfaces across web, mobile, and real-time platforms.