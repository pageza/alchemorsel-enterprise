# Alchemorsel v3 Terraform Variables
# Comprehensive variable definitions for all environments

# General Configuration
variable "aws_region" {
  description = "AWS region for resources"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name (staging, production)"
  type        = string
  validation {
    condition     = contains(["staging", "production"], var.environment)
    error_message = "Environment must be either 'staging' or 'production'."
  }
}

variable "project_name" {
  description = "Name of the project"
  type        = string
  default     = "alchemorsel"
}

# VPC Configuration
variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "private_subnets" {
  description = "Private subnet CIDR blocks"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
}

variable "public_subnets" {
  description = "Public subnet CIDR blocks"
  type        = list(string)
  default     = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]
}

# EKS Configuration
variable "kubernetes_version" {
  description = "Kubernetes version for EKS cluster"
  type        = string
  default     = "1.28"
}

variable "node_instance_types" {
  description = "EC2 instance types for EKS node groups"
  type        = list(string)
  default     = ["t3.medium"]
}

variable "node_group_min_size" {
  description = "Minimum number of nodes in EKS node group"
  type        = number
  default     = 1
}

variable "node_group_max_size" {
  description = "Maximum number of nodes in EKS node group"
  type        = number
  default     = 10
}

variable "node_group_desired_size" {
  description = "Desired number of nodes in EKS node group"
  type        = number
  default     = 3
}

variable "spot_instance_types" {
  description = "EC2 instance types for spot instances"
  type        = list(string)
  default     = ["t3.medium", "t3.large", "t3.xlarge"]
}

variable "spot_max_size" {
  description = "Maximum number of spot instances"
  type        = number
  default     = 5
}

variable "spot_desired_size" {
  description = "Desired number of spot instances"
  type        = number
  default     = 0
}

variable "key_pair_name" {
  description = "EC2 Key Pair name for node access"
  type        = string
  default     = ""
}

variable "allowed_cidr_blocks" {
  description = "CIDR blocks allowed to access EKS API server"
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

# AWS Auth Configuration
variable "aws_auth_users" {
  description = "List of users to add to aws-auth configmap"
  type = list(object({
    userarn  = string
    username = string
    groups   = list(string)
  }))
  default = []
}

# RDS Configuration
variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.micro"
}

variable "db_allocated_storage" {
  description = "RDS allocated storage in GB"
  type        = number
  default     = 20
}

variable "db_max_allocated_storage" {
  description = "RDS maximum allocated storage in GB"
  type        = number
  default     = 100
}

variable "backup_retention_period" {
  description = "RDS backup retention period in days"
  type        = number
  default     = 7
}

variable "backup_window" {
  description = "RDS backup window"
  type        = string
  default     = "03:00-04:00"
}

variable "maintenance_window" {
  description = "RDS maintenance window"
  type        = string
  default     = "sun:04:00-sun:05:00"
}

variable "monitoring_interval" {
  description = "RDS enhanced monitoring interval"
  type        = number
  default     = 60
}

variable "performance_insights_enabled" {
  description = "Enable RDS Performance Insights"
  type        = bool
  default     = true
}

# Redis Configuration
variable "redis_node_type" {
  description = "ElastiCache Redis node type"
  type        = string
  default     = "cache.t3.micro"
}

variable "redis_num_cache_nodes" {
  description = "Number of cache nodes"
  type        = number
  default     = 1
}

# Domain and DNS Configuration
variable "domain_name" {
  description = "Domain name for the application"
  type        = string
  default     = "alchemorsel.com"
}

variable "route53_zone_id" {
  description = "Route53 hosted zone ID"
  type        = string
  default     = ""
}

# Security Configuration
variable "enable_waf" {
  description = "Enable AWS WAF"
  type        = bool
  default     = true
}

# Logging Configuration
variable "log_retention_days" {
  description = "CloudWatch log retention period in days"
  type        = number
  default     = 30
}

variable "alb_log_retention_days" {
  description = "ALB access log retention period in days"
  type        = number
  default     = 30
}

# Environment-specific variable maps
locals {
  environment_configs = {
    staging = {
      node_instance_types      = ["t3.small"]
      node_group_min_size     = 1
      node_group_max_size     = 3
      node_group_desired_size = 2
      db_instance_class       = "db.t3.micro"
      db_allocated_storage    = 20
      redis_node_type         = "cache.t3.micro"
      enable_waf              = false
      log_retention_days      = 7
      backup_retention_period = 3
    }
    production = {
      node_instance_types      = ["t3.medium", "t3.large"]
      node_group_min_size     = 3
      node_group_max_size     = 20
      node_group_desired_size = 6
      db_instance_class       = "db.t3.medium"
      db_allocated_storage    = 100
      redis_node_type         = "cache.t3.small"
      enable_waf              = true
      log_retention_days      = 90
      backup_retention_period = 30
    }
  }
}

# Auto-select configuration based on environment
locals {
  config = local.environment_configs[var.environment]
}

# Override variables with environment-specific values
variable "auto_configure" {
  description = "Automatically configure based on environment"
  type        = bool
  default     = true
}

# Monitoring Configuration
variable "enable_monitoring" {
  description = "Enable comprehensive monitoring stack"
  type        = bool
  default     = true
}

variable "enable_alerting" {
  description = "Enable alerting with SNS topics"
  type        = bool
  default     = true
}

variable "slack_webhook_url" {
  description = "Slack webhook URL for alerts"
  type        = string
  default     = ""
  sensitive   = true
}

variable "pagerduty_integration_key" {
  description = "PagerDuty integration key"
  type        = string
  default     = ""
  sensitive   = true
}

# Cost Optimization
variable "enable_spot_instances" {
  description = "Enable spot instances for cost optimization"
  type        = bool
  default     = true
}

variable "enable_cluster_autoscaler" {
  description = "Enable cluster autoscaler"
  type        = bool
  default     = true
}

# Backup Configuration
variable "enable_automated_backups" {
  description = "Enable automated backups"
  type        = bool
  default     = true
}

variable "backup_schedule" {
  description = "Cron expression for backup schedule"
  type        = string
  default     = "0 2 * * *" # Daily at 2 AM
}

# Security Configuration
variable "enable_pod_security_policy" {
  description = "Enable Pod Security Policy"
  type        = bool
  default     = true
}

variable "enable_network_policy" {
  description = "Enable Kubernetes Network Policy"
  type        = bool
  default     = true
}

variable "enable_encryption_at_rest" {
  description = "Enable encryption at rest for all services"
  type        = bool
  default     = true
}

variable "enable_encryption_in_transit" {
  description = "Enable encryption in transit for all services"
  type        = bool
  default     = true
}

# Compliance Configuration
variable "enable_compliance_monitoring" {
  description = "Enable compliance monitoring and auditing"
  type        = bool
  default     = true
}

variable "enable_config_rules" {
  description = "Enable AWS Config rules for compliance"
  type        = bool
  default     = true
}

# Performance Configuration
variable "enable_performance_monitoring" {
  description = "Enable performance monitoring and APM"
  type        = bool
  default     = true
}

variable "enable_distributed_tracing" {
  description = "Enable distributed tracing with X-Ray"
  type        = bool
  default     = true
}

# AI/ML Configuration
variable "enable_gpu_nodes" {
  description = "Enable GPU-enabled nodes for AI workloads"
  type        = bool
  default     = false
}

variable "gpu_instance_types" {
  description = "EC2 instance types for GPU nodes"
  type        = list(string)
  default     = ["g4dn.xlarge"]
}

variable "ollama_model_storage_size" {
  description = "Storage size for Ollama models in GB"
  type        = number
  default     = 100
}

# Feature Flags
variable "feature_flags" {
  description = "Feature flags for enabling/disabling functionality"
  type = object({
    enable_blue_green_deployment = bool
    enable_canary_deployment     = bool
    enable_chaos_engineering     = bool
    enable_load_testing          = bool
  })
  default = {
    enable_blue_green_deployment = true
    enable_canary_deployment     = false
    enable_chaos_engineering     = false
    enable_load_testing          = true
  }
}

# Tags
variable "additional_tags" {
  description = "Additional tags to apply to all resources"
  type        = map(string)
  default     = {}
}