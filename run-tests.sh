#!/bin/bash

echo "🧪 Alchemorsel v3 Testing Framework"
echo "=================================="
echo

# Use Go 1.23 for compatibility
export PATH="/usr/local/go/bin:$PATH"

echo "📋 Available Test Categories:"
echo "• Unit Tests: Domain logic and business rules"
echo "• Integration Tests: API endpoints and database operations"  
echo "• E2E Tests: HTMX frontend interactions"
echo "• Security Tests: Authentication, authorization, input validation"
echo "• Performance Tests: Load testing and benchmarks"
echo "• TestContainers: Isolated PostgreSQL testing"
echo

echo "🔧 Test Framework Features:"
echo "• Testify for assertions and test suites"
echo "• TestContainers for database isolation" 
echo "• GORM integration testing"
echo "• JWT authentication testing"
echo "• HTMX interaction testing"
echo "• Comprehensive security testing"
echo "• Performance benchmarking"
echo

echo "📁 Test Structure:"
find test -name "*.go" | head -10
echo

echo "⚡ Example Test Commands:"
echo "# Run all unit tests:"
echo "go test ./internal/domain/... -v -short"
echo
echo "# Run integration tests with database:"
echo "go test ./test/integration/... -v"
echo
echo "# Run security tests:"
echo "go test ./test/security/... -v"
echo
echo "# Run performance benchmarks:"
echo "go test ./test/performance/... -bench=."
echo
echo "# Run with coverage:"
echo "go test ./... -cover -coverprofile=coverage.out"
echo

echo "🐳 TestContainers Integration:"
echo "Tests automatically spin up isolated PostgreSQL containers"
echo "No manual database setup required for testing"
echo "Each test suite gets a fresh database instance"
echo

echo "✅ Testing is ready! Use the commands above to run tests."