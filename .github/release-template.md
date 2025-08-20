# 🚀 Alchemorsel v{{VERSION}}

## What's New

{{CHANGELOG}}

## 📦 Installation

### Docker (Recommended)
```bash
# Pull the latest image
docker pull ghcr.io/alchemorsel/v3:v{{VERSION}}

# Run with Docker Compose
curl -o docker-compose.yml https://raw.githubusercontent.com/alchemorsel/v3/v{{VERSION}}/docker-compose.yml
docker-compose up -d
```

### Kubernetes
```bash
# Apply Kubernetes manifests
kubectl apply -k https://github.com/alchemorsel/v3/deployments/kubernetes/overlays/production?ref=v{{VERSION}}
```

### Binary Installation
Download the appropriate binary for your platform from the release assets below:

- **Linux**: `alchemorsel-v{{VERSION}}-linux-amd64.tar.gz`
- **macOS**: `alchemorsel-v{{VERSION}}-darwin-amd64.tar.gz`  
- **Windows**: `alchemorsel-v{{VERSION}}-windows-amd64.zip`

### Build from Source
```bash
git clone https://github.com/alchemorsel/v3.git
cd v3
git checkout v{{VERSION}}
go build -o alchemorsel cmd/api/main.go
```

## 🔧 Configuration

### Environment Variables
```bash
# Database configuration
DATABASE_URL=postgres://user:pass@localhost:5432/alchemorsel
REDIS_URL=redis://localhost:6379

# AI configuration  
OLLAMA_BASE_URL=http://localhost:11434

# Security
JWT_SECRET=your-secret-key
ENCRYPTION_KEY=your-32-byte-key

# Optional: Monitoring
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
PROMETHEUS_METRICS_ENABLED=true
```

### Docker Compose
```yaml
version: '3.8'
services:
  alchemorsel:
    image: ghcr.io/alchemorsel/v3:v{{VERSION}}
    ports:
      - "3010:3010"
    environment:
      - DATABASE_URL=postgres://postgres:password@db:5432/alchemorsel
      - REDIS_URL=redis://redis:6379
      - OLLAMA_BASE_URL=http://ollama:11434
    depends_on:
      - db
      - redis
      - ollama
```

## 🆙 Upgrade Guide

### From v{{PREVIOUS_VERSION}}

{{#if BREAKING_CHANGES}}
⚠️ **Breaking Changes**

{{BREAKING_CHANGES}}

### Migration Steps

1. **Backup your data** before upgrading
2. **Update configuration** according to the breaking changes above
3. **Run database migrations** if applicable
4. **Test thoroughly** in a staging environment first

{{/if}}

### General Upgrade Steps

1. **Stop the current application**
   ```bash
   docker-compose down
   # or
   systemctl stop alchemorsel
   ```

2. **Backup your database**
   ```bash
   pg_dump alchemorsel > backup-$(date +%Y%m%d).sql
   ```

3. **Update to new version**
   ```bash
   # Docker
   docker pull ghcr.io/alchemorsel/v3:v{{VERSION}}
   
   # Binary
   wget https://github.com/alchemorsel/v3/releases/download/v{{VERSION}}/alchemorsel-v{{VERSION}}-linux-amd64.tar.gz
   tar -xzf alchemorsel-v{{VERSION}}-linux-amd64.tar.gz
   ```

4. **Run database migrations**
   ```bash
   ./alchemorsel migrate up
   ```

5. **Start the application**
   ```bash
   docker-compose up -d
   # or
   systemctl start alchemorsel
   ```

6. **Verify the upgrade**
   ```bash
   curl http://localhost:3010/health
   ```

## 🔍 What's Changed Since v{{PREVIOUS_VERSION}}

### 📊 Statistics
- **{{COMMITS_COUNT}}** commits
- **{{FILES_CHANGED}}** files changed
- **{{CONTRIBUTORS_COUNT}}** contributors

### 🏆 Contributors
{{CONTRIBUTORS}}

## 🔗 Links

- **Documentation**: https://docs.alchemorsel.com
- **Docker Images**: https://github.com/alchemorsel/v3/pkgs/container/v3
- **Helm Charts**: https://github.com/alchemorsel/helm-charts
- **API Documentation**: https://api.alchemorsel.com/docs
- **Community**: https://discord.gg/alchemorsel

## 🛡️ Security

This release has been thoroughly tested and scanned for security vulnerabilities:

- ✅ **SAST** (Static Application Security Testing)
- ✅ **DAST** (Dynamic Application Security Testing)  
- ✅ **Dependency Scanning**
- ✅ **Container Security Scanning**
- ✅ **Infrastructure Security Testing**

### Security Advisories
{{#if SECURITY_ADVISORIES}}
{{SECURITY_ADVISORIES}}
{{else}}
No security advisories for this release.
{{/if}}

## 🚨 Support

If you encounter any issues with this release:

1. **Check the documentation**: https://docs.alchemorsel.com/troubleshooting
2. **Search existing issues**: https://github.com/alchemorsel/v3/issues
3. **Create a new issue**: https://github.com/alchemorsel/v3/issues/new/choose
4. **Join our community**: https://discord.gg/alchemorsel

### Getting Help

- 🐛 **Bug Reports**: Use the bug report template
- 💡 **Feature Requests**: Use the feature request template  
- ❓ **Questions**: Use GitHub Discussions or Discord
- 🔒 **Security Issues**: Email security@alchemorsel.com

## 📝 Full Changelog

**Full Changelog**: https://github.com/alchemorsel/v3/compare/v{{PREVIOUS_VERSION}}...v{{VERSION}}

---

## ⚡ Quick Start

Get Alchemorsel running in under 5 minutes:

```bash
# Clone and run with Docker Compose
git clone https://github.com/alchemorsel/v3.git
cd v3
git checkout v{{VERSION}}
cp .env.example .env
docker-compose up -d

# Wait for services to start
sleep 30

# Open in browser
open http://localhost:3011
```

## 🎯 Next Release

Looking ahead to the next release, we're planning:

- Enhanced AI recipe generation
- Mobile app improvements
- Advanced meal planning features
- Performance optimizations

Track progress on our [roadmap](https://github.com/alchemorsel/v3/projects) and join the discussion in [GitHub Discussions](https://github.com/alchemorsel/v3/discussions).

---

*Generated with ❤️ by the Alchemorsel team and our amazing community of contributors.*