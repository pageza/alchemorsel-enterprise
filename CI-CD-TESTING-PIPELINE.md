# Alchemorsel v3 CI/CD Testing Pipeline Documentation

## Table of Contents
1. [Overview](#overview)
2. [Local Testing Environment](#local-testing-environment)
3. [Test Organization](#test-organization)
4. [CI Pipeline Structure](#ci-pipeline-structure)
5. [CD Pipeline Structure](#cd-pipeline-structure)
6. [Security Scanning Pipeline](#security-scanning-pipeline)
7. [Testing Matrix](#testing-matrix)
8. [Build Processes](#build-processes)
9. [Quality Gates](#quality-gates)
10. [Deployment Strategy](#deployment-strategy)

---

## Overview

The Alchemorsel v3 project implements a comprehensive, enterprise-grade CI/CD pipeline with multiple layers of testing, security scanning, and deployment automation. The pipeline is designed to ensure code quality, security, and reliability across all environments.

**Key Standards:**
- Go 1.23 standardized across all environments
- PostgreSQL-only database testing (no SQLite)
- Multi-stage security scanning
- Tiered testing strategy (feature → PR → main)
- Containerized deployments with security hardening

---

## Local Testing Environment

### Prerequisites
- Go 1.23
- Docker & Docker Compose
- PostgreSQL 15+
- Redis 7+

### Available Test Commands

```bash
# Quick validation (formatting, vet, build)
go vet ./...
gofmt -s -l .
go build -v ./...

# Unit tests (fast feedback)
go test -short -v -race -coverprofile=coverage.out ./test/unit/...

# Integration tests (requires services)
docker-compose -f deployments/docker/docker-compose.test.yml up -d
go test -v -race -tags=integration -run="TestIntegration" ./test/integration/...

# Security tests
go test -v -tags=security ./test/security/...

# Performance tests
go test -v -tags=performance -run="TestPerformance" ./test/performance/...
go test -bench=. -benchmem -count=3 -run=^$ ./pkg/healthcheck/

# E2E tests (full stack)
go test -v -tags=e2e -run="TestE2E" ./test/e2e/...

# Health check specific tests
go test -short -v -race -coverprofile=healthcheck-coverage.out ./pkg/healthcheck/

# Static analysis
staticcheck ./...
gosec ./...

# Database migrations
go run cmd/migrate/main.go up
go run cmd/migrate/main.go down
```

### Local Test Environment Setup

**Database Services:**
```yaml
# deployments/docker/docker-compose.test.yml
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: test_user
      POSTGRES_PASSWORD: test_password
      POSTGRES_DB: test_db
    ports:
      - "5432:5432"
      
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
```

**Environment Variables for Testing:**
```bash
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=test_user
POSTGRES_PASSWORD=test_password
POSTGRES_DB=test_db
REDIS_HOST=localhost
REDIS_PORT=6379
```

---

## Test Organization

### Directory Structure
```
test/
├── unit/              # Fast, isolated unit tests
│   ├── ai_service_test.go
│   ├── conversation_service_test.go
│   └── ...
├── integration/       # Service integration tests
│   ├── database_test.go
│   ├── redis_test.go
│   └── ...
├── e2e/              # End-to-end system tests
│   ├── api_flow_test.go
│   └── ...
├── security/         # Security-focused tests
│   ├── auth_test.go
│   └── ...
├── performance/      # Performance and benchmark tests
│   ├── load_test.go
│   └── ...
└── testutils/        # Shared test utilities
    ├── database.go
    ├── fixtures.go
    └── ...
```

### Test Categories

**Unit Tests (`./test/unit/...`)**
- **Purpose**: Test individual components in isolation
- **Speed**: Fast (<5 minutes total)
- **Dependencies**: Mocked/stubbed
- **Coverage**: Core business logic, data structures, algorithms
- **Build Tags**: None (always run)
- **Examples**: AI service logic, conversation handling, data validation

**Integration Tests (`./test/integration/...`)**
- **Purpose**: Test component interactions with real services
- **Speed**: Medium (10-20 minutes)
- **Dependencies**: PostgreSQL, Redis services
- **Coverage**: Database operations, message bus, external APIs
- **Build Tags**: `//go:build integration`
- **Examples**: Database migrations, Redis pub/sub, service integrations

**E2E Tests (`./test/e2e/...`)**
- **Speed**: Slow (20-30 minutes)
- **Dependencies**: Full application stack
- **Coverage**: Complete user journeys, API workflows
- **Build Tags**: `//go:build e2e`
- **Examples**: User registration → recipe creation → sharing workflow

**Security Tests (`./test/security/...`)**
- **Purpose**: Validate security controls and vulnerabilities
- **Speed**: Medium (5-15 minutes)
- **Dependencies**: Varies by test
- **Coverage**: Authentication, authorization, input validation, XSS/injection prevention
- **Build Tags**: `//go:build security`
- **Examples**: Auth boundary tests, input sanitization, rate limiting

**Performance Tests (`./test/performance/...`)**
- **Purpose**: Validate performance characteristics and benchmarks
- **Speed**: Medium (10-15 minutes)
- **Dependencies**: Minimal (mostly synthetic data)
- **Coverage**: Response times, throughput, memory usage
- **Build Tags**: `//go:build performance`
- **Examples**: API response time, database query performance, memory allocation patterns

---

## CI Pipeline Structure

### Workflow Files
- **Primary**: `.github/workflows/ci-main.yml`
- **Reusable**: `.github/workflows/_reusable-build-test.yml`
- **Security**: `.github/workflows/security-main.yml`

### Trigger Conditions

**ci-main.yml Triggers:**
```yaml
on:
  push:
    branches: [ main, develop, "feature/*" ]
  pull_request:
    branches: [ main, develop ]
  workflow_dispatch:
```

**Branch-Specific Behavior:**

| Branch Type | Quick Validation | Unit Tests | Integration Tests | Security Tests | Performance Tests | E2E Tests |
|-------------|------------------|------------|-------------------|----------------|-------------------|-----------|
| feature/*   | ✅               | ✅         | ❌                | ❌             | ❌                | ❌        |
| develop     | ✅               | ✅         | ✅                | ❌*            | ✅                | ❌        |
| main        | ✅               | ✅         | ✅                | ❌*            | ✅                | ✅        |
| PR → main   | ✅               | ✅         | ✅                | ❌*            | ❌                | ❌        |
| PR → develop| ✅               | ✅         | ✅                | ❌*            | ❌                | ❌        |

*Security tests are temporarily disabled while fixing test setup

### CI Jobs Execution Order

**Tier 1: Quick Validation (5 minutes)**
```yaml
quick-validation:
  - Checkout code
  - Setup Go 1.23
  - Download dependencies
  - Run go vet ./...
  - Check code formatting (gofmt)
  - Build check (go build -v ./...)
```

**Tier 2: Parallel Testing (runs after quick-validation)**
```yaml
# Runs in parallel:
unit-tests:           # 10 minutes, no services
static-analysis:      # 10 minutes, staticcheck + gosec
healthcheck-tests:    # 10 minutes, specific to healthcheck changes
```

**Tier 3: Advanced Testing (runs after unit tests)**
```yaml
# Branch-conditional, runs in parallel:
integration-tests:    # 20 minutes, PostgreSQL + Redis services
performance-tests:    # 15 minutes, benchmarks and load tests
```

**Tier 4: Full System Testing (main branch only)**
```yaml
e2e-tests:           # 30 minutes, full stack with services
```

**Tier 5: Summary**
```yaml
test-summary:        # Consolidates all results, creates GitHub summary
```

### Reusable Test Workflow

**File**: `.github/workflows/_reusable-build-test.yml`

**Inputs:**
```yaml
inputs:
  test_type: "unit|integration|e2e|security|performance"
  go_version: "1.23" (default)
  timeout_minutes: 10 (default)
  enable_services: false (default)
```

**Service Management:**
- **With Services**: PostgreSQL 15-alpine + Redis 7-alpine
- **Without Services**: Isolated test execution
- **Health Checks**: Built-in service readiness validation
- **Port Mapping**: Standard ports (5432, 6379)

**Test Execution Logic:**
```yaml
# Service-specific test commands:
unit:         go test -short -v -race -coverprofile=coverage.out ./test/unit/...
integration:  go test -v -race -tags=integration -run="TestIntegration" ./test/integration/...
e2e:          go test -v -tags=e2e -run="TestE2E" ./test/e2e/...
security:     go test -v -tags=security ./test/security/...
performance:  go test -v -tags=performance -run="TestPerformance" ./test/performance/...
              go test -bench=. -benchmem -count=3 -run=^$ ./pkg/healthcheck/
```

**Outputs:**
- Test results (pass/fail)
- Coverage reports (HTML + data)
- Benchmark results (performance tests)
- Artifacts uploaded to GitHub

### Static Analysis Integration

**Tools Used:**
```yaml
staticcheck:  # Go code quality analysis
  - Install: honnef.co/go/tools/cmd/staticcheck@latest
  - Command: staticcheck ./...
  - Output: Terminal (continue-on-error: true)

gosec:       # Security vulnerability scanning
  - Install: github.com/securego/gosec/v2/cmd/gosec@v2.20.0
  - Command: gosec -fmt sarif -out gosec-results.sarif ./...
  - Output: SARIF format uploaded to GitHub Security tab
```

**SARIF Integration:**
- Security findings uploaded to GitHub Security tab
- Integration with GitHub Advanced Security
- Automated vulnerability tracking
- PR-level security feedback

---

## CD Pipeline Structure

### Workflow File
**Primary**: `.github/workflows/cd-main.yml`

### Trigger Conditions
```yaml
on:
  push:
    branches: [main]      # Automatic staging deployment
    tags: ['v*']          # Production deployment on version tags
  workflow_dispatch:      # Manual deployment
    inputs:
      environment: staging|production
```

### CD Pipeline Jobs

**1. Full CI Pipeline Validation**
```yaml
ci-pipeline:
  uses: ./.github/workflows/ci-main.yml
  # Ensures all tests pass before deployment
```

**2. Container Image Build**
```yaml
build-image:
  - Checkout code
  - Setup Docker Buildx
  - Login to ghcr.io (GitHub Container Registry)
  - Extract metadata (tags, labels)
  - Build multi-stage production image
  - Push to registry with caching
  - Output: image URL + digest
```

**Container Registry**: `ghcr.io/alchemorsel/v3`
**Image Tags**:
- `main` - Latest main branch
- `v1.2.3` - Semantic version tags
- `main-abc123` - Branch + commit SHA

**3. Quality Gates**
```yaml
quality-gates:
  uses: ./.github/workflows/quality-gates.yml
  with:
    environment: staging|production
    image_tag: ${{ needs.build-image.outputs.image }}
  secrets:
    SONAR_TOKEN: ${{ secrets.SONAR_TOKEN }}
    DATADOG_API_KEY: ${{ secrets.DATADOG_API_KEY }}
```

**4. Staging Deployment**
```yaml
deploy-staging:
  if: main branch OR manual staging selection
  environment: staging  # GitHub environment protection
  steps:
    - Deploy container to staging
    - Run smoke tests
    - Update deployment status
```

**5. Production Deployment**
```yaml
deploy-production:
  if: version tag (v*) OR manual production selection
  environment: production  # Requires manual approval
  needs: [build-image, quality-gates, deploy-staging]
  steps:
    - Deploy container to production
    - Run production health checks
    - Create GitHub release (for version tags)
    - Update deployment status
```

**6. Deployment Summary**
```yaml
deployment-summary:
  if: always()
  steps:
    - Generate deployment summary
    - Create GitHub step summary
    - Report deployment status
```

### Environment Protection Rules

**Staging Environment:**
- **Auto-deployment**: On main branch pushes
- **Approval**: Not required
- **Secrets**: Staging database, APIs
- **Smoke Tests**: Basic functionality validation

**Production Environment:**
- **Manual Approval**: Required for all deployments
- **Reviewers**: Team leads, DevOps team
- **Secrets**: Production database, APIs, monitoring
- **Health Checks**: Comprehensive system validation
- **Rollback**: Automated on health check failures

---

## Security Scanning Pipeline

### Workflow File
**Primary**: `.github/workflows/security-main.yml`

### Trigger Schedule
```yaml
on:
  schedule:
    - cron: '0 2 * * *'    # Daily at 2 AM UTC
  push:
    branches: [main, develop]
    paths: [go.mod, go.sum, '**/*.go', deployments/**, .github/workflows/security-main.yml]
  pull_request:
    branches: [main, develop]
  workflow_dispatch:
```

### Security Scanning Jobs

**1. Code Security Scanning**
```yaml
security-scan:
  timeout-minutes: 20
  permissions:
    security-events: write  # For SARIF uploads
  tools:
    - Nancy: Go dependency vulnerability scanner
    - OSV-Scanner: Open Source Vulnerability database
    - Gosec: Static Application Security Testing (SAST)
    - CodeQL: GitHub's semantic code analysis
    - TruffleHog: Secret detection
    - go-licenses: License compliance
```

**Dependency Scanning:**
```bash
# Nancy (Sonatype)
go list -json -deps ./... | nancy sleuth

# OSV-Scanner (Google)
osv-scanner -r --skip-git .
```

**Static Application Security Testing:**
```bash
# Gosec
gosec -fmt sarif -out gosec.sarif ./...
# → Uploaded to GitHub Security tab

# CodeQL
# → Full semantic analysis with security-and-quality queries
```

**Secret Scanning:**
```bash
# TruffleHog
trufflehog --path ./ --base main --head HEAD --debug --only-verified
```

**License Compliance:**
```bash
# go-licenses
go-licenses check ./...
go-licenses report ./... > licenses-report.txt
# Check for prohibited licenses: GPL, AGPL, LGPL
```

**2. Container Security Scanning**
```yaml
container-security:
  if: not scheduled (only for pushes/PRs)
  tools:
    - Trivy: Container vulnerability scanning
    - Syft: Software Bill of Materials (SBOM)
  steps:
    - Build production Docker image
    - Scan with Trivy (CRITICAL,HIGH,MEDIUM)
    - Generate SBOM in SPDX format
    - Upload SARIF results
```

**Container Scanning:**
```bash
# Build image
docker build -t alchemorsel:security-scan -f deployments/docker/Dockerfile.production .

# Trivy scan
trivy image --format sarif --output trivy-container.sarif --severity CRITICAL,HIGH,MEDIUM alchemorsel:security-scan

# SBOM generation
syft alchemorsel:security-scan -o spdx-json=sbom.spdx.json
```

**3. Infrastructure Security Scanning**
```yaml
infrastructure-security:
  tools:
    - Checkov: Terraform security scanner
    - Checkov: Kubernetes manifest scanner
    - kube-score: Kubernetes security best practices
  paths:
    - deployments/terraform/**
    - deployments/kubernetes/**
```

**Infrastructure Scanning:**
```bash
# Terraform scanning
checkov -d deployments/terraform --framework terraform --output-format sarif

# Kubernetes scanning
checkov -d deployments/kubernetes --framework kubernetes --output-format sarif

# kube-score
kube-score score deployments/kubernetes/*.yaml
```

**4. Security Summary**
```yaml
security-summary:
  needs: [security-scan, container-security, infrastructure-security]
  if: always()
  steps:
    - Generate security summary report
    - Upload artifacts (reports, SBOMs, licenses)
    - Fail pipeline if critical issues found
```

### SARIF Integration

**Uploaded to GitHub Security Tab:**
- Gosec security findings
- CodeQL analysis results  
- Trivy container vulnerabilities
- Checkov infrastructure issues

**Benefits:**
- Centralized security dashboard
- PR-level security feedback
- Historical vulnerability tracking
- Integration with dependabot

---

## Testing Matrix

### Comprehensive Test Coverage

| Test Type | Scope | Dependencies | Duration | Triggers | Coverage |
|-----------|-------|--------------|----------|----------|----------|
| **Quick Validation** | Code quality | None | 5 min | All pushes | Formatting, vet, build |
| **Unit Tests** | Business logic | Mocked | 10 min | All pushes | Core algorithms, data structures |
| **Integration Tests** | Service integration | PostgreSQL, Redis | 20 min | PR, main, develop | Database, messaging, APIs |
| **E2E Tests** | Complete workflows | Full stack | 30 min | main only | User journeys, API flows |
| **Security Tests** | Security controls | Varies | 10 min | Currently disabled | Auth, validation, boundaries |
| **Performance Tests** | Performance characteristics | Minimal | 15 min | main, develop | Response times, throughput |
| **Health Check Tests** | Specific component | None | 10 min | healthcheck changes | Health endpoints, monitoring |
| **Static Analysis** | Code quality & security | None | 10 min | All pushes | Code smells, vulnerabilities |

### Branch-Specific Test Execution

**Feature Branches (`feature/*`)**:
- ✅ Quick validation (formatting, build)
- ✅ Unit tests (business logic)
- ✅ Static analysis (code quality)
- ✅ Health check tests (if relevant files changed)
- ❌ Integration tests (too slow for rapid feedback)
- ❌ Security tests (disabled during setup)
- ❌ Performance tests (not needed for features)
- ❌ E2E tests (expensive, not needed)

**Develop Branch**:
- ✅ All feature branch tests
- ✅ Integration tests (validate component interactions)
- ✅ Performance tests (catch regressions early)
- ❌ E2E tests (reserved for main)

**Main Branch**:
- ✅ All develop branch tests
- ✅ E2E tests (full system validation)
- ✅ Complete test suite before deployment

**Pull Requests**:
- ✅ Quick validation + unit + static analysis
- ✅ Integration tests (for PR → main/develop)
- ❌ Performance/E2E (unless specifically requested)

### Test Failure Handling

**Quick Validation Failures**:
- **Impact**: Blocks entire pipeline
- **Message**: "Fix formatting/build issues first"
- **Action**: Developer must fix formatting, vet issues, build errors

**Unit Test Failures**:
- **Impact**: Blocks advanced testing
- **Action**: Fix business logic issues before proceeding

**Integration Test Failures**:
- **Impact**: Blocks deployment (for main/develop)
- **Common Issues**: Database schema, service connectivity
- **Action**: Fix service integration issues

**E2E Test Failures**:
- **Impact**: Blocks production deployment
- **Common Issues**: Complete workflow breakage
- **Action**: Fix user-facing functionality

---

## Build Processes

### Local Build Process

**Development Build:**
```bash
# Standard development build
go build -v ./...

# Build specific components
go build -o bin/alchemorsel cmd/api/main.go
go build -o bin/migrate cmd/migrate/main.go

# Cross-platform builds
GOOS=linux GOARCH=amd64 go build -o bin/alchemorsel-linux cmd/api/main.go
```

### CI Build Process

**Quick Validation Build:**
```bash
# Performed in every CI run
go build -v ./...
```

**No artifact creation** - just validation that code compiles

### CD Build Process (Production)

**Multi-Stage Docker Build** (`deployments/docker/Dockerfile.production`):

**Stage 1: Go Application Build**
```dockerfile
FROM golang:1.23-alpine3.20 AS go-builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev

# Build with optimizations and static linking
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags="-s -w \
              -X main.Version=${VERSION} \
              -X main.Commit=${COMMIT} \
              -X main.BuildTime=${BUILD_TIME} \
              -extldflags '-static'" \
    -o alchemorsel \
    cmd/api/main.go

# Build migration tool
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags="-s -w -extldflags '-static'" \
    -o migrate \
    cmd/migrate/main.go
```

**Stage 2: Frontend Assets**
```dockerfile
FROM node:20-alpine3.20 AS frontend-builder

# Build production frontend assets
RUN npm ci --only=production
RUN npm run build:production
```

**Stage 3: Security Scanner**
```dockerfile
FROM alpine:3.20 AS security-scanner

# Install and run Trivy scanner
RUN curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin
RUN trivy filesystem --exit-code 1 --severity CRITICAL /tmp/
```

**Stage 4: Final Production Image**
```dockerfile
FROM gcr.io/distroless/static-debian11:nonroot

# Copy binaries and assets
COPY --from=go-builder --chown=65532:65532 /app/alchemorsel /app/migrate /app/
COPY --from=frontend-builder --chown=65532:65532 /app/dist /app/static/
COPY --from=go-builder --chown=65532:65532 /app/config /app/config/

# Non-root user, minimal attack surface
USER 65532:65532
EXPOSE 3010
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD ["/app/alchemorsel", "healthcheck"]
ENTRYPOINT ["/app/alchemorsel"]
CMD ["serve"]
```

**Build Optimizations:**
- **Static Linking**: No dynamic dependencies
- **Strip Symbols**: Reduce binary size with `-s -w`
- **Distroless Base**: Minimal attack surface
- **Multi-stage**: Separate build and runtime environments
- **Security Scanning**: Fail build on critical vulnerabilities
- **Non-root User**: Enhanced container security

**Build Arguments:**
```bash
--build-arg VERSION=v1.0.0
--build-arg COMMIT=abc123
--build-arg BUILD_TIME=2024-01-01T00:00:00Z
--build-arg GO_VERSION=1.23
--build-arg NODE_VERSION=20
--build-arg ALPINE_VERSION=3.20
--build-arg DISTROLESS_VERSION=nonroot
```

### Container Registry Strategy

**Registry**: `ghcr.io` (GitHub Container Registry)
**Repository**: `ghcr.io/alchemorsel/v3`

**Tagging Strategy:**
```bash
# Branch-based tags
main → ghcr.io/alchemorsel/v3:main
develop → ghcr.io/alchemorsel/v3:develop

# Version tags  
v1.0.0 → ghcr.io/alchemorsel/v3:v1.0.0
         ghcr.io/alchemorsel/v3:1.0
         ghcr.io/alchemorsel/v3:1

# SHA-based tags
main-abc123 → ghcr.io/alchemorsel/v3:main-abc123
```

**Caching Strategy:**
```yaml
cache-from: type=gha  # GitHub Actions cache
cache-to: type=gha,mode=max
```

**Image Metadata:**
```yaml
labels:
  - org.opencontainers.image.title=Alchemorsel v3
  - org.opencontainers.image.description=Enterprise AI-powered recipe platform
  - org.opencontainers.image.version=${VERSION}
  - org.opencontainers.image.created=${BUILD_TIME}
  - org.opencontainers.image.revision=${COMMIT}
  - org.opencontainers.image.vendor=Alchemorsel
  - org.opencontainers.image.licenses=MIT
  - org.opencontainers.image.source=https://github.com/alchemorsel/v3
```

---

## Quality Gates

### Pre-Deployment Quality Gates

**Implemented in**: `.github/workflows/quality-gates.yml`

**Quality Gate Checks:**
1. **Code Quality Assessment**
   - SonarQube analysis (if SONAR_TOKEN provided)
   - Code coverage thresholds
   - Technical debt assessment
   - Code duplication analysis

2. **Security Validation**
   - All security scans must pass
   - No critical vulnerabilities in dependencies
   - Container security validation
   - Infrastructure security compliance

3. **Performance Validation**
   - Performance test results within acceptable ranges
   - No significant performance regressions
   - Resource usage validation
   - Load test benchmarks

4. **Integration Validation**
   - All integration tests must pass
   - External service connectivity validated
   - Database migration compatibility
   - Service mesh integration

**Quality Gate Outputs:**
```yaml
outputs:
  quality_status: pass|fail
  sonar_status: pass|fail|skipped
  security_status: pass|fail
  performance_status: pass|fail
```

**Failure Handling:**
- **Critical Issues**: Block deployment completely
- **Warning Issues**: Allow deployment with notifications
- **Performance Regressions**: Configurable thresholds

### Continuous Quality Monitoring

**Monitoring Integration** (if DATADOG_API_KEY provided):
- Real-time performance metrics
- Error rate monitoring
- Security event tracking
- Infrastructure health monitoring

**Quality Metrics Tracked:**
- **Code Coverage**: Minimum thresholds per test type
- **Security Score**: Vulnerability count and severity
- **Performance Score**: Response time and throughput benchmarks
- **Reliability Score**: Error rates and uptime metrics

---

## Deployment Strategy

### Environment Strategy

**Three-Tier Deployment:**

1. **Development Environment**
   - **Trigger**: Feature branch development
   - **Purpose**: Developer testing and validation
   - **Infrastructure**: Docker Compose locally
   - **Database**: PostgreSQL test instance
   - **Monitoring**: Basic logging

2. **Staging Environment**
   - **Trigger**: Main branch pushes
   - **Purpose**: Pre-production validation
   - **Infrastructure**: Kubernetes/Docker production-like
   - **Database**: PostgreSQL staging (production-like data)
   - **Monitoring**: Full monitoring stack
   - **Tests**: Smoke tests, integration validation

3. **Production Environment**
   - **Trigger**: Version tags (`v*`) or manual approval
   - **Purpose**: Live customer-facing application
   - **Infrastructure**: High-availability Kubernetes/Docker
   - **Database**: PostgreSQL production cluster
   - **Monitoring**: Complete observability stack
   - **Tests**: Health checks, comprehensive monitoring

### Deployment Process Flow

**Staging Deployment:**
```yaml
deploy-staging:
  environment: staging
  steps:
    1. Deploy container: ${{ image }}
    2. Wait for service readiness
    3. Run smoke tests
    4. Validate health endpoints
    5. Update deployment status
```

**Production Deployment:**
```yaml
deploy-production:
  environment: production  # Requires manual approval
  needs: [build-image, quality-gates, deploy-staging]
  steps:
    1. Manual approval gate
    2. Deploy container: ${{ image }}
    3. Blue-green deployment strategy
    4. Run production health checks
    5. Monitor key metrics
    6. Create GitHub release (for version tags)
    7. Update deployment status
```

### Rollback Strategy

**Automatic Rollback Triggers:**
- Health check failures
- Error rate exceeds threshold
- Performance degradation
- Critical security alerts

**Rollback Process:**
1. **Immediate**: Stop new deployments
2. **Rollback**: Deploy previous known-good version
3. **Validation**: Run health checks on rolled-back version
4. **Notification**: Alert team of rollback event
5. **Analysis**: Begin post-incident analysis

### Release Management

**Version Tag Strategy:**
```bash
# Semantic versioning
v1.0.0 - Major release
v1.1.0 - Minor release  
v1.1.1 - Patch release
```

**Release Process:**
1. **Tag Creation**: Create version tag (`git tag v1.0.0`)
2. **Automatic Trigger**: CD pipeline triggers on tag push
3. **Full Validation**: Complete CI + quality gates
4. **Staging Deployment**: Deploy and validate in staging
5. **Manual Approval**: Production deployment approval
6. **Production Deployment**: Deploy to production
7. **Release Creation**: Automatic GitHub release creation
8. **Monitoring**: Post-deployment monitoring and validation

**Release Notes** (automatically generated):
```markdown
## Release v1.0.0

### Changes
Deployed image: `ghcr.io/alchemorsel/v3:v1.0.0`

### Deployment Details
- Environment: Production
- Deployment time: 2024-01-01T12:00:00Z
- Deployed by: github-user
- Commit: abc123def456

### Quality Gates
- ✅ Code Quality: Passed
- ✅ Security Scan: Passed
- ✅ Performance Tests: Passed
- ✅ Integration Tests: Passed
```

### Deployment Monitoring

**Key Metrics Monitored:**
- **Application Health**: Health endpoint responses
- **Database Connectivity**: Connection pool status
- **External Services**: API dependency status
- **Performance**: Response times, throughput
- **Errors**: Error rates, exception tracking
- **Infrastructure**: CPU, memory, disk usage

**Monitoring Tools Integration:**
- **Health Checks**: Built-in application health endpoints
- **Metrics**: Prometheus/Datadog integration
- **Logging**: Structured logging with correlation IDs
- **Alerting**: Critical issue notifications
- **Dashboards**: Real-time operational visibility

**Success Criteria:**
- ✅ Health endpoints return 200 OK
- ✅ Error rate < 1%
- ✅ Response time < 500ms (95th percentile)
- ✅ Database connectivity stable
- ✅ No critical security alerts
- ✅ Resource usage within normal ranges

---

## Summary

This comprehensive CI/CD pipeline provides:

**✅ Multi-layered Testing**
- Quick feedback with unit tests
- Thorough integration validation  
- Complete system E2E testing
- Security and performance validation

**✅ Security-First Approach**
- Daily security scanning
- Container vulnerability assessment
- Infrastructure security validation
- Secret detection and license compliance

**✅ Quality Assurance**
- Automated quality gates
- Code coverage requirements
- Performance regression detection
- Manual approval gates for production

**✅ Reliable Deployment**
- Multi-environment strategy
- Blue-green deployment patterns
- Automatic rollback capabilities
- Comprehensive monitoring

**✅ Developer Experience**
- Fast feedback on feature branches
- Clear test failure reporting
- Comprehensive documentation
- Standardized tooling (Go 1.23)

The pipeline balances speed and safety, providing rapid feedback for developers while ensuring production deployments meet the highest quality and security standards.