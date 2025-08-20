# ADR-0005: PortScan Port Management

## Status
Accepted

## Context
Alchemorsel v3 requires a consistent and conflict-free port allocation strategy for all services running in Docker Compose environments. Port conflicts can cause service startup failures and complicate local development when multiple projects or service instances are running simultaneously.

Current service requirements:
- Web application (HTTP/HTTPS)
- PostgreSQL database
- Redis cache
- Ollama AI service
- Development hot-reload services
- Health check endpoints

Considerations:
- Avoid conflicts with common development ports (3000, 8000, 8080, 5432, 6379)
- Reserve port ranges for different service types
- Maintain consistency across team environments
- Support multiple concurrent development instances

## Decision
We will implement a standardized port allocation strategy using the PortScan approach to avoid conflicts and ensure consistent service discovery.

**Port Allocation Strategy:**

**Production Ports (Docker internal):**
- Web Application: 8080 (internal)
- PostgreSQL: 5432 (internal)
- Redis: 6379 (internal)
- Ollama: 11434 (internal)

**Development External Ports:**
- Web Application: 8090 (external mapping)
- PostgreSQL: 5433 (external mapping)
- Redis: 6380 (external mapping)
- Ollama: 11435 (external mapping)
- Hot Reload: 8091 (external mapping)

**Port Scanning Implementation:**
- `docker-compose.dev.yml` must implement port conflict detection
- Services should fail gracefully with clear error messages if ports are unavailable
- Health check endpoints on predictable ports for monitoring
- Environment variables for port customization in local development

## Consequences

### Positive
- Eliminates port conflicts in multi-developer environments
- Consistent service discovery across all environments
- Clear separation between internal and external port mappings
- Supports running multiple project instances simultaneously
- Predictable debugging and monitoring endpoints

### Negative
- Additional complexity in Docker Compose configuration
- Potential confusion between internal and external port numbers
- Port scanning adds minor startup delay
- Requires documentation of port allocation strategy

### Neutral
- Standard Docker networking practices maintained
- No impact on production deployment (internal ports only)
- Compatible with container orchestration migration path