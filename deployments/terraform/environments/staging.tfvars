# Staging environment configuration for Alchemorsel v3

# General Configuration
environment    = "staging"
project_name  = "alchemorsel"
aws_region    = "us-east-1"

# VPC Configuration
vpc_cidr        = "10.1.0.0/16"
private_subnets = ["10.1.1.0/24", "10.1.2.0/24", "10.1.3.0/24"]
public_subnets  = ["10.1.101.0/24", "10.1.102.0/24", "10.1.103.0/24"]

# EKS Configuration
kubernetes_version = "1.28"
node_instance_types = ["t3.small"]
node_group_min_size = 1
node_group_max_size = 3
node_group_desired_size = 2

# Spot instances configuration (for cost optimization in staging)
enable_spot_instances = true
spot_instance_types = ["t3.small", "t3.medium"]
spot_max_size = 2
spot_desired_size = 1

# Network access (more permissive for staging)
allowed_cidr_blocks = ["0.0.0.0/0"]

# RDS Configuration (minimal for staging)
db_instance_class = "db.t3.micro"
db_allocated_storage = 20
db_max_allocated_storage = 50
backup_retention_period = 3
monitoring_interval = 0
performance_insights_enabled = false

# Redis Configuration (minimal for staging)
redis_node_type = "cache.t3.micro"
redis_num_cache_nodes = 1

# Domain Configuration
domain_name = "alchemorsel.com"
# route53_zone_id = "Z1234567890" # Set this to your actual Route53 zone ID

# Security Configuration (relaxed for staging)
enable_waf = false
enable_pod_security_policy = false
enable_network_policy = false

# Logging Configuration (shorter retention for cost)
log_retention_days = 7
alb_log_retention_days = 7

# Monitoring Configuration
enable_monitoring = true
enable_alerting = false  # Disable alerting in staging
enable_performance_monitoring = false

# Cost Optimization
enable_cluster_autoscaler = true

# Feature Flags for staging
feature_flags = {
  enable_blue_green_deployment = false  # Use simpler deployment in staging
  enable_canary_deployment     = false
  enable_chaos_engineering     = false
  enable_load_testing          = true   # Enable for performance testing
}

# Backup Configuration (minimal for staging)
enable_automated_backups = false
backup_schedule = "0 6 * * 0"  # Weekly on Sunday

# Compliance (disabled for staging)
enable_compliance_monitoring = false
enable_config_rules = false

# GPU Configuration (disabled for staging)
enable_gpu_nodes = false

# AI Configuration
ollama_model_storage_size = 50  # Smaller storage for staging

# Additional tags
additional_tags = {
  Environment = "staging"
  Purpose     = "development-testing"
  CostCenter  = "engineering"
  Owner       = "devops-team"
  AutoShutdown = "true"  # Allow automatic shutdown for cost savings
}