#!/bin/bash
# Ollama Entrypoint Script for Alchemorsel v3
# Handles initialization, model preloading, and graceful startup

set -e

# Colors for logging
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

# Configuration variables
DEFAULT_MODEL="${OLLAMA_DEFAULT_MODEL:-llama3.2:3b}"
PRELOAD_MODELS="${OLLAMA_PRELOAD_MODELS:-$DEFAULT_MODEL}"
HEALTH_CHECK_TIMEOUT="${OLLAMA_HEALTH_TIMEOUT:-60}"
MODEL_PULL_TIMEOUT="${OLLAMA_MODEL_TIMEOUT:-600}"

# Function to check if Ollama is running
wait_for_ollama() {
    local timeout=$1
    local count=0
    
    log_info "Waiting for Ollama service to be ready..."
    
    while [ $count -lt $timeout ]; do
        if curl -s -f http://localhost:11434/api/tags >/dev/null 2>&1; then
            log_success "Ollama service is ready"
            return 0
        fi
        
        sleep 1
        count=$((count + 1))
        
        if [ $((count % 10)) -eq 0 ]; then
            log_info "Still waiting for Ollama service... ($count/${timeout}s)"
        fi
    done
    
    log_error "Ollama service failed to start within ${timeout} seconds"
    return 1
}

# Function to check if a model exists
model_exists() {
    local model=$1
    ollama list | grep -q "^${model}" 2>/dev/null
}

# Function to pull and verify model
pull_model() {
    local model=$1
    
    log_info "Checking model: $model"
    
    if model_exists "$model"; then
        log_success "Model $model already available"
        return 0
    fi
    
    log_info "Pulling model: $model (this may take several minutes...)"
    
    # Pull model with timeout
    if timeout $MODEL_PULL_TIMEOUT ollama pull "$model"; then
        log_success "Successfully pulled model: $model"
        
        # Verify model is accessible
        if ollama list | grep -q "^${model}"; then
            log_success "Model $model verified and ready"
            return 0
        else
            log_error "Model $model pull completed but not accessible"
            return 1
        fi
    else
        log_error "Failed to pull model: $model (timeout: ${MODEL_PULL_TIMEOUT}s)"
        return 1
    fi
}

# Function to preload models into memory
preload_model() {
    local model=$1
    
    log_info "Preloading model into memory: $model"
    
    # Generate a small test prompt to load model into memory
    local test_prompt="Hello, this is a test to load the model."
    
    if echo '{"model":"'$model'","prompt":"'$test_prompt'","stream":false}' | \
       curl -s -X POST http://localhost:11434/api/generate \
            -H "Content-Type: application/json" \
            -d @- >/dev/null 2>&1; then
        log_success "Model $model preloaded into memory"
        return 0
    else
        log_warning "Failed to preload model $model into memory"
        return 1
    fi
}

# Function to setup model management
setup_models() {
    log_info "Setting up AI models for Alchemorsel v3..."
    
    # Parse models to preload (comma-separated)
    IFS=',' read -ra MODELS <<< "$PRELOAD_MODELS"
    
    local success_count=0
    local total_count=${#MODELS[@]}
    
    for model in "${MODELS[@]}"; do
        # Trim whitespace
        model=$(echo "$model" | xargs)
        
        if [ -n "$model" ]; then
            if pull_model "$model"; then
                success_count=$((success_count + 1))
                
                # Preload model into memory for faster first response
                preload_model "$model" || true
            fi
        fi
    done
    
    log_info "Model setup complete: $success_count/$total_count models ready"
    
    if [ $success_count -eq 0 ]; then
        log_error "No models were successfully loaded"
        return 1
    fi
    
    return 0
}

# Function to display system information
display_system_info() {
    log_info "=== Ollama System Information ==="
    log_info "Ollama version: $(ollama --version 2>/dev/null || echo 'Unknown')"
    log_info "Default model: $DEFAULT_MODEL"
    log_info "Preload models: $PRELOAD_MODELS"
    log_info "Max parallel requests: ${OLLAMA_NUM_PARALLEL:-2}"
    log_info "Max loaded models: ${OLLAMA_MAX_LOADED_MODELS:-2}"
    log_info "Memory info:"
    free -h 2>/dev/null || echo "Memory info not available"
    log_info "================================="
}

# Function to handle graceful shutdown
cleanup() {
    log_info "Received shutdown signal, cleaning up..."
    
    # Stop Ollama gracefully
    if pgrep ollama >/dev/null; then
        log_info "Stopping Ollama service..."
        pkill -TERM ollama
        
        # Wait for graceful shutdown
        sleep 5
        
        # Force kill if still running
        if pgrep ollama >/dev/null; then
            log_warning "Force stopping Ollama service..."
            pkill -KILL ollama
        fi
    fi
    
    log_success "Cleanup completed"
    exit 0
}

# Set up signal handlers for graceful shutdown
trap cleanup SIGTERM SIGINT

# Main execution
main() {
    log_info "Starting Ollama service for Alchemorsel v3..."
    
    display_system_info
    
    # Start Ollama service in background
    log_info "Starting Ollama daemon..."
    ollama serve &
    OLLAMA_PID=$!
    
    # Wait for Ollama to be ready
    if ! wait_for_ollama $HEALTH_CHECK_TIMEOUT; then
        log_error "Failed to start Ollama service"
        exit 1
    fi
    
    # Setup models if specified
    if [ -n "$PRELOAD_MODELS" ] && [ "$PRELOAD_MODELS" != "none" ]; then
        if ! setup_models; then
            log_error "Model setup failed"
            exit 1
        fi
    else
        log_info "Model preloading disabled (PRELOAD_MODELS=none or empty)"
    fi
    
    log_success "Ollama service fully initialized and ready for Alchemorsel v3"
    
    # Keep the container running
    wait $OLLAMA_PID
}

# Execute main function if script is run directly
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    main "$@"
fi