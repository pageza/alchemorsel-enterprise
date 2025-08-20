# Alchemorsel v3 Bug Tracking Index

**Last Updated**: 2025-08-19  
**Total Bugs**: 3  
**Active Bugs**: 3  
**Resolved Bugs**: 0

## Overview
This document serves as the master index for all bugs tracked in the Alchemorsel v3 project. Each bug is documented in detail in individual files and referenced here for quick access and status overview.

## Quick Status Summary
- **Critical Bugs**: 2 (blocking deployment)
- **High Priority Bugs**: 1 (affecting functionality)
- **Medium Priority Bugs**: 0
- **Low Priority Bugs**: 0

## Active Bugs by Priority

### Critical Priority (Immediate Action Required)
These bugs are blocking core functionality or deployment:

| Bug ID | Title | Component | Status | Assignee | Created |
|--------|-------|-----------|--------|----------|---------|
| [BUG-001](./BUG-001-go-module-dependency-conflicts.md) | Go Module Dependency Conflicts | Infrastructure | Open | [TBD] | 2025-08-19 |
| [BUG-002](./BUG-002-postgresql-migration-insufficient-arguments.md) | PostgreSQL Migration "Insufficient Arguments" | Database | Open | [TBD] | 2025-08-19 |

### High Priority (Significant Impact)
These bugs affect important functionality but don't completely block the system:

| Bug ID | Title | Component | Status | Assignee | Created |
|--------|-------|-----------|--------|----------|---------|
| [BUG-003](./BUG-003-template-path-resolution-containers.md) | Template Path Resolution in Containers | Frontend/Web | Open | [TBD] | 2025-08-19 |

### Medium Priority
*No medium priority bugs currently tracked.*

### Low Priority  
*No low priority bugs currently tracked.*

## Bugs by Component

### Infrastructure
- **BUG-001**: Go Module Dependency Conflicts (Critical)

### Database
- **BUG-002**: PostgreSQL Migration "Insufficient Arguments" (Critical)

### Frontend/Web
- **BUG-003**: Template Path Resolution in Containers (High)

### API
*No API bugs currently tracked.*

### Backend
*No backend bugs currently tracked.*

## Current Sprint Impact
The current critical bugs are creating a significant blocker for the Docker implementation milestone:

- **BUG-001** and **BUG-002** are completely blocking Docker containerization
- **BUG-003** affects frontend functionality in containerized environments
- All three bugs need resolution before Docker deployment can proceed

## Bug Creation Workflow

### 1. Create New Bug
1. Copy the [bug template](./bug-template.md)
2. Assign next available bug ID (BUG-004, BUG-005, etc.)
3. Fill out all required sections
4. Save as `BUG-XXX-descriptive-filename.md`
5. Update this index file

### 2. Bug ID Format
- Format: `BUG-XXX` where XXX is a zero-padded 3-digit number
- Start from BUG-001 and increment sequentially
- Use descriptive filenames: `BUG-XXX-short-description.md`

### 3. Priority Guidelines
- **Critical**: Blocks deployment, crashes system, security vulnerabilities
- **High**: Significant functionality impact, user experience problems
- **Medium**: Minor functionality issues, performance concerns
- **Low**: Cosmetic issues, nice-to-have improvements

### 4. Status Lifecycle
```
Open → In Progress → Resolved → Closed
```

### 5. Update Requirements
- Update bug status in individual files immediately when changed
- Update this index when new bugs are created or status changes
- Add comments to individual bug files when progress is made
- Reference related ADRs and PRDs when applicable

## Integration with Project Documentation
This bug tracking system integrates with:
- **ADRs**: Architecture Decision Records in `/docs/project/adrs/`
- **PRDs**: Product Requirements Documents
- **Sprint Planning**: Reference bugs in sprint planning sessions
- **Code Reviews**: Link commits and PRs to bug resolution

## Reporting Guidelines
When reporting new bugs:
1. Use the [bug template](./bug-template.md)
2. Provide detailed reproduction steps
3. Include relevant environment information
4. Assess impact and priority accurately
5. Reference related documentation
6. Update this index file

## Bug Statistics
- **Average Resolution Time**: [To be calculated as bugs are resolved]
- **Most Common Component**: [To be analyzed over time]
- **Critical Bug Trend**: [Monitor critical bug creation vs resolution]

## Related Documentation
- [Bug Template](./bug-template.md)
- [ADR Index](../adrs/adr-index.md)
- [Project Documentation](../README.md)

---
**Maintenance**: This index should be updated whenever bugs are created, status changes, or bugs are resolved.