# ADR-0011: Environment Variable Management

## Status
Accepted

## Context
Alchemorsel v3 requires secure, consistent configuration management across development, testing, and production environments. Configuration includes database credentials, API keys, feature flags, and environment-specific settings. Poor configuration management leads to security vulnerabilities and deployment issues.

Configuration needs:
- Database connection strings with credentials
- Third-party API keys (OpenAI, payment processors)
- Environment-specific feature flags
- Security tokens and encryption keys
- Service endpoints and timeout configurations
- Performance tuning parameters

Security requirements:
- No secrets in version control
- Encrypted storage for production secrets
- Access controls for sensitive configuration
- Audit logging for configuration changes
- Rotation procedures for credentials

## Decision
We will implement a structured environment variable management system using .env files for development and secure secret management for production.

**Environment Variable Structure:**

**Development (.env files):**
```
# Database
DATABASE_URL=postgresql://user:pass@localhost:5432/alchemorsel_dev
DATABASE_MAX_CONNECTIONS=25

# Redis
REDIS_URL=redis://localhost:6379/0
REDIS_MAX_CONNECTIONS=10

# AI Services
OPENAI_API_KEY=sk-dev-key-here
OLLAMA_BASE_URL=http://localhost:11434

# Application
APP_ENV=development
APP_PORT=8080
APP_SECRET_KEY=dev-secret-key
LOG_LEVEL=debug

# Feature Flags
ENABLE_AI_FEATURES=true
ENABLE_CACHING=true
```

**Production (Docker Secrets/Environment):**
- Secrets managed via Docker Secrets or cloud provider secret management
- Environment variables for non-sensitive configuration
- Separate secret rotation procedures
- Encrypted storage at rest

**Configuration Loading:**
- `.env` files loaded in development environment only
- Environment variables override .env file values
- Validation of required variables on application startup
- Type conversion and validation for all configuration values

**File Structure:**
- `.env.example` - Template with all required variables (committed)
- `.env` - Local development values (never committed)
- `.env.test` - Test environment overrides (committed, no secrets)
- `.env.production.example` - Production template (committed, no secrets)

**Security Requirements:**
- All `.env` files with real values in .gitignore
- No production secrets in development .env files
- Secret rotation procedures documented
- Access logging for secret management operations

## Consequences

### Positive
- Clear separation between development and production secrets
- Consistent configuration across all environments
- Easy onboarding with .env.example templates
- Secure production deployment with proper secret management
- Environment-specific feature flag support

### Negative
- Additional complexity in configuration management
- Risk of accidentally committing secrets
- Requires discipline in .env file management
- Production secret rotation requires operational procedures

### Neutral
- Industry standard configuration management approach
- Compatible with Docker Compose and orchestration platforms
- Supports migration to more sophisticated secret management systems