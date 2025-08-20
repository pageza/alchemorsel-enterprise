# ADR-0003: Docker Compose Architecture

## Status
Accepted

## Context
Alchemorsel v3 consists of multiple interconnected services (web application, database, cache, AI services) that need to work together in both development and production environments. We need a consistent, reproducible way to orchestrate these services while maintaining development velocity and production reliability.

Requirements:
- Consistent environments across development, testing, and production
- Simple local development setup for new team members
- Service isolation and dependency management
- Environment-specific configuration management
- Easy scaling and service replacement

## Decision
We will use Docker Compose as the primary orchestration tool for all Alchemorsel v3 environments.

**Implementation Requirements:**
- Primary `docker-compose.yml` for production configuration
- `docker-compose.dev.yml` override for development
- `docker-compose.test.yml` override for testing
- All services must be containerized with proper health checks
- Environment variables must be externalized via `.env` files
- Named volumes for data persistence
- Custom networks for service isolation

**Service Architecture:**
```
alchemorsel-network:
  - web (Go application)
  - postgres (PostgreSQL 15+)
  - redis (Redis 7+)
  - ollama (AI service)
  - nginx (reverse proxy, production only)
```

## Consequences

### Positive
- Identical environments across all deployment targets
- One-command setup for new developers (`docker-compose up`)
- Service isolation prevents dependency conflicts
- Easy horizontal scaling with `docker-compose scale`
- Built-in service discovery and networking
- Simplified CI/CD with consistent container interface
- Environment-specific overrides maintain flexibility

### Negative
- Additional Docker knowledge required for all team members
- Resource overhead of containerization in development
- Potential networking complexity for advanced use cases
- Docker Compose limitations for complex production orchestration

### Neutral
- Migration path to Kubernetes available if needed
- Local development performance comparable to native
- Log aggregation simplified with centralized container logs