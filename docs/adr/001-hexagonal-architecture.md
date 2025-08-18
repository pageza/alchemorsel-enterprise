# ADR-001: Adopt Hexagonal Architecture (Ports and Adapters)

## Status
Accepted

## Context
We are building Alchemorsel v3, an enterprise-grade recipe management platform that needs to:
- Support multiple interfaces (REST API, GraphQL, CLI)
- Integrate with various external systems (AI services, payment providers, social media)
- Maintain high testability and maintainability
- Enable independent development of different system components
- Support future scalability and microservices migration

The traditional layered architecture presents several challenges:
- Tight coupling between layers
- Difficulty in testing business logic in isolation
- External dependencies bleeding into business logic
- Limited flexibility for different interface types

## Decision
We will adopt Hexagonal Architecture (also known as Ports and Adapters pattern) for Alchemorsel v3.

### Key Principles:
1. **Business Logic at the Center**: The domain layer contains pure business logic with no external dependencies
2. **Ports Define Contracts**: Interfaces define what the application needs (inbound ports) and what it provides (outbound ports)
3. **Adapters Implement Ports**: Concrete implementations handle external system interactions
4. **Dependency Inversion**: Dependencies point inward toward the business logic

### Structure:
```
internal/
├── domain/          # Core business entities, value objects, domain services
├── application/     # Use cases, application services
├── ports/
│   ├── inbound/     # Interfaces for driving adapters (HTTP, GraphQL)
│   └── outbound/    # Interfaces for driven adapters (database, external APIs)
├── infrastructure/  # Adapters implementation
│   ├── http/        # HTTP REST API adapter
│   ├── persistence/ # Database adapters
│   ├── ai/          # AI service adapters
│   └── messaging/   # Event/message bus adapters
```

## Consequences

### Positive:
- **Testability**: Business logic can be tested in complete isolation using test doubles
- **Flexibility**: Easy to swap implementations (e.g., PostgreSQL → MongoDB)
- **Independence**: External systems don't affect business logic
- **Scalability**: Clear boundaries enable microservices extraction
- **Maintainability**: Separation of concerns makes code easier to understand and modify

### Negative:
- **Initial Complexity**: More interfaces and abstractions to manage
- **Learning Curve**: Team needs to understand the pattern
- **Potential Over-Engineering**: Simple operations may seem unnecessarily complex

### Mitigation Strategies:
1. Comprehensive documentation and team training
2. Clear examples and templates for common patterns
3. Gradual adoption starting with core domains
4. Regular architecture reviews

## Implementation Notes

### Domain Layer
- Pure Go structs with business logic
- No external dependencies
- Rich domain models with behavior
- Domain events for cross-boundary communication

### Application Layer
- Orchestrates use cases
- Coordinates between domain and infrastructure
- Handles cross-cutting concerns (transactions, events)
- Thin layer focused on workflow

### Ports
- **Inbound**: HTTP handlers, GraphQL resolvers, CLI commands
- **Outbound**: Repository interfaces, external service interfaces

### Infrastructure
- Database implementations (PostgreSQL, Redis)
- External service clients (OpenAI, Stripe)
- Message queue implementations (Kafka, RabbitMQ)
- File storage implementations (S3, local filesystem)

## Related Decisions
- ADR-002: Domain-Driven Design Implementation
- ADR-003: Dependency Injection Framework Selection
- ADR-004: Event-Driven Architecture Patterns

## References
- [Hexagonal Architecture by Alistair Cockburn](https://alistair.cockburn.us/hexagonal-architecture/)
- [Clean Architecture by Robert Martin](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Ports and Adapters Pattern](https://jmgarridopaz.github.io/content/hexagonalarchitecture.html)