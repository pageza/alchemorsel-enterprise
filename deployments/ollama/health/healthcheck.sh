#!/bin/bash
# Ollama Health Check Script for Alchemorsel v3
# Comprehensive health monitoring for containerized Ollama service

set -e

# Configuration
OLLAMA_URL="${OLLAMA_URL:-http://localhost:11434}"
DEFAULT_MODEL="${OLLAMA_DEFAULT_MODEL:-llama3.2:3b}"
HEALTH_TIMEOUT="${OLLAMA_HEALTH_TIMEOUT:-10}"
TEST_PROMPT="test"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Logging functions
log_info() {
    echo -e "${GREEN}[HEALTH]${NC} $1" >&2
}

log_warning() {
    echo -e "${YELLOW}[HEALTH]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}[HEALTH]${NC} $1" >&2
}

# Function to check basic service availability
check_service() {
    log_info "Checking Ollama service availability..."
    
    if curl -s -f --max-time $HEALTH_TIMEOUT "$OLLAMA_URL/api/tags" >/dev/null 2>&1; then
        log_info "✓ Ollama service is responding"
        return 0
    else
        log_error "✗ Ollama service is not responding"
        return 1
    fi
}

# Function to check if models are loaded
check_models() {
    log_info "Checking available models..."
    
    local models_response
    if models_response=$(curl -s --max-time $HEALTH_TIMEOUT "$OLLAMA_URL/api/tags" 2>/dev/null); then
        if echo "$models_response" | jq -e '.models | length > 0' >/dev/null 2>&1; then
            local model_count
            model_count=$(echo "$models_response" | jq -r '.models | length')
            log_info "✓ Found $model_count model(s) available"
            
            # Check if default model is available
            if echo "$models_response" | jq -e ".models[] | select(.name | startswith(\"$DEFAULT_MODEL\"))" >/dev/null 2>&1; then
                log_info "✓ Default model '$DEFAULT_MODEL' is available"
            else
                log_warning "⚠ Default model '$DEFAULT_MODEL' not found"
            fi
            
            return 0
        else
            log_error "✗ No models available"
            return 1
        fi
    else
        log_error "✗ Failed to retrieve model list"
        return 1
    fi
}

# Function to test model inference
check_inference() {
    log_info "Testing model inference..."
    
    local test_request='{
        "model": "'$DEFAULT_MODEL'",
        "prompt": "'$TEST_PROMPT'",
        "stream": false,
        "options": {
            "num_predict": 5,
            "temperature": 0.1
        }
    }'
    
    local response
    if response=$(curl -s --max-time $((HEALTH_TIMEOUT * 2)) \
                      -X POST "$OLLAMA_URL/api/generate" \
                      -H "Content-Type: application/json" \
                      -d "$test_request" 2>/dev/null); then
        
        if echo "$response" | jq -e '.response' >/dev/null 2>&1; then
            log_info "✓ Model inference working correctly"
            return 0
        else
            log_error "✗ Model inference failed: Invalid response"
            echo "$response" | jq -r '.error // "Unknown error"' >&2
            return 1
        fi
    else
        log_error "✗ Model inference failed: No response"
        return 1
    fi
}

# Function to check system resources
check_resources() {
    log_info "Checking system resources..."
    
    # Check memory usage
    if command -v free >/dev/null 2>&1; then
        local mem_info
        mem_info=$(free -m)
        local mem_used
        local mem_total
        mem_used=$(echo "$mem_info" | awk '/^Mem:/ {print $3}')
        mem_total=$(echo "$mem_info" | awk '/^Mem:/ {print $2}')
        
        if [ -n "$mem_used" ] && [ -n "$mem_total" ] && [ "$mem_total" -gt 0 ]; then
            local mem_percent
            mem_percent=$((mem_used * 100 / mem_total))
            
            if [ "$mem_percent" -lt 90 ]; then
                log_info "✓ Memory usage: ${mem_percent}% (${mem_used}MB/${mem_total}MB)"
            else
                log_warning "⚠ High memory usage: ${mem_percent}% (${mem_used}MB/${mem_total}MB)"
            fi
        fi
    fi
    
    # Check disk space for model storage
    if [ -d "/root/.ollama" ]; then
        local disk_info
        if disk_info=$(df -h /root/.ollama 2>/dev/null); then
            local disk_usage
            disk_usage=$(echo "$disk_info" | awk 'NR==2 {print $5}' | sed 's/%//')
            
            if [ -n "$disk_usage" ] && [ "$disk_usage" -lt 90 ]; then
                log_info "✓ Disk usage: ${disk_usage}%"
            else
                log_warning "⚠ High disk usage: ${disk_usage}%"
            fi
        fi
    fi
    
    return 0
}

# Function to check API endpoints
check_api_endpoints() {
    log_info "Checking API endpoints..."
    
    # Test /api/tags endpoint
    if curl -s -f --max-time $HEALTH_TIMEOUT "$OLLAMA_URL/api/tags" >/dev/null 2>&1; then
        log_info "✓ /api/tags endpoint responding"
    else
        log_error "✗ /api/tags endpoint failed"
        return 1
    fi
    
    # Test /api/version endpoint (if available)
    if curl -s -f --max-time $HEALTH_TIMEOUT "$OLLAMA_URL/api/version" >/dev/null 2>&1; then
        log_info "✓ /api/version endpoint responding"
    else
        log_info "ℹ /api/version endpoint not available (expected for some versions)"
    fi
    
    return 0
}

# Function to generate health status report
generate_health_report() {
    local status=$1
    local timestamp
    timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    local report='{
        "status": "'$status'",
        "timestamp": "'$timestamp'",
        "service": "ollama",
        "version": "containerized",
        "checks": {
            "service_available": null,
            "models_loaded": null,
            "inference_working": null,
            "resources_ok": null,
            "api_endpoints": null
        }
    }'
    
    echo "$report"
}

# Main health check function
main() {
    local exit_code=0
    local overall_status="healthy"
    
    log_info "Starting Ollama health check..."
    
    # Run all health checks
    if ! check_service; then
        exit_code=1
        overall_status="unhealthy"
    fi
    
    if ! check_models; then
        exit_code=1
        overall_status="unhealthy"
    fi
    
    if ! check_api_endpoints; then
        exit_code=1
        overall_status="unhealthy"
    fi
    
    # Only test inference if basic checks pass
    if [ $exit_code -eq 0 ]; then
        if ! check_inference; then
            exit_code=1
            overall_status="degraded"
        fi
    fi
    
    # Check resources (non-critical)
    check_resources || true
    
    # Generate final status
    if [ $exit_code -eq 0 ]; then
        log_info "✅ Ollama health check PASSED - Service is healthy"
    else
        log_error "❌ Ollama health check FAILED - Service is $overall_status"
    fi
    
    # Output health report for monitoring systems
    generate_health_report "$overall_status"
    
    exit $exit_code
}

# Execute main function
main "$@"