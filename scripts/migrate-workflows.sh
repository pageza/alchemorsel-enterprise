#!/bin/bash

# Alchemorsel v3 - Workflow Migration Script
# This script helps migrate from complex enterprise workflows to simplified Docker Compose workflows

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
WORKFLOWS_DIR="${PROJECT_ROOT}/.github/workflows"

echo "🔄 Alchemorsel v3 - CI/CD Workflow Migration"
echo "============================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${GREEN}✅${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠️${NC} $1"
}

print_error() {
    echo -e "${RED}❌${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ️${NC} $1"
}

# Check if we're in the right directory
if [[ ! -f "${PROJECT_ROOT}/docker-compose.yml" ]]; then
    print_error "This doesn't appear to be the Alchemorsel project root"
    print_error "Please run this script from the project root or scripts directory"
    exit 1
fi

echo
echo "Current workflow files:"
ls -la "${WORKFLOWS_DIR}/"*.yml 2>/dev/null || echo "No workflow files found"

echo
print_info "This will:"
print_info "1. Backup existing complex workflows"
print_info "2. Activate simplified workflows"
print_info "3. Update workflow documentation"

echo
read -p "Continue? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Migration cancelled"
    exit 0
fi

# Create backup directory
BACKUP_DIR="${WORKFLOWS_DIR}/backup-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"

# Backup existing workflows
echo
print_info "Backing up existing workflows..."
for workflow in "${WORKFLOWS_DIR}"/*.yml; do
    if [[ -f "$workflow" && ! "$workflow" =~ (ci-simple|cd-simple) ]]; then
        filename=$(basename "$workflow")
        cp "$workflow" "${BACKUP_DIR}/"
        print_status "Backed up: $filename"
    fi
done

# Disable old workflows by renaming them
print_info "Disabling old workflows..."
for workflow in "${WORKFLOWS_DIR}"/*.yml; do
    if [[ -f "$workflow" && ! "$workflow" =~ (ci-simple|cd-simple) ]]; then
        filename=$(basename "$workflow" .yml)
        mv "$workflow" "${workflow%.yml}.yml.disabled"
        print_warning "Disabled: ${filename}.yml → ${filename}.yml.disabled"
    fi
done

# Activate simplified workflows by renaming them
print_info "Activating simplified workflows..."
if [[ -f "${WORKFLOWS_DIR}/ci-simple.yml" ]]; then
    mv "${WORKFLOWS_DIR}/ci-simple.yml" "${WORKFLOWS_DIR}/ci.yml"
    print_status "Activated: ci-simple.yml → ci.yml"
fi

if [[ -f "${WORKFLOWS_DIR}/cd-simple.yml" ]]; then
    mv "${WORKFLOWS_DIR}/cd-simple.yml" "${WORKFLOWS_DIR}/cd.yml"
    print_status "Activated: cd-simple.yml → cd.yml"
fi

# Create workflow documentation
cat > "${WORKFLOWS_DIR}/README.md" << 'EOF'
# Alchemorsel v3 - CI/CD Workflows

## Current Active Workflows

### `ci.yml` - Continuous Integration
- **Triggers**: Push to main/master/develop/feature/* branches, PRs to main/master
- **Services**: PostgreSQL, Redis
- **Tests**: Go tests with race detection and coverage
- **Security**: Gosec security scanning
- **Build**: Docker images for web and api services
- **Integration**: Docker Compose integration testing

### `cd.yml` - Continuous Deployment  
- **Triggers**: Push to main/master, manual dispatch, tags
- **Environments**: Staging (automatic), Production (manual approval)
- **Deployment**: Docker Compose via SSH
- **Features**: Rolling deployments, health checks, rollback capability

## Migration History

The original workflows were complex enterprise-grade workflows designed for:
- Kubernetes/EKS deployment
- AWS infrastructure with Terraform
- Separate microservices architecture
- Complex monitoring and performance testing

These have been replaced with simplified workflows matching our current:
- Docker Compose deployment model  
- Monolithic web application with HTMX
- Local/VPS deployment targets
- Session-based architecture

## Disabled Workflows

Original workflows have been renamed to `*.yml.disabled` and backed up to:
`backup-YYYYMMDD-HHMMSS/`

To restore original workflows:
```bash
# Disable current simplified workflows
mv ci.yml ci-simple.yml.disabled
mv cd.yml cd-simple.yml.disabled

# Restore from backup (replace DATE with actual backup folder)
cp backup-DATE/*.yml ./
```

## Required GitHub Secrets

### Staging Deployment
- `STAGING_HOST` - Staging server hostname/IP
- `STAGING_USER` - SSH username for staging server  
- `STAGING_SSH_KEY` - SSH private key for staging server
- `STAGING_PORT` - SSH port (optional, defaults to 22)
- `STAGING_URL` - Base URL for staging environment

### Production Deployment  
- `PRODUCTION_HOST` - Production server hostname/IP
- `PRODUCTION_USER` - SSH username for production server
- `PRODUCTION_SSH_KEY` - SSH private key for production server  
- `PRODUCTION_PORT` - SSH port (optional, defaults to 22)
- `PRODUCTION_URL` - Base URL for production environment

### Application Secrets
- `POSTGRES_USER` - Production PostgreSQL username
- `POSTGRES_PASSWORD` - Production PostgreSQL password  
- `REDIS_PASSWORD` - Production Redis password
- `SESSION_SECRET` - Session encryption secret
- `CSRF_SECRET` - CSRF token secret
- `OLLAMA_HOST` - Ollama AI service host

## Testing Locally

Test the CI workflow locally using [act](https://github.com/nektos/act):

```bash
# Install act (GitHub Actions runner)
curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash

# Run CI workflow locally
act -W .github/workflows/ci.yml

# Run specific job
act -W .github/workflows/ci.yml -j test
```

## Workflow Customization

### Adding New Tests
Edit the `test` job in `ci.yml` to add additional test commands:

```yaml
- name: Run custom tests
  run: |
    go test -tags=integration ./...
    # Add your test commands here
```

### Adding Deployment Steps  
Edit the deployment jobs in `cd.yml` to add custom deployment steps:

```yaml
- name: Custom deployment step
  run: |
    # Your deployment commands here
```

### Environment-Specific Configuration
Create environment-specific docker-compose files:
- `docker-compose.staging.yml`
- `docker-compose.production.yml`

Then update the deployment scripts to use them:
```bash
docker compose -f docker-compose.yml -f docker-compose.production.yml up -d
```
EOF

print_status "Created workflow documentation: ${WORKFLOWS_DIR}/README.md"

# Summary
echo
echo "🎉 Migration Complete!"
echo "======================"
print_status "Backed up original workflows to: $(basename "$BACKUP_DIR")"
print_status "Activated simplified CI/CD workflows"  
print_status "Created workflow documentation"

echo
print_info "Next steps:"
print_info "1. Review and customize the new workflows in .github/workflows/"
print_info "2. Add required GitHub secrets for deployment"
print_info "3. Test the CI workflow on your next push"
print_info "4. Set up staging and production servers for CD"

echo
print_warning "Important: The old workflows assumed a different architecture"
print_warning "The new workflows are designed for your current Docker Compose setup"
print_warning "You may need to adjust deployment URLs and server configurations"

echo
echo "Workflow migration completed successfully! 🚀"