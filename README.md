# Alchemorsel v3 🧙‍♂️📱

[![Go Version](https://img.shields.io/badge/go-1.22+-blue.svg)](https://golang.org)
[![Architecture](https://img.shields.io/badge/architecture-hexagonal-green.svg)](docs/adr/001-hexagonal-architecture.md)
[![DDD](https://img.shields.io/badge/design-domain--driven-purple.svg)](docs/adr/002-domain-driven-design.md)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Enterprise-grade recipe management platform with AI-powered features, built using modern software architecture principles.

## 🚀 Architecture Highlights

- **Hexagonal Architecture (Ports & Adapters)** - Clean separation of concerns with dependency inversion
- **Domain-Driven Design** - Rich domain models with business logic encapsulation  
- **SOLID Principles** - Maintainable, extensible, and testable codebase
- **Event-Driven Architecture** - Loose coupling with domain events and message queues
- **Enterprise Patterns** - Repository, Specification, Factory, and Strategy patterns

## 🏗️ Project Structure

```
alchemorsel-v3/
├── cmd/                    # Application entry points
│   ├── api/               # REST API server
│   ├── worker/            # Background job processor  
│   └── migrate/           # Database migration tool
├── internal/              # Private application code
│   ├── domain/            # 🎯 Business logic & domain models
│   │   ├── recipe/        # Recipe aggregate and business rules
│   │   ├── user/          # User management domain
│   │   ├── ai/            # AI-related domain logic
│   │   ├── social/        # Social features domain
│   │   └── shared/        # Common domain components
│   ├── application/       # 🔄 Use cases & application services
│   │   ├── recipe/        # Recipe use case implementations
│   │   ├── user/          # User management use cases
│   │   └── ai/            # AI service orchestration
│   ├── ports/             # 🔌 Interface definitions
│   │   ├── inbound/       # Primary ports (API interfaces)
│   │   └── outbound/      # Secondary ports (repository interfaces)
│   └── infrastructure/    # 🔧 External system adapters
│       ├── http/          # HTTP server & middleware
│       ├── persistence/   # Database implementations
│       ├── ai/            # AI service integrations
│       ├── messaging/     # Event bus implementations
│       ├── storage/       # File storage adapters
│       ├── monitoring/    # Metrics & tracing
│       └── config/        # Configuration management
├── pkg/                   # 📦 Reusable packages
│   ├── errors/            # Structured error handling
│   ├── logger/            # Structured logging
│   ├── validator/         # Input validation
│   ├── crypto/            # Cryptographic utilities
│   └── healthcheck/       # Health check framework
├── api/                   # 📖 API specifications
│   └── openapi/           # OpenAPI 3.0 specifications
├── docs/                  # 📚 Documentation
│   └── adr/               # Architectural Decision Records
├── scripts/               # 🔨 Build & deployment scripts
├── deployments/           # 🚀 Deployment configurations
│   ├── docker/            # Docker configurations
│   ├── kubernetes/        # K8s manifests
│   └── terraform/         # Infrastructure as code
└── test/                  # 🧪 Testing utilities
    ├── integration/       # Integration tests
    ├── e2e/               # End-to-end tests
    └── fixtures/          # Test data fixtures
```

## 🎯 Core Features

### Recipe Management
- ✅ Rich recipe modeling with ingredients, instructions, nutrition
- ✅ Multiple cuisine types and difficulty levels
- ✅ Image and video attachments
- ✅ Version control and optimistic locking
- ✅ Publishing workflow with validation

### AI-Powered Features
- 🤖 AI recipe generation from prompts
- 🧮 Automatic nutrition analysis
- 🔄 Ingredient substitution suggestions
- 📊 Recipe quality scoring
- 🎯 Personalized recommendations

### Social Platform
- 👥 User profiles and preferences
- ❤️ Recipe likes and ratings
- 💬 Comments and reviews
- 👨‍👩‍👧‍👦 Follow system
- 📋 Recipe collections

### Enterprise Features
- 🔐 JWT-based authentication
- 🛡️ Role-based authorization  
- 📊 Comprehensive metrics & monitoring
- 🔍 Full-text search with filters
- 📄 Pagination and sorting
- 🚦 Rate limiting
- 📧 Email notifications
- 🔄 Event-driven integrations

## 🛠️ Technology Stack

### Core
- **Language**: Go 1.22+
- **Framework**: Gin (HTTP), Uber FX (DI)
- **Database**: PostgreSQL 14+ with pgx driver
- **Cache**: Redis 6+
- **Search**: PostgreSQL full-text search

### Infrastructure  
- **Message Queue**: Apache Kafka
- **File Storage**: AWS S3 / Local filesystem
- **Monitoring**: Prometheus, Jaeger, New Relic
- **Documentation**: OpenAPI 3.0, Swagger UI

### AI & External Services
- **AI Providers**: OpenAI GPT-4, Anthropic Claude
- **Email**: SendGrid / SMTP
- **Authentication**: OAuth 2.0 (Google, Facebook)

## 🚀 Quick Start

### Prerequisites
- Go 1.22+
- PostgreSQL 14+
- Redis 6+
- Docker & Docker Compose (optional)

### Local Development

1. **Clone the repository**
```bash
git clone https://github.com/alchemorsel/v3.git
cd alchemorsel-v3
```

2. **Install dependencies**
```bash
go mod download
```

3. **Set up environment**
```bash
cp config/config.yaml config/config.local.yaml
# Edit config.local.yaml with your settings
```

4. **Run database migrations**
```bash
go run cmd/migrate/main.go up
```

5. **Start the API server**
```bash
go run cmd/api/main.go
```

6. **Verify installation**
```bash
curl http://localhost:8080/health
```

### Docker Development

```bash
# Start all services
docker-compose up -d

# Run migrations
docker-compose exec api go run cmd/migrate/main.go up

# View logs
docker-compose logs -f api
```

## 📊 API Documentation

Interactive API documentation is available at:
- **Swagger UI**: http://localhost:8080/swagger/
- **OpenAPI Spec**: [api/openapi/alchemorsel-v3.yaml](api/openapi/alchemorsel-v3.yaml)

### Key Endpoints

```
POST   /api/v3/auth/login           # User authentication
GET    /api/v3/recipes              # List recipes with filters
POST   /api/v3/recipes              # Create new recipe
GET    /api/v3/recipes/{id}         # Get recipe by ID
PUT    /api/v3/recipes/{id}         # Update recipe
POST   /api/v3/recipes/{id}/publish # Publish recipe
POST   /api/v3/recipes/generate     # AI recipe generation
GET    /api/v3/users/{id}/recipes   # Get user's recipes
POST   /api/v3/recipes/{id}/like    # Like a recipe
GET    /api/v3/search/recipes       # Search recipes
GET    /api/v3/health               # Health check
```

## 🧪 Testing

### Unit Tests
```bash
go test ./internal/...
```

### Integration Tests  
```bash
go test ./test/integration/...
```

### End-to-End Tests
```bash
go test ./test/e2e/...
```

### Test Coverage
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 📈 Monitoring & Observability

### Health Checks
- **Liveness**: `/health` - Basic service health
- **Readiness**: `/ready` - Dependencies health  
- **Metrics**: `/metrics` - Prometheus metrics

### Key Metrics
- HTTP request duration and count
- Database connection pool stats
- Cache hit/miss ratios
- AI service response times
- Domain event processing rates

### Distributed Tracing
Jaeger tracing is enabled for:
- HTTP requests
- Database queries  
- External API calls
- Message queue operations

## 🏗️ Architecture Decisions

This project follows several architectural patterns:

- **[ADR-001: Hexagonal Architecture](docs/adr/001-hexagonal-architecture.md)**
- **[ADR-002: Domain-Driven Design](docs/adr/002-domain-driven-design.md)**

See [docs/adr/](docs/adr/) for all architectural decisions.

## 🔧 Configuration

Configuration is managed through:
- YAML files (`config/config.yaml`)
- Environment variables (`ALCHEMORSEL_*`)  
- Command-line flags

### Key Configuration Sections
- **Database**: Connection settings, pool configuration
- **Redis**: Cache configuration  
- **Auth**: JWT secrets, OAuth settings
- **AI**: API keys, model settings
- **Monitoring**: Metrics, tracing, logging
- **Features**: Feature flags for gradual rollouts

## 🚀 Deployment

### Docker
```bash
docker build -t alchemorsel-v3 .
docker run -p 8080:8080 alchemorsel-v3
```

### Kubernetes
```bash
kubectl apply -f deployments/kubernetes/
```

### Production Considerations
- Use secrets management for sensitive configuration
- Enable TLS/HTTPS in production
- Configure proper logging and monitoring
- Set up backup strategies for PostgreSQL
- Consider using managed services (RDS, ElastiCache)

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines
- Follow Go best practices and idioms
- Write comprehensive tests for new features
- Update documentation for API changes
- Use conventional commit messages
- Ensure all checks pass before submitting PR

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Inspired by clean architecture principles
- Built with enterprise-grade Go patterns
- Leverages modern cloud-native technologies

---

**Built with ❤️ by the Alchemorsel Team**