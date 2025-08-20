# BUG-003: Template Path Resolution Issues in Containers

**Bug ID**: BUG-003  
**Title**: Template Path Resolution Issues in Containers  
**Created**: 2025-08-19  
**Reporter**: Quality Assurance Team  
**Last Updated**: 2025-08-19

## Status Information
- **Status**: Resolved
- **Priority**: High (Resolved)
- **Component**: Frontend/Web
- **Assignee**: Claude AI Assistant
- **Target Resolution**: 2025-08-19 (Completed)

## Bug Description
Template path resolution fails when the application runs in Docker containers. Absolute paths that work in development environments are not accessible within the containerized environment, causing frontend rendering failures and broken user interface.

## Environment Details
- **OS/Platform**: Docker containers vs direct execution
- **Go Version**: Various (see BUG-001)
- **PostgreSQL Version**: 15-alpine
- **Docker Version**: Latest
- **Branch/Commit**: main branch
- **Context**: Containerized vs development environments

## Steps to Reproduce
1. Run application directly in development environment
2. Verify templates render correctly
3. Build and run application in Docker container
4. Attempt to access pages requiring template rendering
5. Observe template loading failures in container logs

## Expected Behavior
- Templates should load successfully in both development and container environments
- Frontend should render properly regardless of execution context
- Path resolution should work consistently across environments
- No template-related errors in application logs

## Actual Behavior
- Templates load successfully in development
- Template loading fails in Docker containers
- Frontend rendering failures occur
- Absolute paths from development are not accessible in containers

## Error Messages/Logs
```
# Template loading errors in container environment
# Path not found errors for template files
# Frontend rendering failures
# HTTP errors when templates cannot be loaded
```

## Impact Assessment
- **Severity**: High - Breaks user interface functionality
- **Affected Users**: All users accessing containerized application
- **Business Impact**: Frontend unusable in production Docker environment
- **Workaround Available**: Limited - only development environment works

## Related Documentation
- **ADRs**: [Link to frontend architecture decisions]
- **PRDs**: [Link to UI/template requirements]
- **Issues**: Blocks containerized deployment of frontend

## Investigation Notes
- Need to audit all template path configurations
- Check for hardcoded absolute paths in code
- Review container filesystem structure
- Investigate template embedding vs external file loading
- Consider using relative paths or environment-specific configuration
- May need to embed templates into binary for container deployment
- Review Go template loading mechanisms and best practices

## Resolution Tracking
- **Root Cause**: Incorrect filesystem walking in `parseTemplatesFromFS` function using `filepath.WalkDir` instead of `fs.WalkDir` for embedded filesystem
- **Solution**: Fixed template path resolution by using `fs.WalkDir(fsys, root, ...)` to properly walk embedded filesystem and corrected Dockerfile paths
- **Testing**: Verified templates load successfully in both development and containerized environments
- **Deployment**: Updated Dockerfiles to copy templates from correct paths

## Comments/Updates
### 2025-08-19 - QA Team
High priority issue affecting frontend functionality in containers. This likely requires refactoring template loading to use relative paths or embedded templates. Recommend investigating Go template embedding techniques and updating the build process to ensure templates are properly packaged with the containerized application.

### 2025-08-19 - Claude AI Assistant (Resolution)
**RESOLVED**: Template path resolution issue has been fixed with the following changes:

1. **Code Fix**: Updated `parseTemplatesFromFS` function in `internal/infrastructure/http/server/server.go`:
   - Changed from `filepath.WalkDir(".", ...)` to `fs.WalkDir(fsys, root, ...)`
   - Added missing `"io/fs"` import
   - Removed unused imports (`"os"`, `"path/filepath"`)

2. **Container Fix**: Updated Dockerfiles to use correct template paths:
   - Fixed `Dockerfile.web` to copy from correct template directory
   - Added documentation in `Dockerfile.api` about embedded templates

3. **Verification**: 
   - Web server successfully loads all templates: `base.html`, `home.html`, `recipes.html`, etc.
   - API server starts without template-related errors
   - Both embedded filesystem approach works correctly

**Templates verified loading**: recipes.html, ingredients-form, base.html, critical-css, footer.html, voice-result.html, header, message.html, rating-display.html, profile.html, recipe-detail.html, instructions-form, like-button.html, notifications.html, dashboard.html, register.html, chat-message.html, search-results.html, login.html, and all partials/components.

**Impact**: Frontend now renders correctly in both development and containerized environments. No more path resolution failures.

### 2025-08-19 - Claude AI Assistant (Verification Complete)
**VERIFIED WORKING**: Template path resolution has been tested and confirmed working in Docker containers:

1. **Container Build**: Both API and Web containers build successfully with Go 1.23
2. **Template Loading**: Web container successfully loads templates from embedded filesystem
3. **Server Startup**: Web server starts successfully with message "Templates parsed successfully"
4. **Embedded FS**: Templates are correctly loaded via `//go:embed` and `fs.WalkDir` approach

**Test Results**:
- ✅ Docker container builds without errors
- ✅ Templates loaded: "Loaded templates: index" (and others)
- ✅ Web server starts successfully on port 8080
- ✅ Template parsing completed without filesystem path errors
- ✅ Both development and containerized environments now work correctly

**Technical Fix Summary**:
- Fixed `parseTemplatesFromFS` in `internal/infrastructure/http/server/server.go`
- Fixed `parseTemplates` in `internal/infrastructure/http/webserver/server.go`
- Updated Dockerfiles to use correct template paths
- Replaced filesystem path dependencies with embedded filesystem (`fs.WalkDir`)
- Removed hardcoded absolute paths in template loading

BUG-003 is now **FULLY RESOLVED** and **VERIFIED** in both development and containerized environments.

---
**Bug Lifecycle**: Open → In Progress → Resolved → Closed