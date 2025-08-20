# Ollama Containerization for Alchemorsel v3
# Based on ADR-0016: Ollama Containerization Strategy
# Optimized for both development and production environments

FROM ollama/ollama:latest

LABEL maintainer="Alchemorsel v3 Team"
LABEL description="Containerized Ollama for local AI inference"
LABEL version="1.0.0"

# Set environment variables for optimal Ollama configuration
ENV OLLAMA_HOST=0.0.0.0
ENV OLLAMA_ORIGINS=*
ENV OLLAMA_NUM_PARALLEL=2
ENV OLLAMA_MAX_LOADED_MODELS=2
ENV OLLAMA_DEBUG=false
ENV OLLAMA_FLASH_ATTENTION=false
ENV OLLAMA_LLM_LIBRARY=cpu
ENV OLLAMA_NOPRUNE=false
ENV OLLAMA_RUNNER_SLEEP=10s
ENV OLLAMA_TMPDIR=/tmp

# Create non-root user for security
RUN groupadd -r ollama && useradd -r -g ollama ollama

# Create necessary directories
RUN mkdir -p /root/.ollama/models \
    && mkdir -p /models \
    && mkdir -p /scripts \
    && mkdir -p /health \
    && chown -R ollama:ollama /root/.ollama

# Install additional utilities for health checks and model management
RUN apt-get update && apt-get install -y \
    curl \
    jq \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Copy model initialization and health check scripts
COPY deployments/ollama/scripts/ /scripts/
COPY deployments/ollama/health/ /health/

# Make scripts executable
RUN chmod +x /scripts/*.sh /health/*.sh

# Create model storage volume
VOLUME ["/root/.ollama"]

# Health check for service availability and model readiness
HEALTHCHECK --interval=30s --timeout=15s --start-period=120s --retries=3 \
    CMD /health/healthcheck.sh

# Expose Ollama API port
EXPOSE 11434

# Create entrypoint script for proper initialization
COPY deployments/ollama/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Switch to non-root user for security (except for model directory access)
# Note: Ollama needs access to /root/.ollama for model storage
USER root

# Use custom entrypoint for proper initialization
ENTRYPOINT ["/entrypoint.sh"]
CMD ["ollama", "serve"]