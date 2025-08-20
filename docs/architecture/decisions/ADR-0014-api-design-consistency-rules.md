# ADR-0014: API Design Consistency Rules

## Status
Accepted

## Context
Alchemorsel v3 exposes both internal and external APIs that must be consistent, predictable, and maintainable. Inconsistent API design leads to integration difficulties, increased support burden, and poor developer experience. A well-designed API becomes a competitive advantage and simplifies future development.

API consumers:
- Frontend web application (HTMX/JavaScript)
- Mobile applications (future)
- Third-party integrations
- Internal microservices
- Developer tools and scripts

Current challenges:
- Inconsistent response formats across endpoints
- Varied error handling approaches
- Mixed authentication patterns
- Inconsistent pagination and filtering
- Undocumented breaking changes

## Decision
We will implement comprehensive API design standards ensuring consistency, discoverability, and maintainability across all endpoints.

**API Design Standards:**

**REST Principles:**
- Resource-based URLs: `/api/v1/users/{id}` not `/api/v1/getUser`
- HTTP methods aligned with operations (GET, POST, PUT, DELETE, PATCH)
- Stateless operations with no server-side session dependencies
- Cacheable responses with appropriate HTTP headers
- Uniform interface with consistent patterns

**URL Structure:**
```
/api/{version}/{resource}[/{id}][/{sub-resource}][/{sub-id}]

Examples:
GET    /api/v1/users                    # List users
GET    /api/v1/users/123                # Get user 123  
POST   /api/v1/users                    # Create user
PUT    /api/v1/users/123                # Update user 123
DELETE /api/v1/users/123                # Delete user 123
GET    /api/v1/users/123/orders         # Get user's orders
```

**Response Format (JSON):**
```json
{
  "success": true,
  "data": {
    "id": 123,
    "name": "John Doe",
    "email": "john@example.com"
  },
  "meta": {
    "timestamp": "2024-01-01T12:00:00Z",
    "version": "v1"
  }
}
```

**Error Response Format:**
```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid input provided",
    "details": [
      {
        "field": "email",
        "message": "Must be a valid email address"
      }
    ]
  },
  "meta": {
    "timestamp": "2024-01-01T12:00:00Z",
    "version": "v1"
  }
}
```

**Pagination (List Endpoints):**
```json
{
  "success": true,
  "data": [...],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 150,
    "pages": 8,
    "has_next": true,
    "has_previous": false
  }
}
```

**HTTP Status Codes:**
- 200 OK - Successful GET, PUT, PATCH
- 201 Created - Successful POST
- 204 No Content - Successful DELETE
- 400 Bad Request - Invalid request data
- 401 Unauthorized - Authentication required
- 403 Forbidden - Insufficient permissions
- 404 Not Found - Resource not found
- 422 Unprocessable Entity - Validation errors
- 500 Internal Server Error - Server errors

**Authentication:**
- Bearer token authentication: `Authorization: Bearer {token}`
- Consistent token validation across all protected endpoints
- Clear authentication requirements in API documentation
- Token refresh mechanism for long-lived sessions

**Versioning Strategy:**
- URL-based versioning: `/api/v1/`, `/api/v2/`
- Semantic versioning for breaking changes
- Backward compatibility maintained for at least one major version
- Deprecation notices with sunset dates

**Request/Response Headers:**
```
Content-Type: application/json
Accept: application/json
Cache-Control: private, max-age=0
X-RateLimit-Remaining: 99
X-Request-ID: uuid-here
```

## Consequences

### Positive
- Consistent developer experience across all endpoints
- Reduced integration time and support burden
- Clear patterns for adding new endpoints
- Better caching and performance optimization opportunities
- Foundation for API documentation and tooling

### Negative
- Requires refactoring of existing inconsistent endpoints
- Additional development overhead to maintain consistency
- May require breaking changes for legacy API consumers
- More complex error handling implementation

### Neutral
- Industry standard REST API practices
- Compatible with API documentation tools (OpenAPI/Swagger)
- Supports future API evolution and versioning