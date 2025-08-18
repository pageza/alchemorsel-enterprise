// Package security provides Role-Based Access Control (RBAC) implementation
package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Role represents a user role with permissions
type Role struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	System      bool         `json:"system"` // Cannot be deleted
}

// Permission represents a specific permission
type Permission struct {
	Resource string   `json:"resource"` // e.g., "recipes", "users", "admin"
	Actions  []string `json:"actions"`  // e.g., ["read", "write", "delete"]
}

// RBACService provides role-based access control
type RBACService struct {
	logger      *zap.Logger
	redisClient *redis.Client
	roles       map[string]*Role
}

// NewRBACService creates a new RBAC service
func NewRBACService(logger *zap.Logger, redisClient *redis.Client) *RBACService {
	service := &RBACService{
		logger:      logger,
		redisClient: redisClient,
		roles:       make(map[string]*Role),
	}

	// Initialize default roles
	service.initializeDefaultRoles()
	
	return service
}

// initializeDefaultRoles sets up the default system roles
func (r *RBACService) initializeDefaultRoles() {
	// Guest role - minimal permissions
	r.roles["guest"] = &Role{
		ID:          "guest",
		Name:        "Guest",
		Description: "Unauthenticated user with read-only access to public content",
		System:      true,
		Permissions: []Permission{
			{Resource: "recipes", Actions: []string{"read"}},
			{Resource: "public", Actions: []string{"read"}},
		},
	}

	// User role - standard authenticated user
	r.roles["user"] = &Role{
		ID:          "user",
		Name:        "User",
		Description: "Authenticated user with standard access",
		System:      true,
		Permissions: []Permission{
			{Resource: "recipes", Actions: []string{"read", "create", "update_own", "delete_own"}},
			{Resource: "profile", Actions: []string{"read", "update"}},
			{Resource: "social", Actions: []string{"read", "create", "update_own", "delete_own"}},
			{Resource: "ai", Actions: []string{"use"}},
		},
	}

	// Premium user role - enhanced features
	r.roles["premium"] = &Role{
		ID:          "premium",
		Name:        "Premium User",
		Description: "Premium subscriber with advanced features",
		System:      true,
		Permissions: []Permission{
			{Resource: "recipes", Actions: []string{"read", "create", "update_own", "delete_own", "export", "import"}},
			{Resource: "profile", Actions: []string{"read", "update"}},
			{Resource: "social", Actions: []string{"read", "create", "update_own", "delete_own"}},
			{Resource: "ai", Actions: []string{"use", "advanced"}},
			{Resource: "analytics", Actions: []string{"read"}},
			{Resource: "premium", Actions: []string{"access"}},
		},
	}

	// Moderator role - content moderation
	r.roles["moderator"] = &Role{
		ID:          "moderator",
		Name:        "Moderator",
		Description: "Content moderator with moderation capabilities",
		System:      true,
		Permissions: []Permission{
			{Resource: "recipes", Actions: []string{"read", "create", "update_own", "delete_own", "moderate"}},
			{Resource: "profile", Actions: []string{"read", "update"}},
			{Resource: "social", Actions: []string{"read", "create", "update_own", "delete_own", "moderate"}},
			{Resource: "users", Actions: []string{"read", "moderate"}},
			{Resource: "reports", Actions: []string{"read", "update"}},
		},
	}

	// Admin role - full system access
	r.roles["admin"] = &Role{
		ID:          "admin",
		Name:        "Administrator",
		Description: "Full system administrator",
		System:      true,
		Permissions: []Permission{
			{Resource: "recipes", Actions: []string{"read", "create", "update", "delete", "moderate"}},
			{Resource: "users", Actions: []string{"read", "create", "update", "delete", "moderate"}},
			{Resource: "social", Actions: []string{"read", "create", "update", "delete", "moderate"}},
			{Resource: "analytics", Actions: []string{"read", "export"}},
			{Resource: "system", Actions: []string{"read", "update", "configure"}},
			{Resource: "roles", Actions: []string{"read", "create", "update", "delete"}},
		},
	}

	// Super admin role - system-level access
	r.roles["superadmin"] = &Role{
		ID:          "superadmin",
		Name:        "Super Administrator",
		Description: "System-level administrator with unrestricted access",
		System:      true,
		Permissions: []Permission{
			{Resource: "*", Actions: []string{"*"}}, // Wildcard for all permissions
		},
	}
}

// HasPermission checks if user roles have permission for resource and action
func (r *RBACService) HasPermission(userRoles []string, resource, action string) bool {
	// Check each role
	for _, roleName := range userRoles {
		role, exists := r.roles[roleName]
		if !exists {
			continue
		}

		// Super admin has all permissions
		if roleName == "superadmin" {
			return true
		}

		// Check role permissions
		for _, permission := range role.Permissions {
			// Check for wildcard resource
			if permission.Resource == "*" {
				return true
			}

			// Check specific resource
			if permission.Resource == resource {
				// Check for wildcard action
				for _, allowedAction := range permission.Actions {
					if allowedAction == "*" || allowedAction == action {
						return true
					}
				}
			}
		}
	}

	return false
}

// RequirePermission middleware to check resource and action permissions
func (r *RBACService) RequirePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user roles from context (set by auth middleware)
		rolesInterface, exists := c.Get("user_roles")
		if !exists {
			// Check if user is authenticated
			userID := c.GetString("user_id")
			if userID == "" {
				// Use guest role for unauthenticated users
				rolesInterface = []string{"guest"}
			} else {
				c.JSON(http.StatusForbidden, gin.H{"error": "User roles not found"})
				c.Abort()
				return
			}
		}

		roles, ok := rolesInterface.([]string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid roles format"})
			c.Abort()
			return
		}

		// Check permission
		if !r.HasPermission(roles, resource, action) {
			r.logger.Warn("Permission denied",
				zap.String("user_id", c.GetString("user_id")),
				zap.Strings("roles", roles),
				zap.String("resource", resource),
				zap.String("action", action),
				zap.String("ip", c.ClientIP()),
			)

			c.JSON(http.StatusForbidden, gin.H{
				"error":    "Insufficient permissions",
				"resource": resource,
				"action":   action,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireRole middleware to check specific roles
func (r *RBACService) RequireRole(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rolesInterface, exists := c.Get("user_roles")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "User roles not found"})
			c.Abort()
			return
		}

		userRoles, ok := rolesInterface.([]string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid roles format"})
			c.Abort()
			return
		}

		// Check if user has any of the required roles
		hasRole := false
		for _, userRole := range userRoles {
			for _, requiredRole := range requiredRoles {
				if userRole == requiredRole {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			r.logger.Warn("Role access denied",
				zap.String("user_id", c.GetString("user_id")),
				zap.Strings("user_roles", userRoles),
				zap.Strings("required_roles", requiredRoles),
				zap.String("ip", c.ClientIP()),
			)

			c.JSON(http.StatusForbidden, gin.H{
				"error":          "Insufficient role permissions",
				"required_roles": requiredRoles,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// OwnershipMiddleware checks if user owns the resource
func (r *RBACService) OwnershipMiddleware(resourceParam string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		// Get resource owner from URL parameter or request body
		resourceOwner := c.Param(resourceParam)
		if resourceOwner == "" {
			// Try to get from request body for POST/PUT requests
			var body map[string]interface{}
			if err := c.ShouldBindJSON(&body); err == nil {
				if owner, exists := body[resourceParam]; exists {
					resourceOwner = fmt.Sprintf("%v", owner)
				}
			}
		}

		// Check ownership or admin privileges
		userRoles := c.GetStringSlice("user_roles")
		isAdmin := r.HasPermission(userRoles, "users", "moderate") || 
				  r.HasPermission(userRoles, "*", "*")

		if resourceOwner != userID && !isAdmin {
			r.logger.Warn("Ownership access denied",
				zap.String("user_id", userID),
				zap.String("resource_owner", resourceOwner),
				zap.Strings("user_roles", userRoles),
			)

			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: resource ownership required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CreateRole creates a new custom role
func (r *RBACService) CreateRole(role *Role) error {
	if role.ID == "" {
		return fmt.Errorf("role ID is required")
	}

	// Check if role already exists
	if _, exists := r.roles[role.ID]; exists {
		return fmt.Errorf("role already exists: %s", role.ID)
	}

	// Validate permissions
	if err := r.validatePermissions(role.Permissions); err != nil {
		return fmt.Errorf("invalid permissions: %w", err)
	}

	// Store role
	r.roles[role.ID] = role

	// Persist to Redis
	return r.persistRole(role)
}

// UpdateRole updates an existing role
func (r *RBACService) UpdateRole(roleID string, updates *Role) error {
	role, exists := r.roles[roleID]
	if !exists {
		return fmt.Errorf("role not found: %s", roleID)
	}

	if role.System {
		return fmt.Errorf("cannot update system role: %s", roleID)
	}

	// Validate permissions
	if err := r.validatePermissions(updates.Permissions); err != nil {
		return fmt.Errorf("invalid permissions: %w", err)
	}

	// Update role
	if updates.Name != "" {
		role.Name = updates.Name
	}
	if updates.Description != "" {
		role.Description = updates.Description
	}
	if updates.Permissions != nil {
		role.Permissions = updates.Permissions
	}

	// Persist to Redis
	return r.persistRole(role)
}

// DeleteRole removes a role
func (r *RBACService) DeleteRole(roleID string) error {
	role, exists := r.roles[roleID]
	if !exists {
		return fmt.Errorf("role not found: %s", roleID)
	}

	if role.System {
		return fmt.Errorf("cannot delete system role: %s", roleID)
	}

	// Remove from memory
	delete(r.roles, roleID)

	// Remove from Redis
	ctx := context.Background()
	key := fmt.Sprintf("role:%s", roleID)
	return r.redisClient.Del(ctx, key).Err()
}

// GetRole retrieves a role by ID
func (r *RBACService) GetRole(roleID string) (*Role, error) {
	role, exists := r.roles[roleID]
	if !exists {
		return nil, fmt.Errorf("role not found: %s", roleID)
	}

	return role, nil
}

// ListRoles returns all available roles
func (r *RBACService) ListRoles() []*Role {
	roles := make([]*Role, 0, len(r.roles))
	for _, role := range r.roles {
		roles = append(roles, role)
	}
	return roles
}

// AssignUserRole assigns roles to a user
func (r *RBACService) AssignUserRole(userID string, roles []string) error {
	// Validate all roles exist
	for _, roleID := range roles {
		if _, exists := r.roles[roleID]; !exists {
			return fmt.Errorf("role not found: %s", roleID)
		}
	}

	// Store user roles in Redis
	ctx := context.Background()
	key := fmt.Sprintf("user_roles:%s", userID)
	
	rolesJSON, err := json.Marshal(roles)
	if err != nil {
		return fmt.Errorf("failed to marshal roles: %w", err)
	}

	return r.redisClient.Set(ctx, key, rolesJSON, 0).Err()
}

// GetUserRoles retrieves roles for a user
func (r *RBACService) GetUserRoles(userID string) ([]string, error) {
	ctx := context.Background()
	key := fmt.Sprintf("user_roles:%s", userID)
	
	rolesJSON, err := r.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// User has no specific roles, return default user role
			return []string{"user"}, nil
		}
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}

	var roles []string
	if err := json.Unmarshal([]byte(rolesJSON), &roles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal roles: %w", err)
	}

	return roles, nil
}

// validatePermissions validates permission structure
func (r *RBACService) validatePermissions(permissions []Permission) error {
	validResources := map[string]bool{
		"recipes": true, "users": true, "social": true, "ai": true,
		"analytics": true, "system": true, "roles": true, "premium": true,
		"profile": true, "public": true, "reports": true, "*": true,
	}

	validActions := map[string]bool{
		"read": true, "create": true, "update": true, "delete": true,
		"update_own": true, "delete_own": true, "moderate": true,
		"use": true, "advanced": true, "export": true, "import": true,
		"access": true, "configure": true, "*": true,
	}

	for _, permission := range permissions {
		// Validate resource
		if !validResources[permission.Resource] && !strings.HasPrefix(permission.Resource, "custom:") {
			return fmt.Errorf("invalid resource: %s", permission.Resource)
		}

		// Validate actions
		for _, action := range permission.Actions {
			if !validActions[action] && !strings.HasPrefix(action, "custom:") {
				return fmt.Errorf("invalid action: %s", action)
			}
		}
	}

	return nil
}

// persistRole saves role to Redis
func (r *RBACService) persistRole(role *Role) error {
	ctx := context.Background()
	key := fmt.Sprintf("role:%s", role.ID)
	
	roleJSON, err := json.Marshal(role)
	if err != nil {
		return fmt.Errorf("failed to marshal role: %w", err)
	}

	return r.redisClient.Set(ctx, key, roleJSON, 0).Err()
}

// PermissionCheck is a utility struct for permission checking in templates
type PermissionCheck struct {
	userRoles []string
	rbac      *RBACService
}

// NewPermissionCheck creates a new permission checker
func (r *RBACService) NewPermissionCheck(userRoles []string) *PermissionCheck {
	return &PermissionCheck{
		userRoles: userRoles,
		rbac:      r,
	}
}

// Can checks if user can perform action on resource
func (p *PermissionCheck) Can(resource, action string) bool {
	return p.rbac.HasPermission(p.userRoles, resource, action)
}

// HasRole checks if user has specific role
func (p *PermissionCheck) HasRole(role string) bool {
	for _, userRole := range p.userRoles {
		if userRole == role {
			return true
		}
	}
	return false
}

// IsAdmin checks if user has admin privileges
func (p *PermissionCheck) IsAdmin() bool {
	return p.HasRole("admin") || p.HasRole("superadmin")
}

// IsModerator checks if user has moderator privileges
func (p *PermissionCheck) IsModerator() bool {
	return p.HasRole("moderator") || p.IsAdmin()
}

// IsPremium checks if user has premium access
func (p *PermissionCheck) IsPremium() bool {
	return p.HasRole("premium") || p.IsAdmin()
}