// Package healthcheck dependency management
// Provides service dependency tracking and health monitoring
package healthcheck

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// DependencyType represents the type of dependency
type DependencyType string

const (
	DependencyTypeDatabase    DependencyType = "database"
	DependencyTypeCache       DependencyType = "cache"
	DependencyTypeQueue       DependencyType = "queue"
	DependencyTypeExternalAPI DependencyType = "external_api"
	DependencyTypeFileSystem  DependencyType = "filesystem"
	DependencyTypeNetwork     DependencyType = "network"
	DependencyTypeService     DependencyType = "service"
)

// DependencyStatus represents the health status of a dependency
type DependencyStatus struct {
	Name         string                 `json:"name"`
	Type         DependencyType         `json:"type"`
	Status       Status                 `json:"status"`
	Message      string                 `json:"message,omitempty"`
	Critical     bool                   `json:"critical"`
	LastChecked  time.Time              `json:"last_checked"`
	Duration     time.Duration          `json:"duration_ms"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Dependencies []string               `json:"dependencies,omitempty"`
}

// Dependency represents a service dependency
type Dependency interface {
	// GetName returns the name of the dependency
	GetName() string

	// GetType returns the type of the dependency
	GetType() DependencyType

	// IsCritical returns true if this is a critical dependency
	IsCritical() bool

	// GetDependencies returns the names of dependencies this dependency relies on
	GetDependencies() []string

	// Check performs the health check for this dependency
	Check(ctx context.Context) DependencyStatus
}

// DependencyManager manages service dependencies and their health checks
type DependencyManager struct {
	dependencies map[string]Dependency
	graph        *DependencyGraph
	logger       *zap.Logger
	mu           sync.RWMutex
}

// DependencyGraph represents the dependency graph for topological sorting
type DependencyGraph struct {
	nodes map[string]*DependencyNode
	mu    sync.RWMutex
}

// DependencyNode represents a node in the dependency graph
type DependencyNode struct {
	Name         string
	Dependencies []string
	Dependents   []string
}

// NewDependencyManager creates a new dependency manager
func NewDependencyManager(logger *zap.Logger) *DependencyManager {
	return &DependencyManager{
		dependencies: make(map[string]Dependency),
		graph:        NewDependencyGraph(),
		logger:       logger,
	}
}

// NewDependencyGraph creates a new dependency graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string]*DependencyNode),
	}
}

// Register registers a dependency
func (dm *DependencyManager) Register(dep Dependency) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	name := dep.GetName()
	dm.dependencies[name] = dep
	dm.graph.AddNode(name, dep.GetDependencies())

	if dm.logger != nil {
		dm.logger.Info("Registered dependency",
			zap.String("name", name),
			zap.String("type", string(dep.GetType())),
			zap.Bool("critical", dep.IsCritical()),
			zap.Strings("dependencies", dep.GetDependencies()),
		)
	}
}

// Unregister removes a dependency
func (dm *DependencyManager) Unregister(name string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	delete(dm.dependencies, name)
	dm.graph.RemoveNode(name)

	if dm.logger != nil {
		dm.logger.Info("Unregistered dependency", zap.String("name", name))
	}
}

// CheckAll performs health checks on all dependencies in topological order
func (dm *DependencyManager) CheckAll(ctx context.Context) []DependencyStatus {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	// Get topologically sorted order
	order := dm.graph.TopologicalSort()

	results := make([]DependencyStatus, 0, len(order))
	statusMap := make(map[string]DependencyStatus)

	for _, name := range order {
		dep, exists := dm.dependencies[name]
		if !exists {
			continue
		}

		status := dep.Check(ctx)

		// Check if any dependencies are unhealthy
		for _, depName := range dep.GetDependencies() {
			if depStatus, exists := statusMap[depName]; exists {
				if depStatus.Status == StatusUnhealthy && depStatus.Critical {
					status.Status = StatusDegraded
					status.Message = fmt.Sprintf("Dependency '%s' is unhealthy", depName)
					break
				}
			}
		}

		statusMap[name] = status
		results = append(results, status)
	}

	return results
}

// CheckDependency performs a health check on a specific dependency
func (dm *DependencyManager) CheckDependency(ctx context.Context, name string) (*DependencyStatus, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	dep, exists := dm.dependencies[name]
	if !exists {
		return nil, fmt.Errorf("dependency '%s' not found", name)
	}

	status := dep.Check(ctx)
	return &status, nil
}

// GetDependencies returns all registered dependencies
func (dm *DependencyManager) GetDependencies() map[string]Dependency {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	deps := make(map[string]Dependency)
	for name, dep := range dm.dependencies {
		deps[name] = dep
	}
	return deps
}

// GetCriticalDependencies returns only critical dependencies
func (dm *DependencyManager) GetCriticalDependencies() map[string]Dependency {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	deps := make(map[string]Dependency)
	for name, dep := range dm.dependencies {
		if dep.IsCritical() {
			deps[name] = dep
		}
	}
	return deps
}

// ValidateGraph validates the dependency graph for cycles
func (dm *DependencyManager) ValidateGraph() error {
	return dm.graph.ValidateCycles()
}

// AddNode adds a node to the dependency graph
func (dg *DependencyGraph) AddNode(name string, dependencies []string) {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	node := &DependencyNode{
		Name:         name,
		Dependencies: dependencies,
		Dependents:   []string{},
	}

	dg.nodes[name] = node

	// Update dependents
	for _, dep := range dependencies {
		if depNode, exists := dg.nodes[dep]; exists {
			depNode.Dependents = append(depNode.Dependents, name)
		} else {
			// Create placeholder node
			dg.nodes[dep] = &DependencyNode{
				Name:         dep,
				Dependencies: []string{},
				Dependents:   []string{name},
			}
		}
	}
}

// RemoveNode removes a node from the dependency graph
func (dg *DependencyGraph) RemoveNode(name string) {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	node, exists := dg.nodes[name]
	if !exists {
		return
	}

	// Remove from dependents
	for _, dep := range node.Dependencies {
		if depNode, exists := dg.nodes[dep]; exists {
			depNode.Dependents = removeName(depNode.Dependents, name)
		}
	}

	// Remove from dependencies
	for _, dependent := range node.Dependents {
		if depNode, exists := dg.nodes[dependent]; exists {
			depNode.Dependencies = removeName(depNode.Dependencies, name)
		}
	}

	delete(dg.nodes, name)
}

// TopologicalSort returns nodes in topological order
func (dg *DependencyGraph) TopologicalSort() []string {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	visited := make(map[string]bool)
	temp := make(map[string]bool)
	result := []string{}

	var visit func(string) bool
	visit = func(name string) bool {
		if temp[name] {
			return false // Cycle detected
		}
		if visited[name] {
			return true
		}

		temp[name] = true

		if node, exists := dg.nodes[name]; exists {
			for _, dep := range node.Dependencies {
				if !visit(dep) {
					return false
				}
			}
		}

		temp[name] = false
		visited[name] = true
		result = append(result, name)

		return true
	}

	for name := range dg.nodes {
		if !visited[name] {
			if !visit(name) {
				// Cycle detected, return empty result
				return []string{}
			}
		}
	}

	// Reverse the result
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// ValidateCycles checks for circular dependencies
func (dg *DependencyGraph) ValidateCycles() error {
	sorted := dg.TopologicalSort()
	if len(sorted) == 0 && len(dg.nodes) > 0 {
		return fmt.Errorf("circular dependency detected")
	}
	return nil
}

// BasicDependency provides a basic implementation of the Dependency interface
type BasicDependency struct {
	name         string
	depType      DependencyType
	critical     bool
	dependencies []string
	checker      Checker
}

// NewBasicDependency creates a new basic dependency
func NewBasicDependency(name string, depType DependencyType, critical bool, dependencies []string, checker Checker) *BasicDependency {
	return &BasicDependency{
		name:         name,
		depType:      depType,
		critical:     critical,
		dependencies: dependencies,
		checker:      checker,
	}
}

// GetName returns the name of the dependency
func (bd *BasicDependency) GetName() string {
	return bd.name
}

// GetType returns the type of the dependency
func (bd *BasicDependency) GetType() DependencyType {
	return bd.depType
}

// IsCritical returns true if this is a critical dependency
func (bd *BasicDependency) IsCritical() bool {
	return bd.critical
}

// GetDependencies returns the names of dependencies this dependency relies on
func (bd *BasicDependency) GetDependencies() []string {
	return bd.dependencies
}

// Check performs the health check for this dependency
func (bd *BasicDependency) Check(ctx context.Context) DependencyStatus {
	start := time.Now()

	check := bd.checker.Check(ctx)

	// Safe type assertion for metadata
	var metadata map[string]interface{}
	if check.Metadata != nil {
		if m, ok := check.Metadata.(map[string]interface{}); ok {
			metadata = m
		}
	}

	return DependencyStatus{
		Name:         bd.name,
		Type:         bd.depType,
		Status:       check.Status,
		Message:      check.Message,
		Critical:     bd.critical,
		LastChecked:  start,
		Duration:     check.Duration,
		Metadata:     metadata,
		Dependencies: bd.dependencies,
	}
}

// Helper function to remove a name from a slice
func removeName(slice []string, name string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != name {
			result = append(result, item)
		}
	}
	return result
}

// DatabaseDependency creates a database dependency
func DatabaseDependency(name string, critical bool, checker Checker) *BasicDependency {
	return NewBasicDependency(name, DependencyTypeDatabase, critical, []string{}, checker)
}

// CacheDependency creates a cache dependency
func CacheDependency(name string, critical bool, checker Checker) *BasicDependency {
	return NewBasicDependency(name, DependencyTypeCache, critical, []string{}, checker)
}

// ExternalAPIDependency creates an external API dependency
func ExternalAPIDependency(name string, critical bool, dependencies []string, checker Checker) *BasicDependency {
	return NewBasicDependency(name, DependencyTypeExternalAPI, critical, dependencies, checker)
}

// ServiceDependency creates a service dependency
func ServiceDependency(name string, critical bool, dependencies []string, checker Checker) *BasicDependency {
	return NewBasicDependency(name, DependencyTypeService, critical, dependencies, checker)
}
