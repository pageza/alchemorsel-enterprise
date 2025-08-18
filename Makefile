# Alchemorsel v3 - Enterprise-Grade Makefile
# Comprehensive build, test, and deployment automation

# Variables
APP_NAME := alchemorsel-v3
VERSION ?= $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD)
GO_VERSION := $(shell go version | awk '{print $$3}')

# Directories
BUILD_DIR := build
COVERAGE_DIR := coverage
DOCS_DIR := docs
SCRIPTS_DIR := scripts

# Go build flags
LDFLAGS := -X main.version=$(VERSION) \
          -X main.buildTime=$(BUILD_TIME) \
          -X main.gitCommit=$(GIT_COMMIT) \
          -X main.goVersion=$(GO_VERSION)

# Docker
DOCKER_REPO ?= alchemorsel
DOCKER_TAG ?= $(VERSION)
DOCKER_IMAGE := $(DOCKER_REPO)/$(APP_NAME):$(DOCKER_TAG)

# Test configuration
TEST_TIMEOUT := 30m
INTEGRATION_TEST_TIMEOUT := 45m
COVERAGE_THRESHOLD := 80
BENCHMARK_COUNT := 5

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
PURPLE := \033[0;35m
CYAN := \033[0;36m
WHITE := \033[0;37m
RESET := \033[0m

.PHONY: help
help: ## Display this help message
	@echo "$(CYAN)Alchemorsel v3 - Enterprise-Grade Recipe Platform$(RESET)"
	@echo "$(YELLOW)Version: $(VERSION)$(RESET)"
	@echo ""
	@echo "$(GREEN)Available targets:$(RESET)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(CYAN)%-20s$(RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development Commands

.PHONY: dev
dev: ## Start development server with hot reload
	@echo "$(GREEN)Starting development server...$(RESET)"
	air -c .air.toml

.PHONY: build
build: clean ## Build the application
	@echo "$(GREEN)Building $(APP_NAME)...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(APP_NAME) \
		./cmd/api

.PHONY: build-all
build-all: clean ## Build for all platforms
	@echo "$(GREEN)Building for all platforms...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ "$$os" = "windows" ] && [ "$$arch" = "arm64" ]; then continue; fi; \
			echo "$(BLUE)Building $$os/$$arch...$(RESET)"; \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build \
				-ldflags "$(LDFLAGS)" \
				-o $(BUILD_DIR)/$(APP_NAME)-$$os-$$arch \
				./cmd/api; \
		done; \
	done

.PHONY: install-tools
install-tools: ## Install development tools
	@echo "$(GREEN)Installing development tools...$(RESET)"
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golang/mock/mockgen@latest
	go install golang.org/x/perf/cmd/benchstat@latest

# Testing Commands

.PHONY: test
test: ## Run unit tests
	@echo "$(GREEN)Running unit tests...$(RESET)"
	go test -race -timeout $(TEST_TIMEOUT) ./...

.PHONY: test-unit
test-unit: ## Run only unit tests (exclude integration tests)
	@echo "$(GREEN)Running unit tests only...$(RESET)"
	go test -race -short -timeout $(TEST_TIMEOUT) ./...

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "$(GREEN)Running integration tests...$(RESET)"
	go test -race -tags=integration -timeout $(INTEGRATION_TEST_TIMEOUT) ./test/integration/...

.PHONY: test-security
test-security: ## Run security tests
	@echo "$(GREEN)Running security tests...$(RESET)"
	go test -race -tags=security -timeout $(TEST_TIMEOUT) ./test/security/...

.PHONY: test-performance
test-performance: ## Run performance tests
	@echo "$(GREEN)Running performance tests...$(RESET)"
	go test -tags=performance -timeout $(TEST_TIMEOUT) ./test/performance/...

.PHONY: test-all
test-all: test-unit test-integration test-security test-performance ## Run all tests

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "$(GREEN)Running tests with coverage...$(RESET)"
	@mkdir -p $(COVERAGE_DIR)
	go test -race -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	go tool cover -func=$(COVERAGE_DIR)/coverage.out | grep total | awk '{print "Total coverage: " $$3}'

.PHONY: test-coverage-check
test-coverage-check: test-coverage ## Check if coverage meets threshold
	@echo "$(GREEN)Checking coverage threshold ($(COVERAGE_THRESHOLD)%)...$(RESET)"
	@COVERAGE=$$(go tool cover -func=$(COVERAGE_DIR)/coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	if [ "$$(echo "$$COVERAGE < $(COVERAGE_THRESHOLD)" | bc -l)" -eq 1 ]; then \
		echo "$(RED)Coverage $$COVERAGE% is below threshold $(COVERAGE_THRESHOLD)%$(RESET)"; \
		exit 1; \
	else \
		echo "$(GREEN)Coverage $$COVERAGE% meets threshold $(COVERAGE_THRESHOLD)%$(RESET)"; \
	fi

.PHONY: benchmark
benchmark: ## Run benchmarks
	@echo "$(GREEN)Running benchmarks...$(RESET)"
	@mkdir -p $(COVERAGE_DIR)
	go test -bench=. -benchmem -count=$(BENCHMARK_COUNT) -timeout $(TEST_TIMEOUT) ./... | tee $(COVERAGE_DIR)/benchmark.txt

.PHONY: benchmark-compare
benchmark-compare: ## Compare benchmarks with baseline
	@echo "$(GREEN)Comparing benchmarks with baseline...$(RESET)"
	@if [ -f $(COVERAGE_DIR)/benchmark-baseline.txt ]; then \
		benchstat $(COVERAGE_DIR)/benchmark-baseline.txt $(COVERAGE_DIR)/benchmark.txt; \
	else \
		echo "$(YELLOW)No baseline found. Current results saved as baseline.$(RESET)"; \
		cp $(COVERAGE_DIR)/benchmark.txt $(COVERAGE_DIR)/benchmark-baseline.txt; \
	fi

.PHONY: test-watch
test-watch: ## Watch for changes and run tests
	@echo "$(GREEN)Starting test watcher...$(RESET)"
	@while true; do \
		inotifywait -r -e modify --include='\.go$$' .; \
		echo "$(BLUE)Files changed, running tests...$(RESET)"; \
		make test-unit; \
	done

# Quality Assurance Commands

.PHONY: lint
lint: ## Run linter
	@echo "$(GREEN)Running linter...$(RESET)"
	golangci-lint run ./...

.PHONY: lint-fix
lint-fix: ## Run linter and fix issues
	@echo "$(GREEN)Running linter with fixes...$(RESET)"
	golangci-lint run --fix ./...

.PHONY: format
format: ## Format code
	@echo "$(GREEN)Formatting code...$(RESET)"
	go fmt ./...
	goimports -w .

.PHONY: vet
vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(RESET)"
	go vet ./...

.PHONY: security-scan
security-scan: ## Run security scan
	@echo "$(GREEN)Running security scan...$(RESET)"
	gosec ./...

.PHONY: vuln-check
vuln-check: ## Check for known vulnerabilities
	@echo "$(GREEN)Checking for vulnerabilities...$(RESET)"
	govulncheck ./...

.PHONY: quality-check
quality-check: format vet lint security-scan vuln-check ## Run all quality checks

# Documentation Commands

.PHONY: docs
docs: ## Generate documentation
	@echo "$(GREEN)Generating documentation...$(RESET)"
	swag init -g cmd/api/main.go -o api/openapi

.PHONY: docs-serve
docs-serve: docs ## Serve documentation locally
	@echo "$(GREEN)Serving documentation on http://localhost:8080/swagger/index.html$(RESET)"
	go run cmd/api/main.go

# Database Commands

.PHONY: db-migrate
db-migrate: ## Run database migrations
	@echo "$(GREEN)Running database migrations...$(RESET)"
	go run cmd/migrate/main.go up

.PHONY: db-migrate-down
db-migrate-down: ## Rollback database migrations
	@echo "$(GREEN)Rolling back database migrations...$(RESET)"
	go run cmd/migrate/main.go down

.PHONY: db-reset
db-reset: ## Reset database
	@echo "$(GREEN)Resetting database...$(RESET)"
	go run cmd/migrate/main.go reset

.PHONY: db-seed
db-seed: ## Seed database with test data
	@echo "$(GREEN)Seeding database...$(RESET)"
	go run cmd/seed/main.go

# Docker Commands

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "$(GREEN)Building Docker image $(DOCKER_IMAGE)...$(RESET)"
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(DOCKER_IMAGE) .

.PHONY: docker-run
docker-run: docker-build ## Run Docker container
	@echo "$(GREEN)Running Docker container...$(RESET)"
	docker run --rm -p 8080:8080 \
		-e DATABASE_URL=postgres://postgres:password@host.docker.internal:5432/alchemorsel \
		-e REDIS_URL=redis://host.docker.internal:6379 \
		$(DOCKER_IMAGE)

.PHONY: docker-push
docker-push: docker-build ## Push Docker image
	@echo "$(GREEN)Pushing Docker image $(DOCKER_IMAGE)...$(RESET)"
	docker push $(DOCKER_IMAGE)

.PHONY: docker-compose-up
docker-compose-up: ## Start all services with docker-compose
	@echo "$(GREEN)Starting services with docker-compose...$(RESET)"
	docker-compose up -d

.PHONY: docker-compose-down
docker-compose-down: ## Stop all services with docker-compose
	@echo "$(GREEN)Stopping services with docker-compose...$(RESET)"
	docker-compose down

.PHONY: docker-compose-logs
docker-compose-logs: ## View docker-compose logs
	@echo "$(GREEN)Viewing docker-compose logs...$(RESET)"
	docker-compose logs -f

# Deployment Commands

.PHONY: deploy-staging
deploy-staging: ## Deploy to staging environment
	@echo "$(GREEN)Deploying to staging...$(RESET)"
	@echo "$(YELLOW)This would trigger staging deployment pipeline$(RESET)"
	# kubectl apply -f deployments/kubernetes/staging/

.PHONY: deploy-production
deploy-production: ## Deploy to production environment
	@echo "$(RED)Deploying to production...$(RESET)"
	@echo "$(YELLOW)This would trigger production deployment pipeline$(RESET)"
	# kubectl apply -f deployments/kubernetes/production/

# Monitoring Commands

.PHONY: logs
logs: ## View application logs
	@echo "$(GREEN)Viewing application logs...$(RESET)"
	docker-compose logs -f api

.PHONY: metrics
metrics: ## View metrics
	@echo "$(GREEN)Opening metrics dashboard...$(RESET)"
	@echo "$(BLUE)Prometheus: http://localhost:9090$(RESET)"
	@echo "$(BLUE)Grafana: http://localhost:3000$(RESET)"

# Development Environment Commands

.PHONY: env-setup
env-setup: install-tools ## Setup development environment
	@echo "$(GREEN)Setting up development environment...$(RESET)"
	@if [ ! -f .env ]; then \
		echo "$(BLUE)Creating .env file from template...$(RESET)"; \
		cp .env.example .env; \
	fi
	@echo "$(GREEN)Development environment setup complete!$(RESET)"
	@echo "$(YELLOW)Next steps:$(RESET)"
	@echo "  1. Update .env file with your configuration"
	@echo "  2. Run 'make docker-compose-up' to start dependencies"
	@echo "  3. Run 'make db-migrate' to setup database"
	@echo "  4. Run 'make dev' to start development server"

.PHONY: env-check
env-check: ## Check development environment
	@echo "$(GREEN)Checking development environment...$(RESET)"
	@command -v go >/dev/null 2>&1 || { echo "$(RED)Go is not installed$(RESET)"; exit 1; }
	@command -v docker >/dev/null 2>&1 || { echo "$(RED)Docker is not installed$(RESET)"; exit 1; }
	@command -v docker-compose >/dev/null 2>&1 || { echo "$(RED)Docker Compose is not installed$(RESET)"; exit 1; }
	@echo "$(GREEN)Environment check passed!$(RESET)"

# Cleanup Commands

.PHONY: clean
clean: ## Clean build artifacts
	@echo "$(GREEN)Cleaning build artifacts...$(RESET)"
	rm -rf $(BUILD_DIR)
	rm -rf $(COVERAGE_DIR)
	go clean -cache
	go clean -testcache
	go clean -modcache

.PHONY: clean-docker
clean-docker: ## Clean Docker artifacts
	@echo "$(GREEN)Cleaning Docker artifacts...$(RESET)"
	docker system prune -f
	docker volume prune -f

# CI/CD Commands

.PHONY: ci
ci: env-check quality-check test-coverage-check test-integration ## Run CI pipeline
	@echo "$(GREEN)CI pipeline completed successfully!$(RESET)"

.PHONY: ci-fast
ci-fast: quality-check test-unit ## Run fast CI pipeline (for PRs)
	@echo "$(GREEN)Fast CI pipeline completed successfully!$(RESET)"

.PHONY: pre-commit
pre-commit: format lint test-unit ## Run pre-commit checks
	@echo "$(GREEN)Pre-commit checks completed!$(RESET)"

.PHONY: pre-push
pre-push: ci-fast ## Run pre-push checks
	@echo "$(GREEN)Pre-push checks completed!$(RESET)"

# Release Commands

.PHONY: release-check
release-check: ## Check if ready for release
	@echo "$(GREEN)Checking release readiness...$(RESET)"
	@echo "$(BLUE)Running full test suite...$(RESET)"
	make test-all
	@echo "$(BLUE)Running quality checks...$(RESET)"
	make quality-check
	@echo "$(BLUE)Checking coverage...$(RESET)"
	make test-coverage-check
	@echo "$(GREEN)Release checks passed!$(RESET)"

.PHONY: release-notes
release-notes: ## Generate release notes
	@echo "$(GREEN)Generating release notes...$(RESET)"
	@echo "$(YELLOW)This would generate release notes from git commits$(RESET)"
	# git log --oneline --decorate --graph $(shell git describe --tags --abbrev=0)..HEAD

# Performance Commands

.PHONY: profile-cpu
profile-cpu: ## Profile CPU usage
	@echo "$(GREEN)Profiling CPU usage...$(RESET)"
	go test -cpuprofile=cpu.prof -bench=. ./test/performance/
	go tool pprof cpu.prof

.PHONY: profile-memory
profile-memory: ## Profile memory usage
	@echo "$(GREEN)Profiling memory usage...$(RESET)"
	go test -memprofile=mem.prof -bench=. ./test/performance/
	go tool pprof mem.prof

.PHONY: load-test
load-test: ## Run load tests
	@echo "$(GREEN)Running load tests...$(RESET)"
	@echo "$(YELLOW)This would run load tests with k6 or similar tool$(RESET)"
	# k6 run test/load/load-test.js

# Security Commands

.PHONY: security-audit
security-audit: security-scan vuln-check ## Run comprehensive security audit
	@echo "$(GREEN)Security audit completed!$(RESET)"

.PHONY: dependency-check
dependency-check: ## Check dependencies for vulnerabilities
	@echo "$(GREEN)Checking dependencies...$(RESET)"
	go mod download
	go mod verify
	go list -u -m all

# Default target
.DEFAULT_GOAL := help

# Version target
.PHONY: version
version: ## Show version information
	@echo "$(CYAN)Alchemorsel v3$(RESET)"
	@echo "Version: $(GREEN)$(VERSION)$(RESET)"
	@echo "Build Time: $(GREEN)$(BUILD_TIME)$(RESET)"
	@echo "Git Commit: $(GREEN)$(GIT_COMMIT)$(RESET)"
	@echo "Go Version: $(GREEN)$(GO_VERSION)$(RESET)"