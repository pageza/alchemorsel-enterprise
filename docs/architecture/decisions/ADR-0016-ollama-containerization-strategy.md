# ADR-0016: Ollama Containerization Strategy

## Status
Accepted

## Context
Alchemorsel v3 integrates Ollama for local AI model inference, providing privacy-focused AI capabilities without external API dependencies. Ollama requires careful containerization to handle large language models, GPU resources, and significant memory requirements while maintaining consistent deployment across environments.

Ollama requirements:
- Support for multiple language models (7B-70B parameters)
- GPU acceleration when available
- High memory usage (4GB-32GB per model)
- Model persistence across container restarts
- Integration with the main application stack

Deployment considerations:
- Development environment with limited resources
- Production environment with potential GPU access
- Model downloading and storage requirements
- Service discovery and networking
- Resource limits and allocation

## Decision
We will containerize Ollama with a flexible architecture supporting both CPU and GPU inference while providing efficient model management and integration with the Docker Compose stack.

**Containerization Architecture:**

**Docker Configuration:**
```dockerfile
# Ollama service configuration
FROM ollama/ollama:latest

# Create non-root user
RUN useradd -m -u 1000 ollama

# Model storage directory
VOLUME ["/root/.ollama"]

# Health check for service availability  
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD ollama list || exit 1

# Expose Ollama API port
EXPOSE 11434

# Run as ollama user
USER ollama
CMD ["ollama", "serve"]
```

**Docker Compose Integration:**
```yaml
services:
  ollama:
    image: ollama/ollama:latest
    container_name: alchemorsel-ollama
    restart: unless-stopped
    ports:
      - "11435:11434"  # External port mapping (ADR-0005)
    volumes:
      - ollama_models:/root/.ollama
      - ./ollama/models:/models:ro  # Pre-downloaded models
    environment:
      - OLLAMA_HOST=0.0.0.0
      - OLLAMA_ORIGINS=*
      - OLLAMA_NUM_PARALLEL=2
      - OLLAMA_MAX_LOADED_MODELS=2
    networks:
      - alchemorsel-network
    deploy:
      resources:
        limits:
          memory: 8G
        reservations:
          memory: 4G
    # GPU support (commented for CPU-only environments)
    # runtime: nvidia
    # environment:
    #   - NVIDIA_VISIBLE_DEVICES=all

volumes:
  ollama_models:
    driver: local
```

**Model Management Strategy:**

**Pre-installed Models:**
```bash
# Model initialization script
#!/bin/bash
# ollama/init-models.sh

echo "Initializing Ollama models..."

# Wait for Ollama service to be ready
until curl -f http://localhost:11434/api/tags >/dev/null 2>&1; do
    echo "Waiting for Ollama service..."
    sleep 5
done

# Pull required models
ollama pull llama2:7b-chat      # General conversation
ollama pull codellama:7b        # Code analysis  
ollama pull mistral:7b          # Fast inference

echo "Models initialized successfully"
```

**Integration with Main Application:**
```go
// Go client configuration
type OllamaClient struct {
    baseURL    string
    httpClient *http.Client
    timeout    time.Duration
}

func NewOllamaClient() *OllamaClient {
    return &OllamaClient{
        baseURL: os.Getenv("OLLAMA_BASE_URL"), // http://ollama:11434
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}
```

**Resource Management:**
- Memory limits prevent OOM conditions
- CPU limits ensure fair resource sharing
- Model auto-unloading for memory management
- Graceful degradation when Ollama unavailable

**Development vs Production:**

**Development Environment:**
- CPU-only inference for broad compatibility
- Smaller models (7B parameters) for faster responses
- Shared model storage to reduce disk usage
- Optional service (can be disabled)

**Production Environment:**
- GPU acceleration when available
- Larger models (13B-70B) for better quality
- Dedicated model storage with backup
- High availability with health checks

**Monitoring and Observability:**
```yaml
# Additional monitoring configuration
  ollama:
    # ... existing config
    labels:
      - "com.docker.compose.service=ollama"
      - "monitoring.enable=true"
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

## Consequences

### Positive
- Self-hosted AI inference without external API dependencies
- Privacy-focused AI capabilities with local data processing
- Cost predictability without per-request API charges
- Flexible model selection based on use case requirements
- Consistent AI performance without network latency

### Negative
- Significant resource requirements (memory, storage, compute)
- Complexity in model management and updates
- Slower inference compared to cloud-based solutions
- GPU requirements for optimal performance
- Large container images and download times

### Neutral
- Industry standard containerization approach for AI services
- Compatible with orchestration platforms (Kubernetes, Docker Swarm)
- Supports hybrid deployments (local + cloud AI)