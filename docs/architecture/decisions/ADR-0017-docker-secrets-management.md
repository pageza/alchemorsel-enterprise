# ADR-0017: Docker Secrets Management

## Status
Accepted

## Context
Alchemorsel v3 requires secure management of sensitive configuration including database credentials, API keys, encryption keys, and service tokens. These secrets must be protected from unauthorized access while remaining accessible to services that need them. Poor secrets management is a leading cause of security breaches.

Secrets requiring management:
- Database passwords and connection strings
- Redis authentication tokens
- OpenAI API keys and other AI service credentials
- JWT signing keys and encryption secrets
- Third-party service API keys
- SSL/TLS certificates and private keys

Security requirements:
- Secrets never stored in container images or code
- Encryption at rest and in transit
- Access controls and audit logging
- Rotation procedures for all secrets
- Development vs production secret isolation

## Decision
We will implement Docker Secrets for production environments with structured .env file management for development, ensuring consistent security practices across all deployment targets.

**Secrets Management Architecture:**

**Development Environment (.env files):**
```bash
# .env (never committed)
DATABASE_PASSWORD=dev-secure-password
REDIS_PASSWORD=dev-redis-password
JWT_SECRET=dev-jwt-secret-key-min-32-chars
OPENAI_API_KEY=sk-dev-key-here

# .env.example (committed template)  
DATABASE_PASSWORD=your-database-password-here
REDIS_PASSWORD=your-redis-password-here
JWT_SECRET=generate-secure-32-char-minimum-secret
OPENAI_API_KEY=your-openai-api-key-here
```

**Production Environment (Docker Secrets):**
```yaml
version: '3.8'

services:
  web:
    image: alchemorsel:latest
    secrets:
      - db_password
      - jwt_secret
      - openai_api_key
    environment:
      - DATABASE_HOST=postgres
      - DATABASE_USER=alchemorsel
      - DATABASE_PASSWORD_FILE=/run/secrets/db_password
      - JWT_SECRET_FILE=/run/secrets/jwt_secret
      - OPENAI_API_KEY_FILE=/run/secrets/openai_api_key

  postgres:
    image: postgres:15
    secrets:
      - db_password
    environment:
      - POSTGRES_PASSWORD_FILE=/run/secrets/db_password
      - POSTGRES_USER=alchemorsel
      - POSTGRES_DB=alchemorsel

secrets:
  db_password:
    external: true
    name: alchemorsel_db_password_v1
  jwt_secret:
    external: true
    name: alchemorsel_jwt_secret_v1
  openai_api_key:
    external: true
    name: alchemorsel_openai_key_v1
```

**Secret Creation and Rotation:**
```bash
#!/bin/bash
# scripts/manage-secrets.sh

# Create secrets
echo "secure-database-password" | docker secret create alchemorsel_db_password_v1 -
echo "jwt-secret-key-32-chars-minimum" | docker secret create alchemorsel_jwt_secret_v1 -
echo "sk-actual-openai-api-key" | docker secret create alchemorsel_openai_key_v1 -

# Rotate secrets (example for database password)
echo "new-secure-database-password" | docker secret create alchemorsel_db_password_v2 -
# Update docker-compose.yml to reference v2
# Remove old secret after verification
docker secret rm alchemorsel_db_password_v1
```

**Application Secret Loading:**
```go
// pkg/config/secrets.go
package config

import (
    "io/ioutil"
    "os"
    "strings"
)

type Secrets struct {
    DatabasePassword string
    JWTSecret       string
    OpenAIAPIKey    string
}

func LoadSecrets() (*Secrets, error) {
    secrets := &Secrets{}
    
    // Load from Docker secrets in production
    if secretFile := os.Getenv("DATABASE_PASSWORD_FILE"); secretFile != "" {
        password, err := readSecretFile(secretFile)
        if err != nil {
            return nil, err
        }
        secrets.DatabasePassword = password
    } else {
        // Fallback to environment variable (development)
        secrets.DatabasePassword = os.Getenv("DATABASE_PASSWORD")
    }
    
    // Similar pattern for other secrets...
    
    return secrets, nil
}

func readSecretFile(filename string) (string, error) {
    data, err := ioutil.ReadFile(filename)
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(data)), nil
}
```

**Secret Validation and Security:**
```go
// pkg/config/validation.go
func ValidateSecrets(secrets *Secrets) error {
    validations := []struct {
        name  string
        value string
        minLen int
    }{
        {"JWT_SECRET", secrets.JWTSecret, 32},
        {"DATABASE_PASSWORD", secrets.DatabasePassword, 12},
        {"OPENAI_API_KEY", secrets.OpenAIAPIKey, 10},
    }
    
    for _, v := range validations {
        if len(v.value) < v.minLen {
            return fmt.Errorf("%s must be at least %d characters", v.name, v.minLen)
        }
    }
    
    return nil
}
```

**Secret Rotation Procedures:**
1. Generate new secret value
2. Create new Docker secret with versioned name
3. Update docker-compose.yml to reference new secret
4. Deploy updated configuration
5. Verify all services using new secret
6. Remove old secret after confirmation

**Access Controls:**
- Docker secrets only accessible to services that explicitly declare them
- File system permissions restrict secret file access (600)
- Audit logging for secret creation, rotation, and deletion
- Regular secret rotation schedule (quarterly minimum)

## Consequences

### Positive
- Secrets never exposed in container images or logs
- Encrypted storage and transmission of all sensitive data
- Granular access control per service and secret
- Versioned secret management supporting rotation
- Consistent pattern across development and production

### Negative
- Additional operational complexity for secret management
- Requires Docker Swarm mode for external secrets
- Secret rotation requires coordinated deployment updates
- Development environment setup more complex

### Neutral
- Industry standard approach for containerized secret management
- Compatible with external secret management systems (Vault, AWS Secrets Manager)
- Supports migration to more sophisticated secret management platforms