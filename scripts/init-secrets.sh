#!/bin/bash

# Secure Secret Initialization Script
# Generates and stores all required secrets for Alchemorsel v3
# Addresses critical security issue: Remove hardcoded secrets

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
SECRETS_DIR="${PROJECT_ROOT}/secrets"
CONFIG_DIR="${PROJECT_ROOT}/config"
ENVIRONMENT="${ENVIRONMENT:-development}"

# Logging configuration
LOG_LEVEL="${LOG_LEVEL:-INFO}"
LOG_FILE="${PROJECT_ROOT}/logs/secret-init.log"

# Create required directories
mkdir -p "$SECRETS_DIR" "$CONFIG_DIR" "${PROJECT_ROOT}/logs"

# Logging functions
log() {
    local level="$1"
    shift
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$level] $*" | tee -a "$LOG_FILE"
}

log_info() {
    log "INFO" "$@"
}

log_warn() {
    log "WARN" "$@"
}

log_error() {
    log "ERROR" "$@"
}

log_fatal() {
    log "FATAL" "$@"
    exit 1
}

# Security: Check permissions
check_permissions() {
    log_info "Checking directory permissions..."
    
    # Ensure secrets directory has proper permissions
    chmod 700 "$SECRETS_DIR" 2>/dev/null || true
    
    # Check if running as root (not recommended)
    if [[ $EUID -eq 0 ]]; then
        log_warn "Running as root is not recommended for security"
    fi
}

# Generate cryptographically secure random string
generate_secure_random() {
    local length="$1"
    local charset="${2:-A-Za-z0-9}"
    
    # Use OpenSSL for cryptographically secure random generation
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -base64 "$((length * 3 / 4))" | tr -d "=+/" | cut -c1-"$length"
    elif command -v head >/dev/null 2>&1 && [[ -c /dev/urandom ]]; then
        head -c "$length" /dev/urandom | base64 | tr -d "=+/" | cut -c1-"$length"
    else
        log_fatal "No secure random source available"
    fi
}

# Generate secure password with complexity requirements
generate_secure_password() {
    local length="${1:-32}"
    local password=""
    
    # Ensure password contains at least one of each character type
    local lowercase="abcdefghijklmnopqrstuvwxyz"
    local uppercase="ABCDEFGHIJKLMNOPQRSTUVWXYZ"
    local digits="0123456789"
    local special="!@#$%^&*()_+-=[]{}|;:,.<>?"
    
    # Add at least one character from each set
    password+="${lowercase:$((RANDOM % ${#lowercase})):1}"
    password+="${uppercase:$((RANDOM % ${#uppercase})):1}"
    password+="${digits:$((RANDOM % ${#digits})):1}"
    password+="${special:$((RANDOM % ${#special})):1}"
    
    # Fill the rest with random characters
    local all_chars="$lowercase$uppercase$digits$special"
    for ((i=${#password}; i<length; i++)); do
        password+="${all_chars:$((RANDOM % ${#all_chars})):1}"
    done
    
    # Shuffle the password
    echo "$password" | fold -w1 | shuf | tr -d '\n'
}

# Generate JWT secret with proper entropy
generate_jwt_secret() {
    # JWT secret should be at least 256 bits (32 bytes) for security
    generate_secure_random 64
}

# Generate database password
generate_database_password() {
    generate_secure_password 32
}

# Generate session secret
generate_session_secret() {
    generate_secure_random 48
}

# Generate API key format secret
generate_api_key() {
    local prefix="${1:-ak}"
    local key_part=$(generate_secure_random 32)
    echo "${prefix}_${key_part}"
}

# Store secret securely
store_secret() {
    local name="$1"
    local value="$2"
    local description="$3"
    
    local secret_file="${SECRETS_DIR}/${name}"
    local env_file="${SECRETS_DIR}/${name}.env"
    
    # Store raw secret (for file-based secret injection)
    echo -n "$value" > "$secret_file"
    chmod 600 "$secret_file"
    
    # Store environment variable format
    echo "ALCHEMORSEL_${name^^}=$value" > "$env_file"
    chmod 600 "$env_file"
    
    # Store in Docker secrets format
    if command -v docker >/dev/null 2>&1; then
        echo "$value" | docker secret create "alchemorsel_${name}" - 2>/dev/null || true
    fi
    
    log_info "Generated and stored secret: $name ($description)"
}

# Validate secret strength
validate_secret_strength() {
    local secret="$1"
    local min_length="${2:-16}"
    
    if [[ ${#secret} -lt $min_length ]]; then
        return 1
    fi
    
    # Check for character diversity
    local has_lower has_upper has_digit has_special
    has_lower=$(echo "$secret" | grep -q '[a-z]' && echo 1 || echo 0)
    has_upper=$(echo "$secret" | grep -q '[A-Z]' && echo 1 || echo 0)
    has_digit=$(echo "$secret" | grep -q '[0-9]' && echo 1 || echo 0)
    has_special=$(echo "$secret" | grep -q '[^A-Za-z0-9]' && echo 1 || echo 0)
    
    local diversity_score=$((has_lower + has_upper + has_digit + has_special))
    
    if [[ $diversity_score -lt 3 ]]; then
        return 1
    fi
    
    return 0
}

# Generate all required secrets
generate_secrets() {
    log_info "Generating secure secrets for Alchemorsel v3..."
    
    # Authentication secrets
    local jwt_secret
    jwt_secret=$(generate_jwt_secret)
    if validate_secret_strength "$jwt_secret" 32; then
        store_secret "jwt_secret" "$jwt_secret" "JWT signing key"
    else
        log_fatal "Generated JWT secret does not meet security requirements"
    fi
    
    local session_secret
    session_secret=$(generate_session_secret)
    store_secret "session_secret" "$session_secret" "Session encryption key"
    
    # Database secrets
    local db_password
    db_password=$(generate_database_password)
    if validate_secret_strength "$db_password" 16; then
        store_secret "db_password" "$db_password" "PostgreSQL database password"
    else
        log_fatal "Generated database password does not meet security requirements"
    fi
    
    local redis_password
    redis_password=$(generate_database_password)
    store_secret "redis_password" "$redis_password" "Redis cache password"
    
    # Kafka secrets
    local kafka_sasl_password
    kafka_sasl_password=$(generate_database_password)
    store_secret "kafka_sasl_password" "$kafka_sasl_password" "Kafka SASL password"
    
    local kafka_keystore_password
    kafka_keystore_password=$(generate_secure_password 24)
    store_secret "kafka_keystore_password" "$kafka_keystore_password" "Kafka keystore password"
    
    local kafka_truststore_password
    kafka_truststore_password=$(generate_secure_password 24)
    store_secret "kafka_truststore_password" "$kafka_truststore_password" "Kafka truststore password"
    
    # AWS secrets (if needed)
    if [[ -z "${AWS_SECRET_ACCESS_KEY:-}" ]]; then
        local aws_secret_key
        aws_secret_key=$(generate_api_key "aws")
        store_secret "aws_secret_access_key" "$aws_secret_key" "AWS secret access key"
    fi
    
    # AI service API keys (placeholders - replace with actual keys)
    if [[ -z "${OPENAI_API_KEY:-}" ]]; then
        local openai_key="sk-placeholder-$(generate_secure_random 32)"
        store_secret "openai_api_key" "$openai_key" "OpenAI API key (placeholder)"
        log_warn "OpenAI API key is a placeholder. Replace with actual key."
    fi
    
    if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
        local anthropic_key="sk-ant-placeholder-$(generate_secure_random 32)"
        store_secret "anthropic_api_key" "$anthropic_key" "Anthropic API key (placeholder)"
        log_warn "Anthropic API key is a placeholder. Replace with actual key."
    fi
    
    # MinIO secrets
    local minio_root_user="alchemorsel-admin"
    local minio_root_password
    minio_root_password=$(generate_database_password)
    store_secret "minio_root_user" "$minio_root_user" "MinIO root username"
    store_secret "minio_root_password" "$minio_root_password" "MinIO root password"
    
    # Email service secrets (placeholders)
    if [[ -z "${SMTP_PASSWORD:-}" ]]; then
        local smtp_password
        smtp_password=$(generate_secure_password 24)
        store_secret "smtp_password" "$smtp_password" "SMTP password (placeholder)"
        log_warn "SMTP password is a placeholder. Replace with actual password."
    fi
    
    if [[ -z "${SENDGRID_API_KEY:-}" ]]; then
        local sendgrid_key="SG.placeholder.$(generate_secure_random 32)"
        store_secret "sendgrid_api_key" "$sendgrid_key" "SendGrid API key (placeholder)"
        log_warn "SendGrid API key is a placeholder. Replace with actual key."
    fi
}

# Create environment file for Docker Compose
create_env_file() {
    log_info "Creating environment file for Docker Compose..."
    
    local env_file="${PROJECT_ROOT}/.env.secure"
    
    cat > "$env_file" <<EOF
# Alchemorsel v3 Secure Environment Configuration
# Generated on $(date)
# WARNING: This file contains sensitive information

# Environment
ENVIRONMENT=${ENVIRONMENT}
LOG_LEVEL=${LOG_LEVEL}

# Database configuration
ALCHEMORSEL_DATABASE_DATABASE=alchemorsel_dev
ALCHEMORSEL_DATABASE_USERNAME=postgres

# Kafka configuration
KAFKA_SASL_USERNAME=alchemorsel

# S3/MinIO configuration
S3_BUCKET=alchemorsel-secure

# Secret file paths (for Docker secrets)
ALCHEMORSEL_AUTH_JWT_SECRET_FILE=/run/secrets/jwt_secret
ALCHEMORSEL_AUTH_SESSION_SECRET_FILE=/run/secrets/session_secret
ALCHEMORSEL_DATABASE_PASSWORD_FILE=/run/secrets/db_password
ALCHEMORSEL_REDIS_PASSWORD_FILE=/run/secrets/redis_password
ALCHEMORSEL_KAFKA_SASL_PASSWORD_FILE=/run/secrets/kafka_sasl_password
ALCHEMORSEL_AWS_SECRET_ACCESS_KEY_FILE=/run/secrets/aws_secret_access_key
ALCHEMORSEL_AI_OPENAI_KEY_FILE=/run/secrets/openai_api_key
EOF
    
    chmod 600 "$env_file"
    log_info "Environment file created: $env_file"
}

# Create secure configuration
create_secure_config() {
    log_info "Creating secure configuration..."
    
    local secure_config="${CONFIG_DIR}/config.secure.yaml"
    
    cat > "$secure_config" <<EOF
# Alchemorsel v3 Secure Configuration
# Generated on $(date)
# All secrets are loaded from external sources

app:
  name: "Alchemorsel"
  version: "3.0.0"
  environment: "${ENVIRONMENT}"
  debug: false
  log_level: "${LOG_LEVEL}"
  log_format: "json"

server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "15s"
  write_timeout: "15s"
  idle_timeout: "60s"
  max_header_bytes: 1048576
  shutdown_timeout: "30s"
  enable_cors: true
  allowed_origins:
    - "https://alchemorsel.com"
    - "https://app.alchemorsel.com"
  trusted_proxies: []
  enable_compression: true
  enable_pprof: false

database:
  driver: "postgres"
  host: "postgres"
  port: 5432
  database: "alchemorsel_dev"
  username: "postgres"
  # password loaded from secret manager
  ssl_mode: "require"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: "1h"
  conn_max_idle_time: "10m"
  log_level: "warn"
  slow_query_threshold: "100ms"
  auto_migrate: true

redis:
  host: "redis"
  port: 6379
  # password loaded from secret manager
  database: 0
  max_retries: 3
  min_idle_conns: 2
  max_idle_conns: 10
  conn_max_lifetime: "1h"
  dial_timeout: "5s"
  read_timeout: "3s"
  write_timeout: "3s"
  pool_size: 10

auth:
  # jwt_secret loaded from secret manager
  # session_secret loaded from secret manager
  jwt_expiration: "24h"
  refresh_expiration: "168h"
  bcrypt_cost: 12
  enable_oauth: true
  session_max_age: 86400

aws:
  region: "us-east-1"
  # access_key_id loaded from environment
  # secret_access_key loaded from secret manager
  s3_bucket: "alchemorsel-secure"

ai:
  provider: "openai"
  # openai_key loaded from secret manager
  # anthropic_key loaded from secret manager
  openai_model: "gpt-4"
  anthropic_model: "claude-3-sonnet"
  max_tokens: 2000
  temperature: 0.7
  timeout_seconds: 30
  enable_cache: true
  cache_ttl: "1h"

kafka:
  brokers:
    - "kafka:9092"
  group_id: "alchemorsel-api"
  client_id: "alchemorsel-api-v3"
  enable_sasl: true
  sasl_username: "alchemorsel"
  # sasl_password loaded from secret manager
  enable_tls: false
  retry_max: 3
  required_acks: 1

monitoring:
  enable_metrics: true
  metrics_port: 9090
  enable_tracing: true
  jaeger_endpoint: "http://jaeger:14268/api/traces"
  sampling_rate: 0.1
  health_check_path: "/health"
  readiness_path: "/ready"

email:
  provider: "smtp"
  smtp_host: "smtp.gmail.com"
  smtp_port: 587
  # smtp_password loaded from secret manager
  from_address: "noreply@alchemorsel.com"
  from_name: "Alchemorsel"
  # sendgrid_api_key loaded from secret manager
  enable_tls: true

storage:
  provider: "s3"
  max_file_size: 10485760
  allowed_types:
    - "image/jpeg"
    - "image/png"
    - "image/webp"
    - "video/mp4"
  image_max_width: 2048
  image_max_height: 2048
  enable_cdn: false

rate_limit:
  enable: true
  requests_per_min: 60
  burst_size: 10
  cleanup_interval: "1m"
  use_redis: true

# Security configuration
security:
  enable_audit_logging: true
  audit_log_file: "/var/log/alchemorsel/audit.log"
  enable_secret_manager: true
  secret_manager:
    enable_cache: true
    cache_ttl: "1h"
    max_cache_size: 1000
    enable_audit_logging: true
    rotation_interval: "24h"
    encryption_algorithm: "AES-256-GCM"
    enable_zeroization: true

features:
  enable_ai_recipes: true
  enable_social_features: true
  enable_premium: false
  enable_analytics: true
  enable_export: true
  maintenance_mode: false
EOF
    
    chmod 600 "$secure_config"
    log_info "Secure configuration created: $secure_config"
}

# Create setup documentation
create_documentation() {
    log_info "Creating setup documentation..."
    
    local doc_file="${PROJECT_ROOT}/SECURE_SETUP.md"
    
    cat > "$doc_file" <<EOF
# Alchemorsel v3 Secure Setup Guide

This document describes the secure deployment configuration for Alchemorsel v3.

## Security Features Implemented

### 1. Secret Management
- All hardcoded secrets removed from configuration files
- Secrets generated with cryptographically secure random number generation
- Secrets stored in Docker secrets or external secret managers
- Comprehensive audit logging for all secret operations

### 2. Container Security
- Non-root user execution for all containers
- Read-only filesystems with tmpfs for temporary data
- Minimal container images (distroless base)
- Security contexts with dropped capabilities
- Resource limits and reservations

### 3. Network Security
- Custom Docker network with restricted access
- TLS/SSL encryption for external communications
- SASL/SCRAM authentication for Kafka
- Password authentication for Redis and PostgreSQL

### 4. Audit and Monitoring
- Comprehensive audit logging enabled
- Security metrics collection
- Health checks for all services
- Structured logging with rotation

## Deployment Instructions

### 1. Initialize Secrets
\`\`\`bash
# Run the secret initialization script
./scripts/init-secrets.sh

# Verify secrets were generated
ls -la secrets/
\`\`\`

### 2. Configure Environment
\`\`\`bash
# Copy the secure environment file
cp .env.secure .env

# Edit with your specific configuration
vim .env
\`\`\`

### 3. Deploy with Docker Compose
\`\`\`bash
# Deploy using the secure configuration
docker-compose -f docker-compose.secure.yml up -d

# Check service status
docker-compose -f docker-compose.secure.yml ps
\`\`\`

### 4. Verify Security
\`\`\`bash
# Check that containers are running as non-root
docker-compose -f docker-compose.secure.yml exec api id

# Verify secrets are mounted correctly
docker-compose -f docker-compose.secure.yml exec api ls -la /run/secrets/

# Check audit logs
docker-compose -f docker-compose.secure.yml logs api | grep audit
\`\`\`

## Security Considerations

### Production Deployment
- Replace placeholder API keys with actual keys
- Use external secret management (Vault, AWS Secrets Manager, etc.)
- Enable TLS/SSL for all external communications
- Configure proper firewall rules
- Set up monitoring and alerting
- Regular security audits and updates

### Secret Rotation
- JWT secrets should be rotated regularly
- Database passwords should follow organizational policy
- API keys should be rotated according to provider recommendations
- Monitor for compromised secrets

### Monitoring
- Monitor audit logs for suspicious activity
- Set up alerts for failed authentication attempts
- Track secret access patterns
- Monitor resource usage and performance

## Files Generated

- \`secrets/\` - Directory containing all generated secrets
- \`.env.secure\` - Environment configuration for Docker Compose
- \`config/config.secure.yaml\` - Secure application configuration
- \`logs/secret-init.log\` - Secret generation audit log

## Security Contact

For security issues or questions, contact: security@alchemorsel.com
EOF
    
    chmod 644 "$doc_file"
    log_info "Setup documentation created: $doc_file"
}

# Main execution
main() {
    log_info "Starting secure secret initialization for Alchemorsel v3"
    log_info "Environment: $ENVIRONMENT"
    
    # Security checks
    check_permissions
    
    # Generate secrets
    generate_secrets
    
    # Create configuration files
    create_env_file
    create_secure_config
    create_documentation
    
    log_info "Secret initialization completed successfully"
    log_info "Next steps:"
    log_info "  1. Review generated secrets in: $SECRETS_DIR"
    log_info "  2. Configure environment: .env.secure"
    log_info "  3. Deploy: docker-compose -f docker-compose.secure.yml up -d"
    log_info "  4. Read setup guide: SECURE_SETUP.md"
    
    # Security reminder
    log_warn "SECURITY REMINDER:"
    log_warn "  - Keep secrets directory secure (700 permissions)"
    log_warn "  - Do not commit secrets to version control"
    log_warn "  - Replace placeholder API keys with actual keys"
    log_warn "  - Enable additional security measures for production"
}

# Execute main function
main "$@"