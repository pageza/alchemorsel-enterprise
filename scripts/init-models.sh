#!/bin/bash
# Ollama Model Initialization Script for Alchemorsel v3
# This script downloads and initializes AI models for local chat functionality

set -e

# Configuration
OLLAMA_HOST="${OLLAMA_HOST:-http://localhost:11434}"
MAX_RETRIES=5
RETRY_DELAY=10

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Ollama is running
check_ollama() {
    local retries=0
    while [ $retries -lt $MAX_RETRIES ]; do
        if curl -s "${OLLAMA_HOST}/api/tags" > /dev/null 2>&1; then
            log_success "Ollama is running at ${OLLAMA_HOST}"
            return 0
        fi
        
        retries=$((retries + 1))
        log_warning "Ollama not responding (attempt ${retries}/${MAX_RETRIES}). Waiting ${RETRY_DELAY}s..."
        sleep $RETRY_DELAY
    done
    
    log_error "Ollama is not running at ${OLLAMA_HOST}"
    return 1
}

# Check if a model is already installed
is_model_installed() {
    local model_name="$1"
    curl -s "${OLLAMA_HOST}/api/tags" | grep -q "\"name\":\"${model_name}\""
}

# Download and install a model
install_model() {
    local model_name="$1"
    local description="$2"
    
    log_info "Installing model: ${model_name} (${description})"
    
    if is_model_installed "$model_name"; then
        log_success "Model ${model_name} is already installed"
        return 0
    fi
    
    log_info "Downloading ${model_name}... This may take several minutes"
    
    # Use curl to pull the model
    if curl -X POST "${OLLAMA_HOST}/api/pull" \
        -H "Content-Type: application/json" \
        -d "{\"name\":\"${model_name}\"}" \
        --progress-bar; then
        log_success "Successfully installed ${model_name}"
        return 0
    else
        log_error "Failed to install ${model_name}"
        return 1
    fi
}

# Test a model by generating a simple completion
test_model() {
    local model_name="$1"
    log_info "Testing model: ${model_name}"
    
    local test_prompt="Hello! Please respond with a brief greeting."
    local response
    
    response=$(curl -s -X POST "${OLLAMA_HOST}/api/generate" \
        -H "Content-Type: application/json" \
        -d "{\"model\":\"${model_name}\",\"prompt\":\"${test_prompt}\",\"stream\":false}" \
        | grep -o '"response":"[^"]*"' | head -1 | cut -d'"' -f4)
    
    if [ -n "$response" ] && [ "$response" != "null" ]; then
        log_success "Model ${model_name} test successful"
        log_info "Test response: ${response:0:100}..."
        return 0
    else
        log_error "Model ${model_name} test failed"
        return 1
    fi
}

# Warm up a model to reduce first-request latency
warm_up_model() {
    local model_name="$1"
    log_info "Warming up model: ${model_name}"
    
    # Send a simple request to load the model into memory
    curl -s -X POST "${OLLAMA_HOST}/api/generate" \
        -H "Content-Type: application/json" \
        -d "{\"model\":\"${model_name}\",\"prompt\":\"Ready\",\"stream\":false}" > /dev/null
    
    log_success "Model ${model_name} warmed up"
}

# Get model information
get_model_info() {
    local model_name="$1"
    log_info "Getting information for model: ${model_name}"
    
    curl -s -X POST "${OLLAMA_HOST}/api/show" \
        -H "Content-Type: application/json" \
        -d "{\"name\":\"${model_name}\"}" | \
        grep -E '"parameter_size"|"quantization_level"' | head -2
}

# Main installation function
main() {
    log_info "Starting Alchemorsel v3 AI model initialization"
    log_info "Target Ollama instance: ${OLLAMA_HOST}"
    
    # Check if Ollama is running
    if ! check_ollama; then
        log_error "Cannot continue without Ollama running"
        exit 1
    fi
    
    # Define models to install (matching configuration defaults)
    declare -A models
    models["phi3:3.8b-mini-instruct-q4_0"]="Fast, lightweight model for help and simple queries"
    models["llama3.1:8b-instruct-q4_K_M"]="Balanced model for chat and recipe generation"
    models["codellama:7b-code-q4_K_M"]="Code-specialized model for technical queries"
    
    # Optional high-quality model (comment out if resource-constrained)
    # models["llama3.1:70b-instruct-q4_K_M"]="High-quality model for complex tasks"
    
    log_info "Will install ${#models[@]} models"
    
    # Install each model
    local failed_models=()
    for model in "${!models[@]}"; do
        if install_model "$model" "${models[$model]}"; then
            log_success "✓ ${model} installed successfully"
        else
            log_error "✗ ${model} installation failed"
            failed_models+=("$model")
        fi
        echo # Add spacing
    done
    
    # Report installation results
    if [ ${#failed_models[@]} -eq 0 ]; then
        log_success "All models installed successfully!"
    else
        log_warning "Some models failed to install: ${failed_models[*]}"
    fi
    
    # Test installed models
    log_info "Testing installed models..."
    for model in "${!models[@]}"; do
        if is_model_installed "$model"; then
            if test_model "$model"; then
                warm_up_model "$model"
            fi
        fi
        echo # Add spacing
    done
    
    # Display final status
    log_info "Model initialization complete!"
    log_info "Available models:"
    curl -s "${OLLAMA_HOST}/api/tags" | grep -o '"name":"[^"]*"' | cut -d'"' -f4 | sort | sed 's/^/  - /'
    
    # Display memory usage estimates
    log_info "Estimated memory usage:"
    log_info "  - phi3:3.8b-mini-instruct-q4_0: ~2.3GB"
    log_info "  - llama3.1:8b-instruct-q4_K_M: ~4.7GB"  
    log_info "  - codellama:7b-code-q4_K_M: ~4.1GB"
    log_info "Total estimated: ~11GB (models loaded on-demand)"
    
    log_success "Alchemorsel v3 AI models are ready!"
}

# Handle script arguments
case "${1:-}" in
    --check)
        check_ollama
        exit $?
        ;;
    --test)
        if [ -n "$2" ]; then
            test_model "$2"
        else
            log_error "Usage: $0 --test MODEL_NAME"
            exit 1
        fi
        ;;
    --list)
        curl -s "${OLLAMA_HOST}/api/tags" | grep -o '"name":"[^"]*"' | cut -d'"' -f4 | sort
        ;;
    --help)
        echo "Alchemorsel v3 Model Initialization Script"
        echo ""
        echo "Usage: $0 [OPTION]"
        echo ""
        echo "Options:"
        echo "  (no option)  Install all required models"
        echo "  --check      Check if Ollama is running"
        echo "  --test NAME  Test a specific model"
        echo "  --list       List installed models"
        echo "  --help       Show this help message"
        echo ""
        echo "Environment variables:"
        echo "  OLLAMA_HOST  Ollama server URL (default: http://localhost:11434)"
        exit 0
        ;;
    "")
        main
        ;;
    *)
        log_error "Unknown option: $1"
        log_info "Use --help for usage information"
        exit 1
        ;;
esac