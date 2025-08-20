# Alchemorsel v3 Documentation

## Overview
This directory contains the complete documentation framework for the Alchemorsel v3 project, including architecture decisions, product requirements, and project tracking systems.

## Documentation Structure

```
docs/
├── architecture/
│   └── decisions/           # Architecture Decision Records (ADRs)
├── product/
│   ├── requirements/        # Product Requirements Documents (PRDs)  
│   └── specifications/      # Technical specifications
├── project/
│   ├── bugs/               # Individual bug tracking files
│   ├── bugs-index.md       # Master bug index
│   ├── implementation-log.md # Development progress tracking
│   └── testing-results.md   # Testing activities and results
└── README.md               # This file
```

---

## Architecture Decision Records (ADRs)

**Location**: `/docs/architecture/decisions/`  
**Purpose**: Document all major technical decisions for consistency and future reference.

### Critical Infrastructure ADRs:
- **ADR-0001**: Go 1.23 Standardization *(NEVER use other versions)*
- **ADR-0002**: PostgreSQL-Only Database Strategy *(NO SQLite)*
- **ADR-0003**: Docker Compose Architecture *(Per-environment approach)*
- **ADR-0004**: GitHub Container Registry Selection *(ghcr.io only)*
- **ADR-0005**: PortScan Port Management *(No default ports)*

### Performance & Optimization ADRs:
- **ADR-0006**: Network Optimization Standards *(14KB first packet goal)*
- **ADR-0007**: Redis Caching Strategy *(Cache-first pattern)*
- **ADR-0008**: Database Performance Standards *(PostgreSQL tuning)*
- **ADR-0009**: Core Web Vitals Optimization *(User experience metrics)*

### Development Workflow ADRs:
- **ADR-0010**: Subagent Usage Requirements *(MANDATORY for complex tasks)*
- **ADR-0011**: Environment Variable Management *(Consistent config)*
- **ADR-0012**: Testing Strategy *(PostgreSQL-only testing)*

### Security & Architecture ADRs:
- **ADR-0013**: Security Framework Standards *(Zero-trust model)*
- **ADR-0014**: API Design Consistency Rules *(REST standards)*
- **ADR-0015**: HTMX Frontend Performance Patterns *(Frontend optimization)*

### Advanced Implementation ADRs:
- **ADR-0016**: Ollama Containerization Strategy *(AI service deployment)*
- **ADR-0017**: Docker Secrets Management *(Production security)*
- **ADR-0018**: Hot Reload Development Workflow *(Developer experience)*
- **ADR-0019**: Logging and Monitoring Standards *(Observability)*

### Using ADRs:
1. **ALWAYS** consult relevant ADRs before making technical decisions
2. **NEVER** deviate from ADR standards without creating amendment
3. **Reference** ADR numbers in commit messages and documentation
4. **Update** ADRs when architectural decisions change

---

## Product Requirements Documents (PRDs)

**Location**: `/docs/product/requirements/`  
**Purpose**: Define product features, success metrics, and implementation requirements.

### Active PRDs:
- **PRD-001**: Docker Deployment System *(Zero-downtime containerization)*
- **PRD-002**: Performance Optimization Framework *(14KB first packet, Core Web Vitals)*
- **PRD-003**: AI Integration Platform *(Ollama containerization, recipe generation)*
- **PRD-004**: Developer Experience Improvements *(Hot reload, onboarding)*

### PRD Usage:
1. **Reference** PRDs for feature requirements and success metrics
2. **Validate** implementations against PRD acceptance criteria
3. **Update** PRDs when requirements change
4. **Track** progress against PRD timelines

---

## Bug Tracking System

**Location**: `/docs/project/bugs/`  
**Purpose**: Track issues with individual detailed files and master index.

### Current Critical Bugs:
- **BUG-001**: Go Module Dependency Conflicts *(Blocks Docker)*
- **BUG-002**: PostgreSQL Migration Errors *(Blocks startup)*  
- **BUG-003**: Template Path Resolution *(Blocks containers)*

### Bug Workflow:
1. **Create** new bug from template: `cp bugs/bug-template.md bugs/BUG-XXX-description.md`
2. **Update** bugs-index.md with new bug entry
3. **Track** progress in individual bug file
4. **Reference** related ADRs and PRDs
5. **Move** to resolved section when closed

---

## Project Tracking

### Implementation Log (`/docs/project/implementation-log.md`)
- **Detailed progress tracking** across all development phases
- **Decision documentation** with rationale and context
- **Issue resolution tracking** with root cause analysis
- **Next steps planning** with dependencies and blockers

### Testing Results (`/docs/project/testing-results.md`)  
- **Testing activity tracking** across all testing types
- **Performance metrics** against PRD targets
- **Quality gate validation** results
- **Compliance verification** against ADR requirements

---

## Documentation Standards

### Writing Standards:
- **Clear, concise language** accessible to technical and non-technical readers
- **Structured format** with consistent sections and headers
- **Cross-references** between related documents (ADRs ↔ PRDs ↔ Bugs)
- **Decision rationale** included for all major choices
- **Implementation guidance** with specific examples

### Maintenance Standards:
- **Keep current**: Update docs with code changes
- **Version control**: All docs in Git for change tracking
- **Review process**: Documents reviewed like code changes
- **Link validation**: Ensure all cross-references remain valid
- **Archive obsolete**: Move outdated docs to archive folder

### Quality Standards:
- **Accuracy**: All information current and correct
- **Completeness**: All necessary information included
- **Consistency**: Terminology and format consistent across docs
- **Searchability**: Good file naming and clear headings
- **Actionability**: Clear next steps and implementation guidance

---

## Integration with Development Workflow

### Pre-Development:
1. **Check ADRs** for relevant technical standards
2. **Review PRDs** for feature requirements and acceptance criteria
3. **Scan bug index** for related known issues
4. **Plan implementation** using documented standards

### During Development:
1. **Reference ADRs** for consistent technical decisions
2. **Track progress** in implementation log
3. **Document issues** in bug tracking system
4. **Validate against PRDs** for requirements compliance

### Post-Development:
1. **Update documentation** to reflect changes
2. **Validate against ADRs** for compliance
3. **Test against PRDs** for acceptance criteria
4. **Document lessons learned** in implementation log

---

## Documentation Tools & Automation

### Required Tools:
- **Markdown editor** for document creation and editing
- **Git** for version control and change tracking
- **Link checker** for validation of cross-references
- **Spell checker** for document quality

### Automation Opportunities:
- **ADR compliance checking** in CI/CD pipeline
- **Cross-reference validation** automated testing
- **Documentation generation** from code comments
- **Bug tracking integration** with development tools

---

## Getting Started

### For New Developers:
1. **Read** project CLAUDE.md for context and standards
2. **Review** critical ADRs (0001-0005) for core decisions
3. **Scan** current PRDs for active feature development
4. **Check** bug index for known issues affecting your work

### For Feature Development:
1. **Create/Update** relevant PRD for feature requirements
2. **Reference** ADRs for implementation standards
3. **Track** progress in implementation log
4. **Test** against documented acceptance criteria

### For Bug Resolution:
1. **Create** individual bug file from template
2. **Update** bugs-index.md with bug entry
3. **Reference** related ADRs and PRDs
4. **Document** resolution in bug file

---

## Document Templates

### Available Templates:
- **ADR Template**: Standard architecture decision format
- **PRD Template**: Product requirements document format
- **Bug Template**: Individual bug tracking format

### Creating New Documents:
1. **Copy** relevant template to new file
2. **Follow** naming conventions (e.g., ADR-XXXX, PRD-XXX, BUG-XXX)
3. **Fill** all required sections completely
4. **Update** index files with new document references

---

*This documentation framework is designed to support consistent, high-quality development while maintaining clear architectural decisions and product requirements. All documentation should be treated as living documents that evolve with the project.*