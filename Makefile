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
build: clean optimize ## Build the application
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

# 14KB Optimization Commands

.PHONY: optimize
optimize: ## Run 14KB first packet optimization
	@echo "$(GREEN)Running 14KB optimization build...$(RESET)"
	go run cmd/optimize/main.go --command=build --verbose

.PHONY: optimize-analyze
optimize-analyze: ## Analyze 14KB optimization status
	@echo "$(GREEN)Analyzing 14KB optimization status...$(RESET)"
	go run cmd/optimize/main.go --command=analyze

.PHONY: optimize-validate
optimize-validate: ## Validate 14KB compliance
	@echo "$(GREEN)Validating 14KB compliance...$(RESET)"
	go run cmd/optimize/main.go --command=validate

.PHONY: optimize-report
optimize-report: ## Generate 14KB optimization report
	@echo "$(GREEN)Generating 14KB optimization report...$(RESET)"
	go run cmd/optimize/main.go --command=report --format=html

.PHONY: optimize-clean
optimize-clean: ## Clean optimization artifacts
	@echo "$(GREEN)Cleaning optimization artifacts...$(RESET)"
	go run cmd/optimize/main.go --command=clean

.PHONY: optimize-watch
optimize-watch: ## Watch for changes and re-optimize
	@echo "$(GREEN)Starting optimization watcher...$(RESET)"
	go run cmd/optimize/main.go --command=watch

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

# Hot Reload Development Commands

.PHONY: dev-start
dev-start: ## Start complete hot reload development environment
	@echo "$(GREEN)🚀 Starting Alchemorsel v3 Hot Reload Development Environment$(RESET)"
	@echo "$(BLUE)📋 This will start:$(RESET)"
	@echo "  • PostgreSQL database (port 5434)"
	@echo "  • Redis cache (port 6381)"
	@echo "  • Ollama AI service (port 11435)"
	@echo "  • API service with hot reload (port 3010)"
	@echo "  • Web service with hot reload (port 3011)"
	@echo "  • Live reload WebSocket (port 35729)"
	@echo "  • Development proxy (port 8090)"
	@echo "  • Development dashboard (port 3030)"
	@echo "  • Asset pipeline with hot reload"
	@echo ""
	@echo "$(YELLOW)🌐 Access Points:$(RESET)"
	@echo "  • Main Application: http://localhost:8090"
	@echo "  • Development Dashboard: http://localhost:3030"
	@echo "  • API Direct: http://localhost:3010"
	@echo "  • Web Direct: http://localhost:3011"
	@echo "  • API Documentation: http://localhost:8090/swagger/"
	@echo ""
	docker-compose -f docker-compose.hotreload.yml up -d
	@echo "$(GREEN)✅ Hot reload environment started successfully!$(RESET)"
	@echo "$(BLUE)💡 Run 'make dev-logs' to view logs$(RESET)"

.PHONY: dev-stop
dev-stop: ## Stop hot reload development environment
	@echo "$(GREEN)🛑 Stopping hot reload development environment...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml down
	@echo "$(GREEN)✅ Development environment stopped$(RESET)"

.PHONY: dev-restart
dev-restart: ## Restart hot reload development environment
	@echo "$(GREEN)🔄 Restarting hot reload development environment...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml restart
	@echo "$(GREEN)✅ Development environment restarted$(RESET)"

.PHONY: dev-logs
dev-logs: ## View development environment logs
	@echo "$(GREEN)📋 Viewing development environment logs (Ctrl+C to exit)...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml logs -f

.PHONY: dev-logs-api
dev-logs-api: ## View API service logs
	@echo "$(GREEN)📋 Viewing API service logs...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml logs -f api-dev

.PHONY: dev-logs-web
dev-logs-web: ## View web service logs
	@echo "$(GREEN)📋 Viewing web service logs...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml logs -f web-dev

.PHONY: dev-logs-proxy
dev-logs-proxy: ## View proxy logs
	@echo "$(GREEN)📋 Viewing proxy logs...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml logs -f dev-proxy

.PHONY: dev-status
dev-status: ## Check development environment status
	@echo "$(GREEN)📊 Development Environment Status$(RESET)"
	@echo ""
	@echo "$(BLUE)🐳 Docker Containers:$(RESET)"
	docker-compose -f docker-compose.hotreload.yml ps
	@echo ""
	@echo "$(BLUE)🌐 Service Health Checks:$(RESET)"
	@curl -s http://localhost:8090/dev-status | python3 -m json.tool || echo "Proxy not responding"
	@echo ""
	@curl -s http://localhost:3010/health | python3 -m json.tool || echo "API service not responding"
	@echo ""
	@curl -s http://localhost:3011/health | python3 -m json.tool || echo "Web service not responding"

.PHONY: dev-dashboard
dev-dashboard: ## Open development dashboard in browser
	@echo "$(GREEN)🚀 Opening development dashboard...$(RESET)"
	@which xdg-open > /dev/null && xdg-open http://localhost:3030 || \
	 which open > /dev/null && open http://localhost:3030 || \
	 echo "$(YELLOW)Please open http://localhost:3030 in your browser$(RESET)"

.PHONY: dev-build
dev-build: ## Build development Docker images
	@echo "$(GREEN)🔨 Building development Docker images...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml build --parallel

.PHONY: dev-pull
dev-pull: ## Pull latest development images
	@echo "$(GREEN)📥 Pulling latest development images...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml pull

.PHONY: dev-clean
dev-clean: ## Clean development environment
	@echo "$(GREEN)🧹 Cleaning development environment...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml down -v --remove-orphans
	docker system prune -f
	@echo "$(GREEN)✅ Development environment cleaned$(RESET)"

.PHONY: dev-reset
dev-reset: dev-clean dev-build dev-start ## Reset development environment completely

.PHONY: dev-shell-api
dev-shell-api: ## Open shell in API development container
	@echo "$(GREEN)🐚 Opening shell in API development container...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml exec api-dev /bin/bash

.PHONY: dev-shell-web
dev-shell-web: ## Open shell in Web development container
	@echo "$(GREEN)🐚 Opening shell in Web development container...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml exec web-dev /bin/bash

.PHONY: dev-debug-api
dev-debug-api: ## Start API service with debugger
	@echo "$(GREEN)🐛 Starting API service with debugger...$(RESET)"
	@echo "$(YELLOW)Connect your debugger to localhost:2346$(RESET)"
	docker-compose -f docker-compose.hotreload.yml run --rm --service-ports api-dev /app/debug-dev.sh

.PHONY: dev-debug-web
dev-debug-web: ## Start Web service with debugger
	@echo "$(GREEN)🐛 Starting Web service with debugger...$(RESET)"
	@echo "$(YELLOW)Connect your debugger to localhost:2347$(RESET)"
	docker-compose -f docker-compose.hotreload.yml run --rm --service-ports web-dev /app/debug-dev.sh

.PHONY: dev-test
dev-test: ## Run tests in development environment
	@echo "$(GREEN)🧪 Running tests in development environment...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml run --rm api-dev /app/test-dev.sh

.PHONY: dev-test-watch
dev-test-watch: ## Run tests with file watching
	@echo "$(GREEN)🧪 Starting test watcher...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml up test-runner

.PHONY: dev-migrate
dev-migrate: ## Run database migrations in development
	@echo "$(GREEN)🗃️ Running database migrations...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml run --rm api-dev go run ./cmd/migrate up

.PHONY: dev-migrate-reset
dev-migrate-reset: ## Reset database and run migrations
	@echo "$(GREEN)🗃️ Resetting database and running migrations...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml run --rm api-dev go run ./cmd/migrate reset

.PHONY: dev-seed
dev-seed: ## Seed development database with test data
	@echo "$(GREEN)🌱 Seeding development database...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml run --rm api-dev go run ./cmd/seed

.PHONY: dev-assets-watch
dev-assets-watch: ## Start asset watcher for CSS/JS hot reload
	@echo "$(GREEN)🎨 Starting asset watcher...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml up assets-dev

.PHONY: dev-assets-build
dev-assets-build: ## Build assets for development
	@echo "$(GREEN)🎨 Building assets for development...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml run --rm assets-dev npm run build

.PHONY: dev-reload
dev-reload: ## Manually trigger hot reload
	@echo "$(GREEN)🔄 Triggering hot reload...$(RESET)"
	@curl -X POST http://localhost:3030/api/hotreload/trigger || echo "Dashboard not available"

.PHONY: dev-metrics
dev-metrics: ## View development metrics
	@echo "$(GREEN)📊 Development Metrics$(RESET)"
	@echo ""
	@echo "$(BLUE)Proxy Statistics:$(RESET)"
	@curl -s http://localhost:8090/dev-proxy/stats | python3 -m json.tool || echo "Proxy not responding"
	@echo ""
	@echo "$(BLUE)Live Reload Status:$(RESET)"
	@curl -s http://localhost:35729/status | python3 -m json.tool || echo "Live reload not responding"

.PHONY: dev-health
dev-health: ## Check all service health
	@echo "$(GREEN)🏥 Service Health Check$(RESET)"
	@echo ""
	@services="api:3010 web:3011 proxy:8090 dashboard:3030 livereload:35729"; \
	for service in $$services; do \
		name=$${service%%:*}; \
		port=$${service##*:}; \
		echo -n "$(BLUE)$$name$(RESET): "; \
		if curl -s --max-time 3 http://localhost:$$port/health > /dev/null 2>&1 || \
		   curl -s --max-time 3 http://localhost:$$port > /dev/null 2>&1; then \
			echo "$(GREEN)✅ Healthy$(RESET)"; \
		else \
			echo "$(RED)❌ Unhealthy$(RESET)"; \
		fi; \
	done

.PHONY: dev-ports
dev-ports: ## Show all development ports
	@echo "$(GREEN)🔌 Development Environment Ports$(RESET)"
	@echo ""
	@echo "$(BLUE)Main Services:$(RESET)"
	@echo "  • 8090 - Development Proxy (main entry point)"
	@echo "  • 3010 - API Service (hot reload enabled)"
	@echo "  • 3011 - Web Service (hot reload enabled)"
	@echo "  • 3030 - Development Dashboard"
	@echo ""
	@echo "$(BLUE)Databases & Cache:$(RESET)"
	@echo "  • 5434 - PostgreSQL Database"
	@echo "  • 6381 - Redis Cache"
	@echo "  • 11435 - Ollama AI Service"
	@echo ""
	@echo "$(BLUE)Development Tools:$(RESET)"
	@echo "  • 35729 - Live Reload WebSocket"
	@echo "  • 2346 - API Debugger (Delve)"
	@echo "  • 2347 - Web Debugger (Delve)"
	@echo "  • 6061 - API Profiler (pprof)"
	@echo "  • 9091 - API Metrics"
	@echo "  • 9092 - Web Metrics"

.PHONY: dev-help
dev-help: ## Show development workflow help
	@echo "$(CYAN)🚀 Alchemorsel v3 - Hot Reload Development Workflow$(RESET)"
	@echo ""
	@echo "$(GREEN)Quick Start:$(RESET)"
	@echo "  1. Start development environment:    make dev-start"
	@echo "  2. Open main application:           http://localhost:8090"
	@echo "  3. Open development dashboard:      http://localhost:3030"
	@echo "  4. View logs:                       make dev-logs"
	@echo "  5. Stop environment:                make dev-stop"
	@echo ""
	@echo "$(GREEN)Development Commands:$(RESET)"
	@awk 'BEGIN {FS = ":.*?## "} /^dev-[a-zA-Z_-]+:.*?## / {printf "  $(CYAN)%-25s$(RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(YELLOW)💡 Tips:$(RESET)"
	@echo "  • Code changes trigger automatic rebuilds"
	@echo "  • Templates reload without container restart"
	@echo "  • CSS changes reload instantly"
	@echo "  • Database migrations apply automatically"
	@echo "  • Use the dashboard to monitor all services"
	@echo ""
	@echo "$(BLUE)Debugging:$(RESET)"
	@echo "  • API debugger:  make dev-debug-api  (port 2346)"
	@echo "  • Web debugger:  make dev-debug-web  (port 2347)"
	@echo "  • Service logs:  make dev-logs-api | dev-logs-web"
	@echo "  • Health check:  make dev-health"

# AI Development Commands

.PHONY: dev-ai-models
dev-ai-models: ## List available AI models in development
	@echo "$(GREEN)🤖 Available AI Models$(RESET)"
	@curl -s http://localhost:11435/api/tags | python3 -m json.tool || echo "Ollama not responding"

.PHONY: dev-ai-pull
dev-ai-pull: ## Pull AI model for development (usage: make dev-ai-pull MODEL=llama2)
	@if [ -z "$(MODEL)" ]; then \
		echo "$(RED)❌ Please specify MODEL=<model-name>$(RESET)"; \
		echo "$(BLUE)Example: make dev-ai-pull MODEL=llama2$(RESET)"; \
		exit 1; \
	fi
	@echo "$(GREEN)📥 Pulling AI model: $(MODEL)$(RESET)"
	@curl -X POST http://localhost:11435/api/pull -d '{"name":"$(MODEL)"}' || echo "Ollama not responding"

.PHONY: dev-ai-test
dev-ai-test: ## Test AI service integration
	@echo "$(GREEN)🤖 Testing AI service integration...$(RESET)"
	@curl -s http://localhost:3010/api/ai/health | python3 -m json.tool || echo "AI service not responding"

# Performance Development Commands

.PHONY: dev-benchmark
dev-benchmark: ## Run performance benchmarks in development
	@echo "$(GREEN)⚡ Running performance benchmarks...$(RESET)"
	docker-compose -f docker-compose.hotreload.yml run --rm api-dev go test -bench=. -benchmem ./test/performance/

.PHONY: dev-profile-cpu
dev-profile-cpu: ## Profile CPU usage in development
	@echo "$(GREEN)🔍 Profiling CPU usage...$(RESET)"
	@echo "$(BLUE)Profile will be available at: http://localhost:6061/debug/pprof/$(RESET)"
	@which xdg-open > /dev/null && xdg-open http://localhost:6061/debug/pprof/ || \
	 which open > /dev/null && open http://localhost:6061/debug/pprof/ || \
	 echo "$(YELLOW)Please open http://localhost:6061/debug/pprof/ in your browser$(RESET)"

.PHONY: dev-profile-memory
dev-profile-memory: ## Profile memory usage in development
	@echo "$(GREEN)🔍 Profiling memory usage...$(RESET)"
	@curl -s http://localhost:6061/debug/pprof/heap > heap.prof && \
	go tool pprof heap.prof

.PHONY: dev-load-test
dev-load-test: ## Run load test against development environment
	@echo "$(GREEN)🚀 Running load test...$(RESET)"
	@if command -v hey > /dev/null 2>&1; then \
		hey -n 1000 -c 10 http://localhost:8090/; \
	else \
		echo "$(YELLOW)Installing hey load testing tool...$(RESET)"; \
		go install github.com/rakyll/hey@latest; \
		hey -n 1000 -c 10 http://localhost:8090/; \
	fi

# Version target
.PHONY: version
version: ## Show version information
	@echo "$(CYAN)Alchemorsel v3$(RESET)"
	@echo "Version: $(GREEN)$(VERSION)$(RESET)"
	@echo "Build Time: $(GREEN)$(BUILD_TIME)$(RESET)"
	@echo "Git Commit: $(GREEN)$(GIT_COMMIT)$(RESET)"
	@echo "Go Version: $(GREEN)$(GO_VERSION)$(RESET)"