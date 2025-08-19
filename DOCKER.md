# Alchemorsel v3 Docker Deployment Guide

This guide covers Docker-based deployment options for Alchemorsel v3, supporting both local development and production environments.

## 🏗️ Architecture Overview

Alchemorsel v3 uses an enterprise service separation architecture:

- **API Backend Service** (`cmd/api-pure/main.go`) - Pure JSON API on port 3000
- **Web Frontend Service** (`cmd/web/main.go`) - HTMX templates on port 8080
- **PostgreSQL Database** - Primary data storage
- **Redis Cache** - Session and caching layer
- **Nginx Proxy** - Load balancing and SSL termination (enterprise/full mode)

## 🚀 Quick Start

### Prerequisites

- Docker 20.10+
- Docker Compose 2.0+
- 4GB+ RAM available for containers
- Git (for cloning)

### 1. Clone and Setup

```bash
git clone <repository-url>
cd alchemorsel-v3
cp .env.example .env
# Edit .env with your configuration
```

### 2. Choose Your Environment

#### Local Development (Recommended for development)
```bash
./scripts/docker-dev.sh local up
```

#### Enterprise Setup (Full monitoring stack)
```bash
./scripts/docker-dev.sh enterprise up
```

#### Full Production-like Setup (With Nginx proxy)
```bash
./scripts/docker-dev.sh full up
```

## 📋 Environment Options

### Local Environment (`docker-compose.local.yml`)

**Services:**
- API Backend (port 3000)
- Web Frontend (port 8080)
- PostgreSQL (port 5432)
- Redis (port 6379)

**Best for:** Day-to-day development, testing features

**Resource usage:** ~1GB RAM

### Enterprise Environment (`docker-compose.enterprise.yml`)

**Services:**
- API Backend (port 3000)
- Web Frontend (port 8080)
- PostgreSQL (port 5432)
- Redis (port 6379)
- Jaeger (port 16686) - Distributed tracing
- Prometheus (port 9090) - Metrics collection
- Grafana (port 3001) - Metrics visualization
- MinIO (ports 9000, 9001) - S3-compatible storage
- Nginx (ports 80, 443) - Reverse proxy

**Best for:** Integration testing, performance analysis, production simulation

**Resource usage:** ~3GB RAM

### Full Environment (`docker-compose.yml`)

**Services:** Same as enterprise + additional monitoring and worker services

**Best for:** Full production simulation, load testing

**Resource usage:** ~4GB RAM

## 🔧 Management Commands

The `scripts/docker-dev.sh` script provides easy management:

```bash
# Start services
./scripts/docker-dev.sh local up

# Stop services
./scripts/docker-dev.sh local down

# Restart services
./scripts/docker-dev.sh local restart

# View logs
./scripts/docker-dev.sh local logs

# Check service status
./scripts/docker-dev.sh local status

# Rebuild services
./scripts/docker-dev.sh local build

# Open shell in API container
./scripts/docker-dev.sh local shell
```

## 🌐 Access Points

### Local Environment
- **Web Application:** http://localhost:8080
- **API Backend:** http://localhost:3000
- **API Documentation:** http://localhost:3000/docs
- **Health Check:** http://localhost:3000/health

### Enterprise/Full Environment
- **Web Application:** http://localhost:8080 or http://localhost
- **API Backend:** http://localhost:3000
- **Nginx Proxy:** http://localhost
- **Grafana Dashboard:** http://localhost:3001 (admin/admin)
- **Prometheus Metrics:** http://localhost:9090
- **Jaeger Tracing:** http://localhost:16686
- **MinIO Console:** http://localhost:9001 (minioadmin/minioadmin)

## 🔐 Security Configuration

### Development Security
- JWT secrets are set to development values
- SSL is disabled for local development
- CORS allows localhost origins
- Rate limiting is relaxed

### Production Security Checklist
- [ ] Update JWT secrets in `.env`
- [ ] Enable SSL with proper certificates
- [ ] Configure restrictive CORS origins
- [ ] Enable rate limiting
- [ ] Set secure session cookies
- [ ] Update default passwords for monitoring tools

## 📊 Monitoring and Observability

### Metrics (Prometheus + Grafana)
- Application metrics on port 9091
- Infrastructure metrics collection
- Custom Grafana dashboards for business metrics

### Tracing (Jaeger)
- Distributed request tracing
- Performance bottleneck identification
- Service dependency mapping

### Logging
- Structured JSON logging
- Centralized log aggregation
- Error tracking and alerting

## 🗃️ Data Persistence

### Volumes
- `postgres_data` - Database storage
- `redis_data` - Cache persistence
- `grafana_data` - Dashboard configurations
- `prometheus_data` - Metrics storage
- `minio_data` - File uploads

### Backup Strategy
```bash
# Database backup
docker-compose exec postgres pg_dump -U postgres alchemorsel_dev > backup.sql

# Restore database
docker-compose exec -T postgres psql -U postgres alchemorsel_dev < backup.sql
```

## 🚀 Production Deployment

### Docker Image Building

```bash
# Build API service
docker build -f Dockerfile.api -t alchemorsel/api:latest .

# Build Web service
docker build -f Dockerfile.web -t alchemorsel/web:latest .
```

### Environment Variables

Key production environment variables:
```bash
ALCHEMORSEL_APP_ENVIRONMENT=production
ALCHEMORSEL_DATABASE_SSL_MODE=require
ALCHEMORSEL_SESSION_SECURE=true
ALCHEMORSEL_JWT_SECRET=<strong-random-secret>
ANTHROPIC_API_KEY=<your-claude-api-key>
```

### Health Checks

Both services include health check endpoints:
- API: `GET /health`
- Web: `GET /health`

Docker health checks run every 30 seconds with 3 retries.

## 🔧 Troubleshooting

### Common Issues

#### Services Won't Start
```bash
# Check Docker daemon
docker info

# Check resource usage
docker stats

# View service logs
./scripts/docker-dev.sh local logs
```

#### Database Connection Issues
```bash
# Check PostgreSQL logs
docker-compose logs postgres

# Test database connection
docker-compose exec postgres psql -U postgres -d alchemorsel_dev -c "SELECT version();"
```

#### API Backend Issues
```bash
# Check API logs
docker-compose logs api-backend

# Test API health
curl http://localhost:3000/health
```

#### Web Frontend Issues
```bash
# Check web logs
docker-compose logs web-frontend

# Test web health
curl http://localhost:8080/health
```

### Performance Optimization

#### Resource Limits
Add resource limits to docker-compose services:
```yaml
services:
  api-backend:
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: '0.5'
```

#### Database Tuning
For production PostgreSQL:
```yaml
postgres:
  environment:
    POSTGRES_SHARED_PRELOAD_LIBRARIES: pg_stat_statements
  command: >
    postgres
    -c max_connections=100
    -c shared_buffers=256MB
    -c effective_cache_size=1GB
```

## 📝 Development Workflow

### Local Development with Hot Reload

For development with file watching:
```bash
# Start dependencies only
docker-compose -f docker-compose.local.yml up postgres redis -d

# Run services locally for hot reload
API_URL=http://localhost:3000 go run cmd/web/main.go &
PORT=3000 go run cmd/api-pure/main.go
```

### Testing with Docker

```bash
# Run tests in container
docker-compose exec api-backend go test ./...

# Run integration tests
docker-compose exec api-backend go test -tags=integration ./...
```

### Debugging

```bash
# Run service with debugging
docker-compose run --rm api-backend dlv debug cmd/api-pure/main.go

# Attach to running container
docker-compose exec api-backend sh
```

## 🔄 Updates and Maintenance

### Updating Services
```bash
# Pull latest images
docker-compose pull

# Rebuild and restart
./scripts/docker-dev.sh local build
./scripts/docker-dev.sh local restart
```

### Database Migrations
```bash
# Run migrations
docker-compose exec api-backend ./migrate-up

# Check migration status
docker-compose exec api-backend ./migrate-status
```

### Cleanup
```bash
# Remove all containers and volumes
./scripts/docker-dev.sh local down
docker volume prune

# Clean up images
docker image prune -a
```

## 📞 Support

For issues with Docker deployment:
1. Check the logs with `./scripts/docker-dev.sh <env> logs`
2. Verify environment configuration in `.env`
3. Ensure adequate system resources (4GB+ RAM)
4. Check Docker and Docker Compose versions

## 🔗 Related Documentation

- [API Documentation](./docs/api.md)
- [Web Frontend Guide](./docs/web.md)
- [Monitoring Setup](./docs/monitoring.md)
- [Security Guide](./docs/security.md)