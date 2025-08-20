# Production environment configuration for Alchemorsel v3

# General Configuration
environment   = "production"
project_name = "alchemorsel"
aws_region   = "us-east-1"

# VPC Configuration
vpc_cidr        = "10.0.0.0/16"
private_subnets = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
public_subnets  = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]

# EKS Configuration
kubernetes_version = "1.28"
node_instance_types = ["t3.medium", "t3.large"]
node_group_min_size = 3
node_group_max_size = 20
node_group_desired_size = 6

# Spot instances configuration (controlled use in production)
enable_spot_instances = true
spot_instance_types = ["t3.medium", "t3.large", "t3.xlarge"]
spot_max_size = 5
spot_desired_size = 2

# Network access (restricted for production)
allowed_cidr_blocks = [
  "10.0.0.0/8",     # Private networks
  "172.16.0.0/12",  # Private networks
  "192.168.0.0/16", # Private networks
  # Add your office/VPN CIDR blocks here
]

# RDS Configuration (production-grade)
db_instance_class = "db.t3.medium"
db_allocated_storage = 100
db_max_allocated_storage = 1000
backup_retention_period = 30
monitoring_interval = 60
performance_insights_enabled = true

# Redis Configuration (production-grade)
redis_node_type = "cache.t3.small"
redis_num_cache_nodes = 2

# Domain Configuration
domain_name = "alchemorsel.com"
# route53_zone_id = "Z1234567890" # Set this to your actual Route53 zone ID

# Security Configuration (maximum security for production)
enable_waf = true
enable_pod_security_policy = true
enable_network_policy = true
enable_encryption_at_rest = true
enable_encryption_in_transit = true

# Logging Configuration (extended retention for production)
log_retention_days = 90
alb_log_retention_days = 90

# Monitoring Configuration (comprehensive monitoring)
enable_monitoring = true
enable_alerting = true
enable_performance_monitoring = true
enable_distributed_tracing = true

# Cost Optimization
enable_cluster_autoscaler = true

# Feature Flags for production
feature_flags = {
  enable_blue_green_deployment = true   # Enable for zero-downtime deployments
  enable_canary_deployment     = true   # Enable for safe rollouts
  enable_chaos_engineering     = false  # Consider enabling after stabilization
  enable_load_testing          = false  # Disable in production (use staging)
}

# Backup Configuration (comprehensive backups)
enable_automated_backups = true
backup_schedule = "0 2 * * *"  # Daily at 2 AM

# Compliance (full compliance monitoring)
enable_compliance_monitoring = true
enable_config_rules = true

# GPU Configuration (enable if AI workloads require it)
enable_gpu_nodes = false
gpu_instance_types = ["g4dn.xlarge", "g4dn.2xlarge"]

# AI Configuration
ollama_model_storage_size = 200  # Larger storage for production models

# AWS Auth Users (add your team members)
aws_auth_users = [
  # {
  #   userarn  = "arn:aws:iam::ACCOUNT-ID:user/devops-user"
  #   username = "devops-user"
  #   groups   = ["system:masters"]
  # },
  # {
  #   userarn  = "arn:aws:iam::ACCOUNT-ID:user/developer"
  #   username = "developer"
  #   groups   = ["system:authenticated"]
  # }
]

# Additional tags
additional_tags = {
  Environment  = "production"
  Purpose      = "customer-facing"
  CostCenter   = "engineering"
  Owner        = "devops-team"
  Backup       = "required"
  Monitoring   = "required"
  Compliance   = "required"
  AutoShutdown = "false"  # Never auto-shutdown production
  DataClass    = "confidential"
}