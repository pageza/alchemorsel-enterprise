# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records for Alchemorsel v3, documenting important architectural decisions and their rationale.

## ADR Index

| ADR | Title | Status | Description |
|-----|-------|--------|-------------|
| [ADR-0001](./ADR-0001-go-1.23-standardization.md) | Go 1.23 Standardization | Accepted | Standardization on Go 1.23 runtime across all environments |
| [ADR-0002](./ADR-0002-postgresql-only-database-strategy.md) | PostgreSQL-Only Database Strategy | Accepted | Single database technology decision for operational simplicity |
| [ADR-0003](./ADR-0003-docker-compose-architecture.md) | Docker Compose Architecture | Accepted | Container orchestration strategy for all environments |
| [ADR-0004](./ADR-0004-github-container-registry-selection.md) | GitHub Container Registry Selection | Accepted | Container registry choice for image distribution |
| [ADR-0005](./ADR-0005-portscan-port-management.md) | PortScan Port Management | Accepted | Port allocation strategy to avoid conflicts |
| [ADR-0006](./ADR-0006-network-optimization-standards.md) | Network Optimization Standards (14KB First Packet) | Accepted | Web performance optimization targeting first packet limits |
| [ADR-0007](./ADR-0007-redis-caching-strategy.md) | Redis Caching Strategy | Accepted | Comprehensive caching layer implementation |
| [ADR-0008](./ADR-0008-database-performance-standards.md) | Database Performance Standards | Accepted | PostgreSQL performance requirements and monitoring |
| [ADR-0009](./ADR-0009-core-web-vitals-optimization.md) | Core Web Vitals Optimization | Accepted | User experience metrics optimization strategy |
| [ADR-0010](./ADR-0010-subagent-usage-requirements.md) | Subagent Usage Requirements | Accepted | AI service integration patterns and cost optimization |
| [ADR-0011](./ADR-0011-environment-variable-management.md) | Environment Variable Management | Accepted | Configuration and secrets management across environments |
| [ADR-0012](./ADR-0012-testing-strategy-postgresql-only.md) | Testing Strategy (PostgreSQL-Only) | Accepted | Testing approach aligned with database architecture |
| [ADR-0013](./ADR-0013-security-framework-standards.md) | Security Framework Standards | Accepted | Comprehensive security implementation requirements |
| [ADR-0014](./ADR-0014-api-design-consistency-rules.md) | API Design Consistency Rules | Accepted | REST API standards for consistent developer experience |
| [ADR-0015](./ADR-0015-htmx-frontend-performance-patterns.md) | HTMX Frontend Performance Patterns | Accepted | Frontend optimization patterns for HTMX architecture |
| [ADR-0016](./ADR-0016-ollama-containerization-strategy.md) | Ollama Containerization Strategy | Accepted | Local AI service deployment and resource management |
| [ADR-0017](./ADR-0017-docker-secrets-management.md) | Docker Secrets Management | Accepted | Production-grade secrets management with Docker |
| [ADR-0018](./ADR-0018-hot-reload-development-workflow.md) | Hot Reload Development Workflow | Accepted | Development environment optimization for rapid iteration |
| [ADR-0019](./ADR-0019-logging-monitoring-standards.md) | Logging and Monitoring Standards | Accepted | Observability framework for system reliability |

## ADR Categories

### Core Infrastructure
- ADR-0001: Go 1.23 Standardization
- ADR-0002: PostgreSQL-Only Database Strategy
- ADR-0003: Docker Compose Architecture
- ADR-0004: GitHub Container Registry Selection

### Performance & Optimization
- ADR-0005: PortScan Port Management
- ADR-0006: Network Optimization Standards (14KB First Packet)
- ADR-0007: Redis Caching Strategy
- ADR-0008: Database Performance Standards
- ADR-0009: Core Web Vitals Optimization

### AI & Services Integration
- ADR-0010: Subagent Usage Requirements
- ADR-0016: Ollama Containerization Strategy

### Development & Operations
- ADR-0011: Environment Variable Management
- ADR-0012: Testing Strategy (PostgreSQL-Only)
- ADR-0013: Security Framework Standards
- ADR-0017: Docker Secrets Management
- ADR-0018: Hot Reload Development Workflow
- ADR-0019: Logging and Monitoring Standards

### API & Frontend
- ADR-0014: API Design Consistency Rules
- ADR-0015: HTMX Frontend Performance Patterns

## Using These ADRs

Each ADR follows a consistent format:
- **Status**: Current state of the decision (Accepted, Deprecated, Superseded)
- **Context**: Background and motivation for the decision
- **Decision**: The specific architectural decision made
- **Consequences**: Positive, negative, and neutral outcomes

These ADRs serve as:
1. **Decision Reference**: Quick lookup for architectural choices
2. **Onboarding Guide**: New team members understanding system design
3. **Change Documentation**: Historical record of architectural evolution
4. **Implementation Guide**: Specific requirements and patterns to follow

## Contributing

When adding new ADRs:
1. Use the next sequential number (ADR-0020, ADR-0021, etc.)
2. Follow the standard ADR template format
3. Update this index with the new ADR
4. Link related ADRs where appropriate
5. Ensure the decision is final before marking as "Accepted"

## Related Documentation

- [Project Requirements Documents (PRDs)](../requirements/)
- [API Documentation](../../api/)
- [Deployment Guide](../../deployment/)
- [Development Setup](../../development/)