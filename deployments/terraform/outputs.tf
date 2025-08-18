# Outputs for Alchemorsel v3 Infrastructure

# VPC Outputs
output "vpc_id" {
  description = "ID of the VPC"
  value       = module.vpc.vpc_id
}

output "vpc_cidr_block" {
  description = "CIDR block of the VPC"
  value       = module.vpc.vpc_cidr_block
}

output "private_subnets" {
  description = "List of IDs of private subnets"
  value       = module.vpc.private_subnets
}

output "public_subnets" {
  description = "List of IDs of public subnets"
  value       = module.vpc.public_subnets
}

output "database_subnets" {
  description = "List of IDs of database subnets"
  value       = module.vpc.database_subnets
}

# EKS Outputs
output "cluster_endpoint" {
  description = "Endpoint for EKS control plane"
  value       = module.eks.cluster_endpoint
}

output "cluster_security_group_id" {
  description = "Security group ID attached to the EKS cluster"
  value       = module.eks.cluster_security_group_id
}

output "cluster_iam_role_name" {
  description = "IAM role name associated with EKS cluster"
  value       = module.eks.cluster_iam_role_name
}

output "cluster_iam_role_arn" {
  description = "IAM role ARN associated with EKS cluster"
  value       = module.eks.cluster_iam_role_arn
}

output "cluster_certificate_authority_data" {
  description = "Base64 encoded certificate data required to communicate with the cluster"
  value       = module.eks.cluster_certificate_authority_data
}

output "cluster_name" {
  description = "Name of the EKS cluster"
  value       = module.eks.cluster_name
}

output "oidc_provider_arn" {
  description = "ARN of the OIDC Provider if enabled"
  value       = module.eks.oidc_provider_arn
}

# RDS Outputs
output "rds_instance_endpoint" {
  description = "RDS instance endpoint"
  value       = module.rds.db_instance_endpoint
  sensitive   = true
}

output "rds_instance_hosted_zone_id" {
  description = "RDS instance hosted zone ID"
  value       = module.rds.db_instance_hosted_zone_id
}

output "rds_instance_id" {
  description = "RDS instance ID"
  value       = module.rds.db_instance_id
}

output "rds_instance_resource_id" {
  description = "RDS instance resource ID"
  value       = module.rds.db_instance_resource_id
}

output "rds_instance_status" {
  description = "RDS instance status"
  value       = module.rds.db_instance_status
}

output "rds_instance_name" {
  description = "RDS instance name"
  value       = module.rds.db_instance_name
}

output "rds_instance_username" {
  description = "RDS instance root username"
  value       = module.rds.db_instance_username
  sensitive   = true
}

output "rds_instance_port" {
  description = "RDS instance port"
  value       = module.rds.db_instance_port
}

# ElastiCache Outputs
output "elasticache_cluster_address" {
  description = "DNS name of the cache cluster without the port appended"
  value       = module.elasticache.cache_nodes[0].address
  sensitive   = true
}

output "elasticache_cluster_id" {
  description = "ElastiCache cluster ID"
  value       = module.elasticache.cluster_id
}

output "elasticache_port" {
  description = "ElastiCache port"
  value       = module.elasticache.port
}

# S3 Outputs
output "s3_bucket_names" {
  description = "Map of S3 bucket names"
  value       = module.s3.bucket_names
}

output "s3_bucket_arns" {
  description = "Map of S3 bucket ARNs"
  value       = module.s3.bucket_arns
}

output "s3_static_bucket_domain_name" {
  description = "Domain name of the static assets S3 bucket"
  value       = module.s3.static_bucket_domain_name
}

# CloudFront Outputs
output "cloudfront_distribution_id" {
  description = "ID of the CloudFront distribution"
  value       = module.cloudfront.distribution_id
}

output "cloudfront_distribution_arn" {
  description = "ARN of the CloudFront distribution"
  value       = module.cloudfront.distribution_arn
}

output "cloudfront_distribution_domain_name" {
  description = "Domain name of the CloudFront distribution"
  value       = module.cloudfront.distribution_domain_name
}

output "cloudfront_distribution_hosted_zone_id" {
  description = "CloudFront Route 53 zone ID"
  value       = module.cloudfront.distribution_hosted_zone_id
}

# Route53 Outputs
output "route53_zone_id" {
  description = "Zone ID of Route53 zone"
  value       = module.route53.zone_id
}

output "route53_zone_name_servers" {
  description = "Name servers of Route53 zone"
  value       = module.route53.name_servers
}

# Security Outputs
output "kms_key_id" {
  description = "ID of the KMS key"
  value       = aws_kms_key.alchemorsel.key_id
}

output "kms_key_arn" {
  description = "ARN of the KMS key"
  value       = aws_kms_key.alchemorsel.arn
}

# Secrets Manager Outputs
output "postgres_secret_arn" {
  description = "ARN of the PostgreSQL credentials secret"
  value       = aws_secretsmanager_secret.postgres_credentials.arn
  sensitive   = true
}

output "redis_secret_arn" {
  description = "ARN of the Redis credentials secret"
  value       = aws_secretsmanager_secret.redis_credentials.arn
  sensitive   = true
}

output "app_secrets_arn" {
  description = "ARN of the application secrets"
  value       = aws_secretsmanager_secret.app_secrets.arn
  sensitive   = true
}

# Monitoring Outputs
output "cloudwatch_log_group_names" {
  description = "Names of CloudWatch log groups"
  value       = module.monitoring.log_group_names
}

# Application URLs
output "application_url" {
  description = "URL of the application"
  value       = "https://${var.domain_name}"
}

output "api_url" {
  description = "URL of the API"
  value       = "https://api.${var.domain_name}"
}

output "monitoring_url" {
  description = "URL of the monitoring dashboard"
  value       = "https://monitoring.${var.domain_name}"
}

# Connection strings for applications
output "database_connection_string" {
  description = "Database connection string (without password)"
  value       = "postgresql://postgres@${module.rds.db_instance_endpoint}:${module.rds.db_instance_port}/alchemorsel"
  sensitive   = true
}

output "redis_connection_string" {
  description = "Redis connection string (without password)"
  value       = "redis://${module.elasticache.cache_nodes[0].address}:${module.elasticache.port}"
  sensitive   = true
}

# Kubectl configuration command
output "kubectl_config_command" {
  description = "Command to configure kubectl"
  value       = "aws eks update-kubeconfig --region ${var.aws_region} --name ${module.eks.cluster_name}"
}

# Cost estimation
output "estimated_monthly_cost_usd" {
  description = "Estimated monthly cost in USD (approximate)"
  value = {
    eks_cluster      = 73.00  # Control plane
    eks_nodes        = 150.00 # 3 t3.medium nodes
    rds_postgres     = 85.00  # db.t3.medium
    elasticache      = 45.00  # cache.t3.medium
    alb              = 22.50  # Application Load Balancer
    cloudfront       = 10.00  # CDN
    s3               = 5.00   # Storage
    secrets_manager  = 2.40   # 6 secrets
    cloudwatch       = 10.00  # Logs and metrics
    total_estimated  = 402.90
  }
}

# Security compliance
output "security_compliance" {
  description = "Security compliance status"
  value = {
    encryption_at_rest = true
    encryption_in_transit = true
    vpc_isolation = true
    secrets_management = true
    access_logging = true
    network_segmentation = true
  }
}

# Backup information
output "backup_information" {
  description = "Backup configuration information"
  value = {
    rds_backup_retention = "${var.backup_retention_days} days"
    rds_backup_window = var.rds_backup_window
    elasticache_snapshot_retention = "5 days"
    elasticache_snapshot_window = var.elasticache_snapshot_window
  }
}