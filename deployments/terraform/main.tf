# Alchemorsel v3 Infrastructure as Code
# Production-ready Terraform configuration for AWS deployment

terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }

  backend "s3" {
    bucket         = "alchemorsel-terraform-state"
    key            = "production/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "alchemorsel-terraform-locks"
  }
}

# Configure AWS Provider
provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "Alchemorsel"
      Environment = var.environment
      ManagedBy   = "Terraform"
      Owner       = "DevOps Team"
      CostCenter  = "Engineering"
    }
  }
}

# Data sources
data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_caller_identity" "current" {}

# Local values for common configurations
locals {
  name_prefix = "alchemorsel-${var.environment}"
  
  common_tags = {
    Project     = "Alchemorsel"
    Environment = var.environment
    ManagedBy   = "Terraform"
  }

  vpc_cidr = var.vpc_cidr
  azs      = slice(data.aws_availability_zones.available.names, 0, 3)
}

# Random password generation for databases
resource "random_password" "postgres_password" {
  length  = 32
  special = true
}

resource "random_password" "redis_password" {
  length  = 32
  special = false
}

# KMS Key for encryption
resource "aws_kms_key" "alchemorsel" {
  description             = "KMS key for Alchemorsel encryption"
  deletion_window_in_days = 7
  enable_key_rotation     = true

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-kms-key"
  })
}

resource "aws_kms_alias" "alchemorsel" {
  name          = "alias/${local.name_prefix}-key"
  target_key_id = aws_kms_key.alchemorsel.key_id
}

# VPC Module
module "vpc" {
  source = "./modules/vpc"

  name_prefix = local.name_prefix
  vpc_cidr    = local.vpc_cidr
  azs         = local.azs
  
  # Public subnets for ALB, NAT Gateway
  public_subnets = [
    cidrsubnet(local.vpc_cidr, 8, 1),
    cidrsubnet(local.vpc_cidr, 8, 2),
    cidrsubnet(local.vpc_cidr, 8, 3),
  ]
  
  # Private subnets for EKS worker nodes
  private_subnets = [
    cidrsubnet(local.vpc_cidr, 8, 11),
    cidrsubnet(local.vpc_cidr, 8, 12),
    cidrsubnet(local.vpc_cidr, 8, 13),
  ]
  
  # Database subnets
  database_subnets = [
    cidrsubnet(local.vpc_cidr, 8, 21),
    cidrsubnet(local.vpc_cidr, 8, 22),
    cidrsubnet(local.vpc_cidr, 8, 23),
  ]

  enable_nat_gateway   = true
  enable_vpn_gateway   = false
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = local.common_tags
}

# EKS Cluster Module
module "eks" {
  source = "./modules/eks"

  cluster_name    = "${local.name_prefix}-cluster"
  cluster_version = var.kubernetes_version
  
  vpc_id          = module.vpc.vpc_id
  subnet_ids      = module.vpc.private_subnets
  
  # Node groups configuration
  node_groups = {
    application = {
      desired_capacity = 3
      max_capacity     = 10
      min_capacity     = 2
      instance_types   = ["t3.medium", "t3.large"]
      disk_size        = 50
      
      k8s_labels = {
        node-type = "application"
      }
      
      taints = []
    }
    
    monitoring = {
      desired_capacity = 1
      max_capacity     = 3
      min_capacity     = 1
      instance_types   = ["t3.large"]
      disk_size        = 100
      
      k8s_labels = {
        node-type = "monitoring"
      }
      
      taints = [
        {
          key    = "monitoring"
          value  = "true"
          effect = "NoSchedule"
        }
      ]
    }
  }

  # Add-ons
  enable_cluster_autoscaler = true
  enable_aws_load_balancer_controller = true
  enable_external_dns = true
  enable_cert_manager = true

  tags = local.common_tags
}

# RDS PostgreSQL Module
module "rds" {
  source = "./modules/rds"

  identifier     = "${local.name_prefix}-postgres"
  engine         = "postgres"
  engine_version = "15.4"
  instance_class = var.rds_instance_class
  
  allocated_storage     = 100
  max_allocated_storage = 1000
  storage_encrypted     = true
  kms_key_id           = aws_kms_key.alchemorsel.arn
  
  db_name  = "alchemorsel"
  username = "postgres"
  password = random_password.postgres_password.result
  
  vpc_security_group_ids = [module.security_groups.rds_security_group_id]
  db_subnet_group_name   = module.vpc.database_subnet_group_name
  
  backup_retention_period = 7
  backup_window          = "03:00-04:00"
  maintenance_window     = "Mon:04:00-Mon:05:00"
  
  enabled_cloudwatch_logs_exports = ["postgresql"]
  monitoring_interval             = 60
  
  performance_insights_enabled = true
  performance_insights_kms_key_id = aws_kms_key.alchemorsel.arn
  
  tags = local.common_tags
}

# ElastiCache Redis Module
module "elasticache" {
  source = "./modules/elasticache"

  cluster_id         = "${local.name_prefix}-redis"
  node_type          = var.redis_node_type
  num_cache_nodes    = 1
  engine_version     = "7.0"
  port               = 6379
  
  subnet_group_name  = module.vpc.elasticache_subnet_group_name
  security_group_ids = [module.security_groups.redis_security_group_id]
  
  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token                 = random_password.redis_password.result
  
  maintenance_window = "sun:05:00-sun:09:00"
  snapshot_window    = "03:00-05:00"
  snapshot_retention_limit = 5
  
  tags = local.common_tags
}

# Security Groups Module
module "security_groups" {
  source = "./modules/security-groups"

  name_prefix = local.name_prefix
  vpc_id      = module.vpc.vpc_id
  
  tags = local.common_tags
}

# S3 Buckets for application storage
module "s3" {
  source = "./modules/s3"

  bucket_prefix = local.name_prefix
  kms_key_id    = aws_kms_key.alchemorsel.arn
  
  # Application buckets
  buckets = {
    uploads = {
      versioning_enabled = true
      lifecycle_enabled  = true
      public_access      = false
    }
    
    backups = {
      versioning_enabled = true
      lifecycle_enabled  = true
      public_access      = false
    }
    
    static = {
      versioning_enabled = false
      lifecycle_enabled  = true
      public_access      = true  # For CDN
    }
  }
  
  tags = local.common_tags
}

# CloudFront CDN
module "cloudfront" {
  source = "./modules/cloudfront"

  name_prefix = local.name_prefix
  
  # Origins
  alb_domain_name = module.eks.alb_dns_name
  s3_bucket_domain = module.s3.static_bucket_domain_name
  
  # SSL certificate ARN (should be created separately)
  acm_certificate_arn = var.ssl_certificate_arn
  
  tags = local.common_tags
}

# Route53 DNS
module "route53" {
  source = "./modules/route53"

  domain_name = var.domain_name
  
  # CloudFront distribution
  cloudfront_domain_name = module.cloudfront.distribution_domain_name
  cloudfront_zone_id     = module.cloudfront.distribution_hosted_zone_id
  
  tags = local.common_tags
}

# Monitoring and Logging
module "monitoring" {
  source = "./modules/monitoring"

  name_prefix = local.name_prefix
  
  # EKS cluster for monitoring setup
  cluster_name = module.eks.cluster_name
  
  # Log retention
  log_retention_in_days = 30
  
  tags = local.common_tags
}

# Secrets Manager for sensitive data
resource "aws_secretsmanager_secret" "postgres_credentials" {
  name        = "${local.name_prefix}-postgres-credentials"
  description = "PostgreSQL database credentials"
  kms_key_id  = aws_kms_key.alchemorsel.arn

  tags = local.common_tags
}

resource "aws_secretsmanager_secret_version" "postgres_credentials" {
  secret_id = aws_secretsmanager_secret.postgres_credentials.id
  secret_string = jsonencode({
    username = "postgres"
    password = random_password.postgres_password.result
    host     = module.rds.db_instance_endpoint
    port     = 5432
    dbname   = "alchemorsel"
  })
}

resource "aws_secretsmanager_secret" "redis_credentials" {
  name        = "${local.name_prefix}-redis-credentials"
  description = "Redis cache credentials"
  kms_key_id  = aws_kms_key.alchemorsel.arn

  tags = local.common_tags
}

resource "aws_secretsmanager_secret_version" "redis_credentials" {
  secret_id = aws_secretsmanager_secret.redis_credentials.id
  secret_string = jsonencode({
    host     = module.elasticache.cache_nodes[0].address
    port     = 6379
    password = random_password.redis_password.result
  })
}

# Application secrets (API keys, etc.)
resource "aws_secretsmanager_secret" "app_secrets" {
  name        = "${local.name_prefix}-app-secrets"
  description = "Application secrets (API keys, JWT secrets, etc.)"
  kms_key_id  = aws_kms_key.alchemorsel.arn

  tags = local.common_tags
}

resource "aws_secretsmanager_secret_version" "app_secrets" {
  secret_id = aws_secretsmanager_secret.app_secrets.id
  secret_string = jsonencode({
    jwt_secret     = random_password.jwt_secret.result
    session_secret = random_password.session_secret.result
    openai_api_key = var.openai_api_key
    anthropic_api_key = var.anthropic_api_key
  })
}

resource "random_password" "jwt_secret" {
  length  = 64
  special = true
}

resource "random_password" "session_secret" {
  length  = 32
  special = true
}