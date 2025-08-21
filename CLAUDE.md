# CLAUDE.md - Alchemorsel v3 Project Context

## Global Standards Compliance
**CRITICAL**: Follow `/home/hermes/CLAUDE.md` for mandatory subagent usage and development standards.

**⚠️ REMINDER**: You are getting lax on using subagents. ALL complex tasks must use appropriate subagents.

## Project Overview
Alchemorsel v3 is an AI-first recipe platform with HTMX frontend, focusing on 14KB first packet optimization and enterprise-grade architecture.

## Documentation Framework
**All technical decisions and requirements are documented in `/docs/`**

### Architecture Decisions (NEVER DEVIATE FROM THESE):
- **ADRs Location**: `/docs/architecture/decisions/` - 19 technical standards
- **Critical Standards**: 
  - Go 1.23 ONLY (ADR-0001)
  - PostgreSQL-only, no SQLite (ADR-0002) 
  - ghcr.io container registry (ADR-0004)
  - portscan port allocation (ADR-0005)
  - Docker Compose per environment (ADR-0003)
- **Performance Goals**: 14KB first packet, cache-first patterns (ADR-0006, ADR-0007)

### Product Requirements:
- **PRDs Location**: `/docs/product/requirements/` - Feature specifications  
- **Current Focus**: Docker deployment system (PRD-001)
- **Active PRDs**: PRD-001 (Docker), PRD-002 (Performance), PRD-003 (AI), PRD-004 (DevEx)

### Project Tracking:
- **Bug Index**: `/docs/project/bugs-index.md` (references individual bug files in `/docs/project/bugs/`)
- **Implementation Log**: `/docs/project/implementation-log.md`
- **Testing Results**: `/docs/project/testing-results.md`

## Project-Specific Subagent Requirements

### When to Use Subagents (ALWAYS):
1. **Complex, multi-step tasks** (3+ distinct steps)
2. **Non-trivial implementations** requiring careful planning
3. **User explicitly requests** subagent usage
4. **Multiple tasks provided** in a single request
5. **After receiving new instructions** - immediately capture as todos

### Required Subagent Types:
- **`software-architect`**: Major architectural decisions, system design, feature planning
- **`qa-code-reviewer`**: ALL significant code changes, security reviews, pre-PR validation
- **`cybersecurity-auditor`**: Security assessments, vulnerability analysis, auth systems
- **`code-executor`**: Implementing features based on architectural plans
- **`test-framework-researcher`**: Comprehensive test coverage, testing strategy
- **`network-performance-optimizer`**: Performance optimization, network analysis

### Subagent Usage Rules:
1. **ALWAYS use qa-code-reviewer** for significant code changes before committing
2. **ALWAYS use cybersecurity-auditor** for security-related modifications
3. **ALWAYS use software-architect** for new features or architectural changes
4. **Multiple subagents concurrently** when possible for maximum performance
5. **Reference ADRs and PRDs** in subagent prompts for context

### Examples of Mandatory Subagent Usage:
```markdown
❌ DON'T: Directly implement new authentication system
✅ DO: Use software-architect → code-executor → qa-code-reviewer → cybersecurity-auditor

❌ DON'T: Make database schema changes directly
✅ DO: Use software-architect → test-framework-researcher → qa-code-reviewer

❌ DON'T: Optimize performance without analysis
✅ DO: Use network-performance-optimizer → code-executor → qa-code-reviewer
```

## Development Workflow (STRICT ORDER):

### 1. **Pre-Implementation**:
- Check relevant ADRs before making technical decisions
- Reference PRDs for feature requirements  
- Check bug index for known related issues
- **ALWAYS consult ADRs for consistency standards**

### 2. **Task Planning**:
- Use TodoWrite tool for complex tasks
- Break down work into manageable pieces
- Identify required subagents for each task type

### 3. **Implementation**:
- **Use appropriate subagents** (see requirements above)
- Create new bug files for any issues discovered
- Update implementation log with progress
- **NEVER deviate from ADR standards**

### 4. **Quality Assurance**:
- **MANDATORY**: Use qa-code-reviewer for all significant changes
- **MANDATORY**: Use cybersecurity-auditor for security changes
- Update testing results
- Validate against ADR requirements

### 5. **Documentation**:
- Update related ADRs if architectural changes made
- Update PRDs if requirements change
- Log implementation details
- Update CLAUDE.md if workflow changes

## Bug Tracking Workflow:
1. **Discover Issue**: Create new bug file from `/docs/project/bugs/bug-template.md`
2. **Investigation**: Document findings in individual bug file
3. **Resolution**: Update bug file with solution details
4. **Closure**: Move to resolved section in `/docs/project/bugs-index.md`

## Current Priority Focus:
**Docker Deployment System** (PRD-001) with supporting ADRs 0001-0005

### Critical Blockers:
- BUG-001: Go Module Dependency Conflicts (Go 1.23 standardization)
- BUG-002: PostgreSQL Migration Errors (insufficient arguments)
- BUG-003: Template Path Resolution in Containers

### Implementation Sequence:
1. **Fix critical bugs** (BUG-001, BUG-002, BUG-003)
2. **Docker infrastructure** (ADRs 0003, 0004, 0005, 0017, 0018)
3. **Performance optimization** (ADRs 0006, 0007, 0008, 0009)
4. **AI integration** (ADR-0016 Ollama containerization)

## Quality Standards (NON-NEGOTIABLE):
- **Go 1.23**: All modules, Dockerfiles, CI/CD (ADR-0001)
- **PostgreSQL Only**: No SQLite in testing or development (ADR-0002)
- **Performance**: Target 14KB first packet, cache-first patterns (ADR-0006, ADR-0007)
- **Security**: All inputs sanitized, proper auth boundaries (ADR-0013)
- **Testing**: Real PostgreSQL testing only (ADR-0012)

## Development Diary System (MANDATORY):

### Memory System Requirements:
**ALL sessions must use `/DEVELOPMENT_DIARY.md` as a memory system to prevent break/fix/break loops**

### Diary Contents (REQUIRED):
1. **Application Intent**: Brief summary of platform purpose and core features
2. **Feature References**: Link to relevant ADRs and PRDs for context
3. **Decision Log**: Record key decisions with date, summary, and reasoning
4. **Current State**: Track completed phases and current focus areas
5. **Technical Debt**: Document known issues and architecture risks
6. **Memory Triggers**: Critical reminders for future sessions

### Session Workflow (MANDATORY):
1. **Start Session**: Read diary to understand previous context and decisions
2. **During Session**: Update diary with any significant decisions or discoveries
3. **End Session**: Document current state and next steps for future sessions

### Purpose:
- **Prevent Forgetfulness**: Maintain context across sessions
- **Avoid Reversions**: Remember why decisions were made
- **Break Fix/Break Loops**: Learn from previous attempts
- **Maintain Consistency**: Ensure architectural decisions stick

## Emergency Procedures:
- **Architecture Drift**: Immediate ADR review and correction
- **Security Issues**: STOP development, use cybersecurity-auditor immediately
- **Performance Regression**: Use network-performance-optimizer for analysis
- **Unknown Territory**: Use software-architect for guidance
- **Session Confusion**: READ DEVELOPMENT_DIARY.md immediately

---

**Remember**: These ADRs, workflows, and the development diary exist to maintain consistency and quality. When in doubt, consult the diary, appropriate ADR, and use the recommended subagent for the task type.