# Kubernetes Security Policies for Alchemorsel v3
# OPA/Conftest policies for enforcing security best practices

package kubernetes.security

import rego.v1

# Deny containers running as root
deny contains msg if {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    container.securityContext.runAsUser == 0
    msg := sprintf("Container '%s' should not run as root (UID 0)", [container.name])
}

# Require non-root filesystem
deny contains msg if {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    container.securityContext.readOnlyRootFilesystem != true
    msg := sprintf("Container '%s' should use read-only root filesystem", [container.name])
}

# Require security context
deny contains msg if {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.securityContext
    msg := sprintf("Container '%s' must have securityContext defined", [container.name])
}

# Require resource limits
deny contains msg if {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.resources.limits
    msg := sprintf("Container '%s' must have resource limits defined", [container.name])
}

# Require memory limits
deny contains msg if {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.resources.limits.memory
    msg := sprintf("Container '%s' must have memory limits defined", [container.name])
}

# Require CPU limits
deny contains msg if {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.resources.limits.cpu
    msg := sprintf("Container '%s' must have CPU limits defined", [container.name])
}

# Deny privileged containers
deny contains msg if {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    container.securityContext.privileged == true
    msg := sprintf("Container '%s' should not run in privileged mode", [container.name])
}

# Deny allowPrivilegeEscalation
deny contains msg if {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    container.securityContext.allowPrivilegeEscalation == true
    msg := sprintf("Container '%s' should not allow privilege escalation", [container.name])
}

# Require capabilities to be dropped
deny contains msg if {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.securityContext.capabilities.drop
    msg := sprintf("Container '%s' must drop capabilities", [container.name])
}

# Require ALL capabilities to be dropped
deny contains msg if {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    capabilities := container.securityContext.capabilities.drop
    not "ALL" in capabilities
    msg := sprintf("Container '%s' must drop ALL capabilities", [container.name])
}

# Require livenessProbe
deny contains msg if {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.livenessProbe
    msg := sprintf("Container '%s' must have livenessProbe defined", [container.name])
}

# Require readinessProbe
deny contains msg if {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.readinessProbe
    msg := sprintf("Container '%s' must have readinessProbe defined", [container.name])
}

# Deny latest image tags in production
deny contains msg if {
    input.kind == "Deployment"
    input.metadata.namespace == "alchemorsel"  # production namespace
    container := input.spec.template.spec.containers[_]
    endswith(container.image, ":latest")
    msg := sprintf("Container '%s' should not use 'latest' tag in production", [container.name])
}

# Require image pull policy
deny contains msg if {
    input.kind == "Deployment"
    container := input.spec.template.spec.containers[_]
    not container.imagePullPolicy
    msg := sprintf("Container '%s' must have imagePullPolicy defined", [container.name])
}

# Require pod security context
deny contains msg if {
    input.kind == "Deployment"
    not input.spec.template.spec.securityContext
    msg := "Pod must have securityContext defined"
}

# Require runAsNonRoot in pod security context
deny contains msg if {
    input.kind == "Deployment"
    input.spec.template.spec.securityContext.runAsNonRoot != true
    msg := "Pod must run as non-root user"
}

# Require fsGroup in pod security context
deny contains msg if {
    input.kind == "Deployment"
    not input.spec.template.spec.securityContext.fsGroup
    msg := "Pod must have fsGroup defined"
}

# Network policy requirements
deny contains msg if {
    input.kind == "NetworkPolicy"
    not input.spec.policyTypes
    msg := "NetworkPolicy must define policyTypes"
}

# Service account requirements
deny contains msg if {
    input.kind == "Deployment"
    not input.spec.template.spec.serviceAccountName
    msg := "Pod must specify serviceAccountName"
}

# PodDisruptionBudget requirements for production
warn contains msg if {
    input.kind == "Deployment"
    input.metadata.namespace == "alchemorsel"
    input.spec.replicas > 1
    msg := "Deployment with multiple replicas should have a PodDisruptionBudget"
}

# Ingress security requirements
deny contains msg if {
    input.kind == "Ingress"
    not input.spec.tls
    msg := "Ingress must use TLS/SSL"
}

# ConfigMap and Secret security
warn contains msg if {
    input.kind == "ConfigMap"
    key := input.data[_]
    contains(lower(key), "password")
    msg := "ConfigMap should not contain passwords - use Secrets instead"
}

warn contains msg if {
    input.kind == "ConfigMap"
    key := input.data[_]
    contains(lower(key), "secret")
    msg := "ConfigMap should not contain secrets - use Secrets instead"
}

# Helper functions
lower(s) := l if {
    l := strings.lower(s)
}