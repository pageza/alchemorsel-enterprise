#!/bin/bash

# Docker Development Environment Manager
# Manages different Docker Compose configurations for Alchemorsel v3

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if Docker is running
check_docker() {
    if ! docker info >/dev/null 2>&1; then
        print_error "Docker is not running. Please start Docker and try again."
        exit 1
    fi
}

# Function to check if .env file exists
check_env() {
    if [ ! -f .env ]; then
        print_warning ".env file not found. Creating from .env.example..."
        if [ -f .env.example ]; then
            cp .env.example .env
            print_info "Please edit .env file with your configuration before continuing."
        else
            print_error ".env.example file not found. Please create .env file manually."
            exit 1
        fi
    fi
}

# Function to show usage
show_usage() {
    echo "Usage: $0 {local|enterprise|full} {up|down|restart|logs|status|build}"
    echo ""
    echo "Environments:"
    echo "  local      - Simple setup with just API, Web, PostgreSQL, and Redis"
    echo "  enterprise - Full enterprise setup with monitoring and observability"
    echo "  full       - Complete setup with all services including Nginx proxy"
    echo ""
    echo "Commands:"
    echo "  up         - Start services"
    echo "  down       - Stop and remove services"
    echo "  restart    - Restart services"
    echo "  logs       - Show service logs"
    echo "  status     - Show service status"
    echo "  build      - Build/rebuild services"
    echo "  shell      - Open shell in API container"
    echo ""
    echo "Examples:"
    echo "  $0 local up          # Start local development environment"
    echo "  $0 enterprise logs   # Show logs for enterprise environment"
    echo "  $0 local down        # Stop local environment"
}

# Function to get the correct docker-compose file
get_compose_file() {
    case $1 in
        "local")
            echo "docker-compose.local.yml"
            ;;
        "enterprise")
            echo "docker-compose.enterprise.yml"
            ;;
        "full")
            echo "docker-compose.yml"
            ;;
        *)
            print_error "Unknown environment: $1"
            show_usage
            exit 1
            ;;
    esac
}

# Function to start services
start_services() {
    local env=$1
    local compose_file=$(get_compose_file $env)
    
    print_info "Starting $env environment..."
    print_info "Using compose file: $compose_file"
    
    docker-compose -f $compose_file up -d
    
    print_success "$env environment started successfully!"
    print_info "Services are starting up. Use '$0 $env status' to check health."
    
    case $env in
        "local")
            echo ""
            print_info "Access points:"
            print_info "  Web Frontend: http://localhost:8080"
            print_info "  API Backend:  http://localhost:3000"
            print_info "  PostgreSQL:   localhost:5432"
            print_info "  Redis:        localhost:6379"
            ;;
        "enterprise"|"full")
            echo ""
            print_info "Access points:"
            print_info "  Web Frontend: http://localhost:8080"
            print_info "  API Backend:  http://localhost:3000"
            print_info "  Nginx Proxy:  http://localhost:80"
            print_info "  Grafana:      http://localhost:3001 (admin/admin)"
            print_info "  Prometheus:   http://localhost:9090"
            print_info "  Jaeger:       http://localhost:16686"
            print_info "  MinIO:        http://localhost:9001 (minioadmin/minioadmin)"
            ;;
    esac
}

# Function to stop services
stop_services() {
    local env=$1
    local compose_file=$(get_compose_file $env)
    
    print_info "Stopping $env environment..."
    docker-compose -f $compose_file down
    
    print_success "$env environment stopped successfully!"
}

# Function to restart services
restart_services() {
    local env=$1
    local compose_file=$(get_compose_file $env)
    
    print_info "Restarting $env environment..."
    docker-compose -f $compose_file restart
    
    print_success "$env environment restarted successfully!"
}

# Function to show logs
show_logs() {
    local env=$1
    local compose_file=$(get_compose_file $env)
    
    print_info "Showing logs for $env environment..."
    docker-compose -f $compose_file logs -f
}

# Function to show status
show_status() {
    local env=$1
    local compose_file=$(get_compose_file $env)
    
    print_info "Status for $env environment:"
    docker-compose -f $compose_file ps
}

# Function to build services
build_services() {
    local env=$1
    local compose_file=$(get_compose_file $env)
    
    print_info "Building services for $env environment..."
    docker-compose -f $compose_file build --no-cache
    
    print_success "Services built successfully!"
}

# Function to open shell in API container
open_shell() {
    local env=$1
    local compose_file=$(get_compose_file $env)
    
    print_info "Opening shell in API container..."
    
    if [ "$env" = "local" ]; then
        docker-compose -f $compose_file exec api-backend sh
    else
        docker-compose -f $compose_file exec api-backend sh
    fi
}

# Main script logic
if [ $# -lt 2 ]; then
    show_usage
    exit 1
fi

ENV=$1
COMMAND=$2

# Check prerequisites
check_docker

# Don't check .env for certain commands
if [[ "$COMMAND" != "down" && "$COMMAND" != "status" && "$COMMAND" != "logs" ]]; then
    check_env
fi

# Execute command
case $COMMAND in
    "up")
        start_services $ENV
        ;;
    "down")
        stop_services $ENV
        ;;
    "restart")
        restart_services $ENV
        ;;
    "logs")
        show_logs $ENV
        ;;
    "status")
        show_status $ENV
        ;;
    "build")
        build_services $ENV
        ;;
    "shell")
        open_shell $ENV
        ;;
    *)
        print_error "Unknown command: $COMMAND"
        show_usage
        exit 1
        ;;
esac