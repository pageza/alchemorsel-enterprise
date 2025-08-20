#!/bin/bash
# Ollama Development Management Script for Alchemorsel v3
# Provides easy commands for managing the containerized Ollama service

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COMPOSE_FILE="docker-compose.services.yml"
SERVICE_NAME="ollama"
CONTAINER_NAME="alchemorsel-ollama"
DEFAULT_MODEL="llama3.2:3b"

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

# Function to check if Docker is running
check_docker() {
    if ! docker info >/dev/null 2>&1; then
        log_error "Docker is not running. Please start Docker first."
        exit 1
    fi
}

# Function to check if compose file exists
check_compose_file() {
    if [ ! -f "$COMPOSE_FILE" ]; then
        log_error "Docker Compose file ($COMPOSE_FILE) not found in current directory."
        exit 1
    fi
}

# Function to start Ollama service
start_service() {
    log_info "Starting Ollama service..."
    
    check_docker
    check_compose_file
    
    # Start dependencies first
    log_info "Starting dependencies (postgres, redis)..."
    docker compose -f "$COMPOSE_FILE" up -d postgres redis
    
    # Wait for dependencies
    log_info "Waiting for dependencies to be ready..."
    sleep 10
    
    # Start Ollama service
    log_info "Starting Ollama service..."
    docker compose -f "$COMPOSE_FILE" up -d "$SERVICE_NAME"
    
    # Wait for service to be ready
    log_info "Waiting for Ollama service to initialize..."
    wait_for_service
    
    log_success "Ollama service started successfully!"
    show_status
}

# Function to stop Ollama service
stop_service() {
    log_info "Stopping Ollama service..."
    
    docker compose -f "$COMPOSE_FILE" stop "$SERVICE_NAME"
    
    log_success "Ollama service stopped."
}

# Function to restart Ollama service
restart_service() {
    log_info "Restarting Ollama service..."
    
    stop_service
    sleep 2
    start_service
}

# Function to wait for service to be ready
wait_for_service() {
    local max_attempts=30
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if docker exec "$CONTAINER_NAME" curl -s -f http://localhost:11434/api/tags >/dev/null 2>&1; then
            log_success "Ollama service is ready!"
            return 0
        fi
        
        log_info "Waiting for Ollama service... (attempt $attempt/$max_attempts)"
        sleep 10
        attempt=$((attempt + 1))
    done
    
    log_error "Ollama service failed to become ready within expected time"
    return 1
}

# Function to show service status
show_status() {
    log_info "=== Ollama Service Status ==="
    
    # Container status
    if docker ps --filter "name=$CONTAINER_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -q "$CONTAINER_NAME"; then
        log_success "Container is running:"
        docker ps --filter "name=$CONTAINER_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    else
        log_warning "Container is not running"
        return 1
    fi
    
    # Health check status
    local health_status
    health_status=$(docker inspect "$CONTAINER_NAME" --format='{{.State.Health.Status}}' 2>/dev/null || echo "unknown")
    log_info "Health status: $health_status"
    
    # API status
    if docker exec "$CONTAINER_NAME" curl -s -f http://localhost:11434/api/tags >/dev/null 2>&1; then
        log_success "API is responding"
        
        # List available models
        log_info "Available models:"
        docker exec "$CONTAINER_NAME" ollama list 2>/dev/null || log_warning "Failed to list models"
    else
        log_warning "API is not responding"
    fi
    
    echo "================================"
}

# Function to show logs
show_logs() {
    local follow_flag=""
    if [ "$1" = "-f" ] || [ "$1" = "--follow" ]; then
        follow_flag="-f"
        log_info "Following Ollama service logs (Ctrl+C to stop)..."
    else
        log_info "Showing recent Ollama service logs..."
    fi
    
    docker compose -f "$COMPOSE_FILE" logs $follow_flag "$SERVICE_NAME"
}

# Function to execute commands in Ollama container
exec_command() {
    if [ $# -eq 0 ]; then
        log_info "Opening interactive shell in Ollama container..."
        docker exec -it "$CONTAINER_NAME" /bin/bash
    else
        log_info "Executing command in Ollama container: $*"
        docker exec -it "$CONTAINER_NAME" "$@"
    fi
}

# Function to pull models
pull_model() {
    local model=${1:-$DEFAULT_MODEL}
    
    log_info "Pulling model: $model"
    
    if ! docker exec "$CONTAINER_NAME" ollama pull "$model"; then
        log_error "Failed to pull model: $model"
        return 1
    fi
    
    log_success "Model pulled successfully: $model"
    
    # Test the model
    log_info "Testing model..."
    if docker exec "$CONTAINER_NAME" ollama run "$model" "Hello" >/dev/null 2>&1; then
        log_success "Model is working correctly"
    else
        log_warning "Model pull succeeded but test failed"
    fi
}

# Function to list models
list_models() {
    log_info "Available models in Ollama:"
    docker exec "$CONTAINER_NAME" ollama list
}

# Function to remove model
remove_model() {
    local model=$1
    
    if [ -z "$model" ]; then
        log_error "Model name is required for removal"
        exit 1
    fi
    
    log_warning "Removing model: $model"
    
    if docker exec "$CONTAINER_NAME" ollama rm "$model"; then
        log_success "Model removed successfully: $model"
    else
        log_error "Failed to remove model: $model"
        return 1
    fi
}

# Function to test AI functionality
test_ai() {
    log_info "Testing AI functionality..."
    
    # Test recipe generation endpoint
    log_info "Testing recipe generation..."
    local api_response
    if api_response=$(curl -s -X POST http://localhost:3010/api/v1/ai/generate-recipe \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer test-token" \
        -d '{
            "prompt": "pasta with tomatoes",
            "max_calories": 500,
            "dietary": ["vegetarian"]
        }' 2>/dev/null); then
        
        if echo "$api_response" | jq -e '.success' >/dev/null 2>&1; then
            log_success "Recipe generation test passed"
            echo "$api_response" | jq '.data.title' 2>/dev/null || true
        else
            log_warning "Recipe generation test failed - API returned error"
            echo "$api_response" | jq '.error' 2>/dev/null || echo "$api_response"
        fi
    else
        log_warning "Recipe generation test failed - API not accessible"
    fi
}

# Function to show health
health_check() {
    log_info "Performing comprehensive health check..."
    
    # Container health
    if docker ps --filter "name=$CONTAINER_NAME" --filter "status=running" | grep -q "$CONTAINER_NAME"; then
        log_success "✓ Container is running"
    else
        log_error "✗ Container is not running"
        return 1
    fi
    
    # Service health
    if docker exec "$CONTAINER_NAME" /health/healthcheck.sh >/dev/null 2>&1; then
        log_success "✓ Service health check passed"
    else
        log_error "✗ Service health check failed"
        return 1
    fi
    
    # API health
    if curl -s -f http://localhost:11435/api/tags >/dev/null 2>&1; then
        log_success "✓ API is accessible externally"
    else
        log_warning "⚠ API not accessible externally (may be normal if not exposed)"
    fi
    
    # Model availability
    local model_count
    model_count=$(docker exec "$CONTAINER_NAME" ollama list 2>/dev/null | grep -c "^" || echo "0")
    if [ "$model_count" -gt 1 ]; then
        log_success "✓ Models are available ($((model_count - 1)) models)"
    else
        log_warning "⚠ No models available"
    fi
    
    log_success "Health check completed"
}

# Function to show usage
show_usage() {
    echo "Ollama Development Management Script for Alchemorsel v3"
    echo ""
    echo "Usage: $0 <command> [options]"
    echo ""
    echo "Commands:"
    echo "  start              Start Ollama service and dependencies"
    echo "  stop               Stop Ollama service"
    echo "  restart            Restart Ollama service"
    echo "  status             Show service status and health"
    echo "  logs [-f|--follow] Show service logs"
    echo "  exec [command]     Execute command in container (or open shell)"
    echo "  pull [model]       Pull a model (default: $DEFAULT_MODEL)"
    echo "  list               List available models"
    echo "  remove <model>     Remove a model"
    echo "  test               Test AI functionality"
    echo "  health             Perform comprehensive health check"
    echo "  help               Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 start"
    echo "  $0 logs -f"
    echo "  $0 pull llama3.2:1b"
    echo "  $0 exec ollama list"
    echo "  $0 test"
}

# Main script logic
main() {
    case "${1:-help}" in
        start)
            start_service
            ;;
        stop)
            stop_service
            ;;
        restart)
            restart_service
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs "$2"
            ;;
        exec)
            shift
            exec_command "$@"
            ;;
        pull)
            pull_model "$2"
            ;;
        list)
            list_models
            ;;
        remove|rm)
            remove_model "$2"
            ;;
        test)
            test_ai
            ;;
        health)
            health_check
            ;;
        help|--help|-h)
            show_usage
            ;;
        *)
            log_error "Unknown command: $1"
            echo ""
            show_usage
            exit 1
            ;;
    esac
}

# Execute main function with all arguments
main "$@"