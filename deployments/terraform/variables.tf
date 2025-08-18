# Variables for Alchemorsel v3 Infrastructure

variable "aws_region" {
  description = "AWS region for resources"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name (production, staging, development)"
  type        = string
  default     = "production"
  
  validation {
    condition     = contains(["production", "staging", "development"], var.environment)
    error_message = "Environment must be production, staging, or development."
  }
}

variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "kubernetes_version" {
  description = "Kubernetes version for EKS cluster"
  type        = string
  default     = "1.28"
}

variable "rds_instance_class" {
  description = "RDS instance class for PostgreSQL"
  type        = string
  default     = "db.t3.medium"
  
  validation {
    condition = can(regex("^db\\.(t3|t4g|r5|r6g|m5|m6g)\\.(micro|small|medium|large|xlarge|2xlarge|4xlarge|8xlarge|12xlarge|16xlarge|24xlarge)$", var.rds_instance_class))
    error_message = "RDS instance class must be a valid AWS RDS instance type."
  }
}

variable "redis_node_type" {
  description = "ElastiCache Redis node type"
  type        = string
  default     = "cache.t3.medium"
  
  validation {
    condition = can(regex("^cache\\.(t3|t4g|r5|r6g|m5|m6g)\\.(micro|small|medium|large|xlarge|2xlarge|4xlarge|8xlarge|12xlarge|16xlarge|24xlarge)$", var.redis_node_type))
    error_message = "Redis node type must be a valid AWS ElastiCache node type."
  }
}

variable "domain_name" {
  description = "Primary domain name for the application"
  type        = string
  default     = "alchemorsel.com"
}

variable "ssl_certificate_arn" {
  description = "ARN of the SSL certificate in ACM (must be in us-east-1 for CloudFront)"
  type        = string
  default     = ""
}

# API Keys (sensitive variables)
variable "openai_api_key" {
  description = "OpenAI API key for AI features"
  type        = string
  default     = ""
  sensitive   = true
}

variable "anthropic_api_key" {
  description = "Anthropic API key for AI features"
  type        = string
  default     = ""
  sensitive   = true
}

# Monitoring and alerting
variable "slack_webhook_url" {
  description = "Slack webhook URL for alerts"
  type        = string
  default     = ""
  sensitive   = true
}

variable "pagerduty_service_key" {
  description = "PagerDuty service key for critical alerts"
  type        = string
  default     = ""
  sensitive   = true
}

# Cost optimization
variable "enable_cost_optimization" {
  description = "Enable cost optimization features (spot instances, etc.)"
  type        = bool
  default     = true
}

variable "backup_retention_days" {
  description = "Number of days to retain backups"
  type        = number
  default     = 30
  
  validation {
    condition     = var.backup_retention_days >= 1 && var.backup_retention_days <= 365
    error_message = "Backup retention days must be between 1 and 365."
  }
}

# Security
variable "enable_deletion_protection" {
  description = "Enable deletion protection for critical resources"
  type        = bool
  default     = true
}

variable "allowed_cidr_blocks" {
  description = "CIDR blocks allowed to access the application"
  type        = list(string)
  default     = ["0.0.0.0/0"]  # Should be restricted in production
}

variable "enable_vpc_flow_logs" {
  description = "Enable VPC flow logs for security monitoring"
  type        = bool
  default     = true
}

# Performance
variable "enable_performance_insights" {
  description = "Enable RDS Performance Insights"
  type        = bool
  default     = true
}

variable "cloudfront_price_class" {
  description = "CloudFront price class (PriceClass_100, PriceClass_200, PriceClass_All)"
  type        = string
  default     = "PriceClass_100"
  
  validation {
    condition     = contains(["PriceClass_100", "PriceClass_200", "PriceClass_All"], var.cloudfront_price_class)
    error_message = "CloudFront price class must be PriceClass_100, PriceClass_200, or PriceClass_All."
  }
}

# High availability
variable "multi_az_deployment" {
  description = "Enable multi-AZ deployment for RDS"
  type        = bool
  default     = true
}

variable "enable_cross_region_backup" {
  description = "Enable cross-region backup replication"
  type        = bool
  default     = false
}

# Scaling
variable "min_cluster_size" {
  description = "Minimum number of nodes in EKS cluster"
  type        = number
  default     = 2
  
  validation {
    condition     = var.min_cluster_size >= 1 && var.min_cluster_size <= 100
    error_message = "Minimum cluster size must be between 1 and 100."
  }
}

variable "max_cluster_size" {
  description = "Maximum number of nodes in EKS cluster"
  type        = number
  default     = 20
  
  validation {
    condition     = var.max_cluster_size >= 1 && var.max_cluster_size <= 100
    error_message = "Maximum cluster size must be between 1 and 100."
  }
}

# Feature flags
variable "enable_monitoring_stack" {
  description = "Enable monitoring stack (Prometheus, Grafana, Jaeger)"
  type        = bool
  default     = true
}

variable "enable_service_mesh" {
  description = "Enable service mesh (Istio)"
  type        = bool
  default     = false
}

variable "enable_external_secrets" {
  description = "Enable External Secrets Operator"
  type        = bool
  default     = true
}

# Compliance and governance
variable "enable_compliance_logging" {
  description = "Enable compliance logging and auditing"
  type        = bool
  default     = true
}

variable "data_classification" {
  description = "Data classification level (public, internal, confidential, restricted)"
  type        = string
  default     = "internal"
  
  validation {
    condition     = contains(["public", "internal", "confidential", "restricted"], var.data_classification)
    error_message = "Data classification must be public, internal, confidential, or restricted."
  }
}

# Resource tagging
variable "additional_tags" {
  description = "Additional tags to apply to all resources"
  type        = map(string)
  default     = {}
}

variable "cost_center" {
  description = "Cost center for billing purposes"
  type        = string
  default     = "Engineering"
}

variable "owner" {
  description = "Owner of the infrastructure"
  type        = string
  default     = "DevOps Team"
}

# Maintenance windows
variable "rds_maintenance_window" {
  description = "RDS maintenance window"
  type        = string
  default     = "Mon:04:00-Mon:05:00"
}

variable "elasticache_maintenance_window" {
  description = "ElastiCache maintenance window"
  type        = string
  default     = "sun:05:00-sun:09:00"
}

# Backup windows
variable "rds_backup_window" {
  description = "RDS backup window"
  type        = string
  default     = "03:00-04:00"
}

variable "elasticache_snapshot_window" {
  description = "ElastiCache snapshot window"
  type        = string
  default     = "03:00-05:00"
}