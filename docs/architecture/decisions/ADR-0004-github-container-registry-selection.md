# ADR-0004: GitHub Container Registry Selection

## Status
Accepted

## Context
Alchemorsel v3 requires a container registry for storing and distributing Docker images across development, testing, and production environments. The registry must integrate well with our GitHub-based development workflow and provide reliable, secure image distribution.

Evaluation criteria:
- Integration with GitHub Actions CI/CD
- Security and access control features
- Cost effectiveness for private repositories
- Performance and global distribution
- Team familiarity and tooling support

Options considered:
1. GitHub Container Registry (ghcr.io)
2. Docker Hub
3. AWS ECR
4. Google Container Registry

## Decision
We will use GitHub Container Registry (ghcr.io) as the primary container registry for all Alchemorsel v3 images.

**Implementation Requirements:**
- All container images must be pushed to `ghcr.io/username/alchemorsel-v3`
- Image naming convention: `ghcr.io/username/alchemorsel-v3/service:tag`
- Semantic versioning for production images (v1.0.0, v1.1.0, etc.)
- Branch-based tagging for development (main, feature-xyz)
- Automated image building via GitHub Actions
- Private repository access controls aligned with GitHub repository permissions

**Image Tagging Strategy:**
- `latest` - Current production release
- `main` - Latest main branch build
- `v{major}.{minor}.{patch}` - Specific version releases
- `{branch-name}` - Feature branch builds
- `{commit-sha}` - Specific commit builds

## Consequences

### Positive
- Seamless integration with GitHub Actions and repository permissions
- No additional authentication setup required for team members
- Cost-effective for private repositories (included in GitHub plans)
- Automatic cleanup policies available
- Native support in GitHub workflow ecosystem
- Security scanning integrated with GitHub Security features

### Negative
- Vendor lock-in to GitHub ecosystem
- Less flexibility compared to cloud-native registries
- Limited geographic distribution compared to major cloud providers
- Dependency on GitHub service availability

### Neutral
- Performance adequate for our deployment scale
- Migration path available to other registries if needed
- Standard Docker registry API compatibility