#!/bin/bash
# Ollama Model Initialization Script for Alchemorsel v3
# Handles intelligent model management, caching, and optimization

set -e

# Colors for logging
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Logging functions
log_info() {
    echo -e "${BLUE}[MODELS]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_success() {
    echo -e "${GREEN}[MODELS]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_warning() {
    echo -e "${YELLOW}[MODELS]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_error() {
    echo -e "${RED}[MODELS]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

# Configuration
OLLAMA_URL="${OLLAMA_URL:-http://localhost:11434}"
MODELS_CONFIG_FILE="${MODELS_CONFIG_FILE:-/scripts/models.json}"
PULL_TIMEOUT="${OLLAMA_MODEL_TIMEOUT:-600}"
MAX_RETRIES="${OLLAMA_MAX_RETRIES:-3}"

# Default model configuration for Alchemorsel v3
DEFAULT_MODELS_CONFIG='{
    "models": [
        {
            "name": "llama3.2:3b",
            "purpose": "general",
            "priority": 1,
            "preload": true,
            "required": true,
            "description": "Primary model for recipe generation and AI interactions"
        },
        {
            "name": "llama3.2:1b",
            "purpose": "lightweight",
            "priority": 2,
            "preload": false,
            "required": false,
            "description": "Lightweight model for quick responses and development"
        },
        {
            "name": "codellama:7b",
            "purpose": "code",
            "priority": 3,
            "preload": false,
            "required": false,
            "description": "Code analysis and generation model"
        }
    ],
    "settings": {
        "auto_cleanup": true,
        "max_disk_usage_gb": 50,
        "preserve_required": true
    }
}'

# Function to wait for Ollama service
wait_for_ollama() {
    local timeout=${1:-60}
    local count=0
    
    log_info "Waiting for Ollama service to be ready..."
    
    while [ $count -lt $timeout ]; do
        if curl -s -f "$OLLAMA_URL/api/tags" >/dev/null 2>&1; then
            log_success "Ollama service is ready"
            return 0
        fi
        
        sleep 1
        count=$((count + 1))
        
        if [ $((count % 15)) -eq 0 ]; then
            log_info "Still waiting for Ollama... ($count/${timeout}s)"
        fi
    done
    
    log_error "Ollama service not ready after ${timeout} seconds"
    return 1
}

# Function to load models configuration
load_models_config() {
    if [ -f "$MODELS_CONFIG_FILE" ]; then
        log_info "Loading models configuration from $MODELS_CONFIG_FILE"
        cat "$MODELS_CONFIG_FILE"
    else
        log_info "Using default models configuration"
        echo "$DEFAULT_MODELS_CONFIG"
    fi
}

# Function to check if model exists locally
model_exists() {
    local model_name=$1
    ollama list | grep -q "^${model_name}" 2>/dev/null
}

# Function to get model size
get_model_size() {
    local model_name=$1
    ollama list | grep "^${model_name}" | awk '{print $2}' | head -1
}

# Function to check available disk space
check_disk_space() {
    local required_gb=${1:-10}
    
    if [ -d "/root/.ollama" ]; then
        local available_gb
        available_gb=$(df -BG /root/.ollama | awk 'NR==2 {print $4}' | sed 's/G//')
        
        if [ "$available_gb" -ge "$required_gb" ]; then
            log_success "Sufficient disk space: ${available_gb}GB available"
            return 0
        else
            log_warning "Low disk space: ${available_gb}GB available, ${required_gb}GB required"
            return 1
        fi
    else
        log_warning "Cannot check disk space: /root/.ollama not found"
        return 1
    fi
}

# Function to pull model with retries
pull_model_with_retry() {
    local model_name=$1
    local max_retries=${2:-$MAX_RETRIES}
    local retry_count=0
    
    while [ $retry_count -lt $max_retries ]; do
        log_info "Pulling model: $model_name (attempt $((retry_count + 1))/$max_retries)"
        
        if timeout $PULL_TIMEOUT ollama pull "$model_name" 2>&1 | while IFS= read -r line; do
            echo "[PULL] $line"
        done; then
            log_success "Successfully pulled model: $model_name"
            return 0
        else
            retry_count=$((retry_count + 1))
            if [ $retry_count -lt $max_retries ]; then
                local wait_time=$((retry_count * 10))
                log_warning "Pull failed, retrying in ${wait_time}s..."
                sleep $wait_time
            fi
        fi
    done
    
    log_error "Failed to pull model $model_name after $max_retries attempts"
    return 1
}

# Function to preload model into memory
preload_model() {
    local model_name=$1
    
    log_info "Preloading model into memory: $model_name"
    
    local test_request='{
        "model": "'$model_name'",
        "prompt": "Hello",
        "stream": false,
        "options": {
            "num_predict": 1,
            "temperature": 0.1
        }
    }'
    
    if curl -s --max-time 30 \
            -X POST "$OLLAMA_URL/api/generate" \
            -H "Content-Type: application/json" \
            -d "$test_request" >/dev/null 2>&1; then
        log_success "Model $model_name preloaded successfully"
        return 0
    else
        log_warning "Failed to preload model $model_name"
        return 1
    fi
}

# Function to cleanup old models
cleanup_models() {
    local config=$1
    local auto_cleanup
    auto_cleanup=$(echo "$config" | jq -r '.settings.auto_cleanup // false')
    
    if [ "$auto_cleanup" != "true" ]; then
        log_info "Auto cleanup disabled"
        return 0
    fi
    
    log_info "Starting model cleanup..."
    
    # Get list of required models
    local required_models
    required_models=$(echo "$config" | jq -r '.models[] | select(.required == true) | .name')
    
    # Get list of all models
    local all_models
    all_models=$(ollama list | awk 'NR>1 {print $1}' | grep -v '^$')
    
    # Remove models not in required list
    echo "$all_models" | while IFS= read -r model; do
        if [ -n "$model" ] && ! echo "$required_models" | grep -q "^$model$"; then
            log_info "Removing unused model: $model"
            if ollama rm "$model" 2>/dev/null; then
                log_success "Removed model: $model"
            else
                log_warning "Failed to remove model: $model"
            fi
        fi
    done
}

# Function to validate model functionality
validate_model() {
    local model_name=$1
    
    log_info "Validating model functionality: $model_name"
    
    local test_request='{
        "model": "'$model_name'",
        "prompt": "What is 2+2?",
        "stream": false,
        "options": {
            "num_predict": 10,
            "temperature": 0.1
        }
    }'
    
    local response
    if response=$(curl -s --max-time 30 \
                      -X POST "$OLLAMA_URL/api/generate" \
                      -H "Content-Type: application/json" \
                      -d "$test_request" 2>/dev/null); then
        
        if echo "$response" | jq -e '.response' >/dev/null 2>&1; then
            local answer
            answer=$(echo "$response" | jq -r '.response')
            log_success "Model $model_name validation passed (response: ${answer:0:50}...)"
            return 0
        else
            log_error "Model $model_name validation failed: No valid response"
            return 1
        fi
    else
        log_error "Model $model_name validation failed: Request timeout or error"
        return 1
    fi
}

# Function to display model status
show_model_status() {
    log_info "=== Current Model Status ==="
    
    if command -v ollama >/dev/null 2>&1 && curl -s -f "$OLLAMA_URL/api/tags" >/dev/null 2>&1; then
        ollama list | while IFS= read -r line; do
            log_info "$line"
        done
        
        # Show total disk usage
        if [ -d "/root/.ollama" ]; then
            local total_size
            total_size=$(du -sh /root/.ollama 2>/dev/null | cut -f1 || echo "Unknown")
            log_info "Total model storage: $total_size"
        fi
    else
        log_warning "Cannot retrieve model status: Ollama not available"
    fi
    
    log_info "==============================="
}

# Main function
main() {
    log_info "Starting Ollama model initialization for Alchemorsel v3..."
    
    # Wait for Ollama service
    if ! wait_for_ollama 60; then
        log_error "Ollama service not available"
        exit 1
    fi
    
    # Load configuration
    local config
    config=$(load_models_config)
    
    # Check disk space
    local max_disk_usage
    max_disk_usage=$(echo "$config" | jq -r '.settings.max_disk_usage_gb // 50')
    check_disk_space "$max_disk_usage" || log_warning "Continuing despite disk space warning..."
    
    # Process each model
    local models
    models=$(echo "$config" | jq -c '.models[]')
    
    local total_models=0
    local successful_models=0
    local required_failures=0
    
    echo "$models" | while IFS= read -r model_config; do
        local model_name
        local required
        local preload
        local priority
        local purpose
        
        model_name=$(echo "$model_config" | jq -r '.name')
        required=$(echo "$model_config" | jq -r '.required // false')
        preload=$(echo "$model_config" | jq -r '.preload // false')
        priority=$(echo "$model_config" | jq -r '.priority // 999')
        purpose=$(echo "$model_config" | jq -r '.purpose // "general"')
        
        total_models=$((total_models + 1))
        
        log_info "Processing model: $model_name (purpose: $purpose, priority: $priority)"
        
        # Check if model already exists
        if model_exists "$model_name"; then
            log_success "Model $model_name already available"
            
            # Validate existing model
            if validate_model "$model_name"; then
                successful_models=$((successful_models + 1))
                
                # Preload if requested
                if [ "$preload" = "true" ]; then
                    preload_model "$model_name" || true
                fi
            else
                log_warning "Model $model_name exists but validation failed"
                if [ "$required" = "true" ]; then
                    required_failures=$((required_failures + 1))
                fi
            fi
        else
            # Pull model
            if pull_model_with_retry "$model_name"; then
                if validate_model "$model_name"; then
                    successful_models=$((successful_models + 1))
                    
                    # Preload if requested
                    if [ "$preload" = "true" ]; then
                        preload_model "$model_name" || true
                    fi
                else
                    log_error "Model $model_name pulled but validation failed"
                    if [ "$required" = "true" ]; then
                        required_failures=$((required_failures + 1))
                    fi
                fi
            else
                log_error "Failed to pull model $model_name"
                if [ "$required" = "true" ]; then
                    required_failures=$((required_failures + 1))
                fi
            fi
        fi
    done
    
    # Cleanup if enabled
    cleanup_models "$config"
    
    # Show final status
    show_model_status
    
    log_info "Model initialization complete"
    log_info "Summary: $successful_models successful, $((total_models - successful_models)) failed"
    
    if [ $required_failures -gt 0 ]; then
        log_error "$required_failures required model(s) failed - this may impact functionality"
        exit 1
    elif [ $successful_models -eq 0 ]; then
        log_error "No models successfully initialized"
        exit 1
    else
        log_success "Model initialization completed successfully"
        exit 0
    fi
}

# Execute main function if script is run directly
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    main "$@"
fi