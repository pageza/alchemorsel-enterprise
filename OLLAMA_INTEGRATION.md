# Ollama Containerization Integration for Alchemorsel v3

## Overview

This document describes the comprehensive Ollama containerization implementation for Alchemorsel v3, providing self-hosted AI inference capabilities with enterprise-grade performance, monitoring, and caching.

## Implementation Summary

### ✅ Completed Components

1. **Containerized Ollama Service**
   - Custom Dockerfile with optimized configuration
   - Automated model management and preloading
   - Health checks and monitoring integration
   - Resource management with memory/CPU limits

2. **Docker Compose Integration**
   - Service definition in `docker-compose.services.yml`
   - Persistent volume management for models
   - Network configuration and service discovery
   - Development overrides for local development

3. **AI Service Architecture**
   - Multi-provider support (Ollama, OpenAI) with intelligent fallback
   - Cached AI service with Redis integration
   - Health check system for AI providers
   - Performance optimization for Ollama

4. **Development Tooling**
   - Management script (`scripts/ollama-dev.sh`)
   - Development configuration overrides
   - Automated model initialization
   - Comprehensive health monitoring

5. **Configuration Management**
   - Updated configuration files for Ollama integration
   - Environment variable management
   - Secure secrets handling

## Architecture Details

### Service Structure

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Web Service   │    │   API Service   │    │  Ollama Service │
│   (Port 3011)   │────│   (Port 3010)   │────│  (Port 11434)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                        │
                       ┌─────────────────┐    ┌─────────────────┐
                       │  Redis Cache    │    │  Model Storage  │
                       │   (Port 6379)   │    │   (Persistent)  │
                       └─────────────────┘    └─────────────────┘
```

### AI Provider Hierarchy

1. **Primary Provider**: Ollama (containerized, local inference)
2. **Fallback Provider**: OpenAI (cloud-based, API key required)
3. **Final Fallback**: Mock service (development/testing)

### Caching Strategy

- **Recipe Generation**: 2-hour TTL with Redis caching
- **Ingredient Suggestions**: 6-hour TTL
- **Nutrition Analysis**: 12-hour TTL
- **Intelligent Cache Warmup**: Pre-loads common queries
- **Performance Optimization**: Async caching, compression

## File Structure

```
deployments/ollama/
├── Dockerfile                     # Ollama container definition
├── entrypoint.sh                 # Container initialization script
├── docker-compose.override.yml   # Development overrides
├── scripts/
│   ├── init-models.sh            # Model initialization
│   └── models.json               # Model configuration
└── health/
    └── healthcheck.sh            # Health check script

internal/infrastructure/ai/
├── ollama/
│   └── client.go                 # Ollama client implementation
├── openai/
│   └── client.go                 # OpenAI client (updated)
├── health.go                     # AI health monitoring
└── cache_integration.go          # AI caching optimization

scripts/
└── ollama-dev.sh                 # Development management script
```

## Usage Instructions

### Starting the System

```bash
# Start all services including Ollama
docker-compose -f docker-compose.services.yml up -d

# Or use the development script
./scripts/ollama-dev.sh start
```

### Development Commands

```bash
# Check service status
./scripts/ollama-dev.sh status

# View logs
./scripts/ollama-dev.sh logs -f

# Pull additional models
./scripts/ollama-dev.sh pull llama3.2:1b

# Test AI functionality
./scripts/ollama-dev.sh test

# Health check
./scripts/ollama-dev.sh health
```

### Model Management

```bash
# List available models
./scripts/ollama-dev.sh list

# Remove unused models
./scripts/ollama-dev.sh remove llama3.2:1b

# Execute commands in container
./scripts/ollama-dev.sh exec ollama --version
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ALCHEMORSEL_AI_PROVIDER` | `ollama` | Primary AI provider |
| `ALCHEMORSEL_OLLAMA_HOST` | `http://ollama:11434` | Ollama service URL |
| `ALCHEMORSEL_OLLAMA_MODEL` | `llama3.2:3b` | Default model |
| `ALCHEMORSEL_OLLAMA_TIMEOUT` | `30s` | Request timeout |

### Model Configuration

Default models loaded:
- **llama3.2:3b** (Primary - recipe generation, general AI)
- **llama3.2:1b** (Lightweight - quick responses, development)

Optional models:
- **codellama:7b** (Code analysis)
- **mistral:7b** (Alternative responses)

### Resource Requirements

**Development**:
- Memory: 4GB limit, 2GB reserved
- CPU: 2 cores limit, 1 core reserved
- Disk: ~5GB for models

**Production**:
- Memory: 8GB limit, 4GB reserved
- CPU: 4 cores limit, 2 cores reserved
- Disk: ~20GB for multiple models

## Performance Features

### Intelligent Caching
- **Cache-first pattern**: Check Redis before AI inference
- **Prompt hashing**: Efficient cache key generation
- **Adaptive TTL**: Longer cache for high-confidence responses
- **Background refresh**: Async cache updates

### Model Optimization
- **Preloading**: Models loaded into memory at startup
- **Connection pooling**: Efficient HTTP client management
- **Streaming support**: Ready for real-time inference
- **Graceful degradation**: Fallback providers for reliability

### Monitoring Integration
- **Health checks**: Deep health monitoring with metrics
- **Performance tracking**: Response times and success rates
- **Resource monitoring**: Memory and CPU usage
- **Alert integration**: Ready for production monitoring

## Health Monitoring

The system provides comprehensive health monitoring:

```bash
# Basic health check
curl http://localhost:3010/health

# AI-specific health
curl http://localhost:3010/api/v1/ai/health

# Ollama direct health
curl http://localhost:11435/api/tags
```

Health check includes:
- ✅ Container status
- ✅ API responsiveness  
- ✅ Model availability
- ✅ Resource usage
- ✅ Inference testing

## Integration with Existing Infrastructure

### Cache Integration
- Leverages existing Redis infrastructure
- Integrates with enterprise cache service
- Uses established cache patterns and monitoring

### Health Check Integration
- Extends enterprise health check framework
- Adds AI-specific dependency monitoring
- Maintains existing circuit breaker patterns

### Configuration Management
- Uses existing configuration loading system
- Follows established environment variable patterns
- Integrates with secure secrets management

### Monitoring Integration
- Exports metrics to existing Prometheus setup
- Uses established logging patterns
- Integrates with existing alerting

## Development Workflow

### Initial Setup
```bash
# Clone repository and navigate to project
cd alchemorsel-v3

# Create data directory for models
mkdir -p data/ollama

# Start services
./scripts/ollama-dev.sh start
```

### Daily Development
```bash
# Check status
./scripts/ollama-dev.sh status

# View logs during development
./scripts/ollama-dev.sh logs -f

# Test AI features
./scripts/ollama-dev.sh test
```

### Troubleshooting
```bash
# Comprehensive health check
./scripts/ollama-dev.sh health

# Restart if needed
./scripts/ollama-dev.sh restart

# Inspect container
./scripts/ollama-dev.sh exec bash
```

## Production Considerations

### GPU Support
Uncomment GPU configuration in docker-compose.services.yml:
```yaml
runtime: nvidia
environment:
  NVIDIA_VISIBLE_DEVICES: all
  OLLAMA_LLM_LIBRARY: cuda
```

### Model Selection
- **Production**: Use larger models (7B-13B parameters)
- **Development**: Use smaller models (1B-3B parameters)
- **Testing**: Use minimal models or mocks

### Security
- Models stored in persistent volumes with appropriate permissions
- No external network access required for AI inference
- Secrets managed through existing security framework

### Scaling
- Horizontal scaling through Docker Swarm or Kubernetes
- Load balancing with health check integration
- Model sharding for large-scale deployments

## Performance Benchmarks

Expected performance metrics:
- **Recipe Generation**: 2-5 seconds (uncached), <100ms (cached)
- **Ingredient Suggestions**: 1-2 seconds (uncached), <50ms (cached)
- **Nutrition Analysis**: 1-3 seconds (uncached), <50ms (cached)
- **Model Loading**: 30-60 seconds (startup)
- **Memory Usage**: 2-4GB per loaded model

## Next Steps

### Testing Phase
```bash
# Run comprehensive tests
./scripts/ollama-dev.sh test

# Load testing
# (Add load testing scripts as needed)
```

### Production Deployment
1. Update resource limits for production workloads
2. Configure GPU support if available
3. Set up monitoring and alerting
4. Configure backup strategies for model storage
5. Implement model rotation strategies

## Support and Troubleshooting

### Common Issues

**Service not starting**: Check Docker resources and model download status
**Slow responses**: Monitor model loading and consider preloading optimization
**Memory issues**: Adjust model selection and resource limits
**Cache misses**: Verify Redis connectivity and cache configuration

### Debugging Commands
```bash
# Service logs
./scripts/ollama-dev.sh logs

# Container inspection
docker inspect alchemorsel-ollama

# Resource usage
docker stats alchemorsel-ollama

# Model status
./scripts/ollama-dev.sh exec ollama list
```

This implementation provides a robust, scalable, and maintainable foundation for AI-powered recipe generation in Alchemorsel v3, with comprehensive monitoring, caching, and development tools.