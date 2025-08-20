# Alchemorsel v3 Infrastructure as Code
# AWS EKS-based production deployment with security and monitoring

terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.23"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.11"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.5"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }

  backend "s3" {
    # Backend configuration will be provided via -backend-config
    # bucket = "alchemorsel-terraform-state"
    # key    = "alchemorsel-v3/terraform.tfstate"
    # region = "us-east-1"
    # encrypt = true
    # dynamodb_table = "alchemorsel-terraform-locks"
  }
}

# Configure AWS Provider
provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "Alchemorsel v3"
      Environment = var.environment
      ManagedBy   = "Terraform"
      Owner       = "DevOps"
    }
  }
}

# Data sources
data "aws_availability_zones" "available" {
  filter {
    name   = "opt-in-status"
    values = ["opt-in-not-required"]
  }
}

data "aws_caller_identity" "current" {}

# VPC Module
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"

  name = "${var.project_name}-${var.environment}-vpc"
  cidr = var.vpc_cidr

  azs             = slice(data.aws_availability_zones.available.names, 0, 3)
  private_subnets = var.private_subnets
  public_subnets  = var.public_subnets

  enable_nat_gateway   = true
  enable_vpn_gateway   = false
  enable_dns_hostnames = true
  enable_dns_support   = true

  # Enable VPC Flow Logs for security
  enable_flow_log                      = true
  create_flow_log_cloudwatch_iam_role  = true
  create_flow_log_cloudwatch_log_group = true
  flow_log_cloudwatch_log_group_retention_in_days = 30

  # Kubernetes-specific tags
  public_subnet_tags = {
    "kubernetes.io/role/elb" = "1"
    "kubernetes.io/cluster/${var.project_name}-${var.environment}-cluster" = "owned"
  }

  private_subnet_tags = {
    "kubernetes.io/role/internal-elb" = "1"
    "kubernetes.io/cluster/${var.project_name}-${var.environment}-cluster" = "owned"
  }

  tags = {
    "kubernetes.io/cluster/${var.project_name}-${var.environment}-cluster" = "shared"
  }
}

# Security Groups
module "security_groups" {
  source = "./modules/security-groups"

  vpc_id      = module.vpc.vpc_id
  environment = var.environment
  vpc_cidr    = var.vpc_cidr
}

# EKS Cluster
module "eks" {
  source = "terraform-aws-modules/eks/aws"
  version = "~> 19.0"

  cluster_name    = "${var.project_name}-${var.environment}-cluster"
  cluster_version = var.kubernetes_version

  vpc_id                         = module.vpc.vpc_id
  subnet_ids                     = module.vpc.private_subnets
  cluster_endpoint_public_access = var.environment == "production" ? false : true
  cluster_endpoint_private_access = true

  # Cluster endpoint access
  cluster_endpoint_public_access_cidrs = var.environment == "production" ? var.allowed_cidr_blocks : ["0.0.0.0/0"]

  # Cluster encryption
  cluster_encryption_config = {
    provider_key_arn = aws_kms_key.eks.arn
    resources        = ["secrets"]
  }

  # Cluster security group
  cluster_security_group_additional_rules = {
    ingress_nodes_443 = {
      description                = "Node groups to cluster API"
      protocol                   = "tcp"
      from_port                  = 443
      to_port                    = 443
      type                       = "ingress"
      source_node_security_group = true
    }
  }

  # Node security group
  node_security_group_additional_rules = {
    ingress_self_all = {
      description = "Node to node all ports/protocols"
      protocol    = "-1"
      from_port   = 0
      to_port     = 65535
      type        = "ingress"
      self        = true
    }
    ingress_cluster_443 = {
      description                   = "Cluster API to node groups"
      protocol                      = "tcp"
      from_port                     = 443
      to_port                       = 443
      type                          = "ingress"
      source_cluster_security_group = true
    }
    ingress_cluster_kubelet = {
      description                   = "Cluster API to node kubelets"
      protocol                      = "tcp"
      from_port                     = 10250
      to_port                       = 10250
      type                          = "ingress"
      source_cluster_security_group = true
    }
    egress_all = {
      description      = "Node all egress"
      protocol         = "-1"
      from_port        = 0
      to_port          = 0
      type             = "egress"
      cidr_blocks      = ["0.0.0.0/0"]
      ipv6_cidr_blocks = ["::/0"]
    }
  }

  # EKS Managed Node Groups
  eks_managed_node_groups = {
    general = {
      name = "general"

      instance_types = var.node_instance_types
      capacity_type  = "ON_DEMAND"

      min_size     = var.node_group_min_size
      max_size     = var.node_group_max_size
      desired_size = var.node_group_desired_size

      # Launch template
      launch_template_name            = ""
      launch_template_use_name_prefix = true
      launch_template_version         = "$Latest"

      # Remote access
      remote_access = {
        ec2_ssh_key               = var.key_pair_name
        source_security_group_ids = [module.security_groups.bastion_sg_id]
      }

      # Node group update configuration
      update_config = {
        max_unavailable_percentage = 33
      }

      labels = {
        role = "general"
      }

      taints = []

      tags = {
        ExtraTag = "EKS managed node group"
      }
    }

    spot = {
      name = "spot"

      instance_types = var.spot_instance_types
      capacity_type  = "SPOT"

      min_size     = 0
      max_size     = var.spot_max_size
      desired_size = var.spot_desired_size

      # Launch template
      launch_template_name            = ""
      launch_template_use_name_prefix = true
      launch_template_version         = "$Latest"

      labels = {
        role = "spot"
      }

      taints = [
        {
          key    = "spot"
          value  = "true"
          effect = "NO_SCHEDULE"
        }
      ]

      tags = {
        ExtraTag = "EKS managed node group - spot instances"
      }
    }
  }

  # EKS Addons
  cluster_addons = {
    coredns = {
      most_recent = true
      resolve_conflicts = "OVERWRITE"
    }
    kube-proxy = {
      most_recent = true
      resolve_conflicts = "OVERWRITE"
    }
    vpc-cni = {
      most_recent = true
      resolve_conflicts = "OVERWRITE"
      configuration_values = jsonencode({
        env = {
          ENABLE_PREFIX_DELEGATION = "true"
          WARM_PREFIX_TARGET       = "1"
        }
      })
    }
    aws-ebs-csi-driver = {
      most_recent = true
      resolve_conflicts = "OVERWRITE"
      service_account_role_arn = module.irsa-ebs-csi.iam_role_arn
    }
  }

  # aws-auth configmap
  manage_aws_auth_configmap = true

  aws_auth_roles = [
    {
      rolearn  = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/AWSReservedSSO_AdministratorAccess_*"
      username = "admin"
      groups   = ["system:masters"]
    },
  ]

  aws_auth_users = var.aws_auth_users

  tags = {
    Environment = var.environment
  }
}

# KMS Key for EKS encryption
resource "aws_kms_key" "eks" {
  description             = "EKS Secret Encryption Key"
  deletion_window_in_days = 7
  enable_key_rotation     = true

  tags = {
    Name = "${var.project_name}-${var.environment}-eks-key"
  }
}

resource "aws_kms_alias" "eks" {
  name          = "alias/${var.project_name}-${var.environment}-eks"
  target_key_id = aws_kms_key.eks.key_id
}

# IRSA for EBS CSI Driver
module "irsa-ebs-csi" {
  source = "terraform-aws-modules/iam/aws//modules/iam-assumable-role-with-oidc"
  version = "~> 5.0"

  create_role                   = true
  role_name                     = "${var.project_name}-${var.environment}-ebs-csi-driver"
  provider_url                  = module.eks.cluster_oidc_issuer_url
  role_policy_arns              = ["arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"]
  oidc_fully_qualified_subjects = ["system:serviceaccount:kube-system:ebs-csi-controller-sa"]
}

# RDS Database
module "rds" {
  source = "./modules/rds"

  environment = var.environment
  vpc_id      = module.vpc.vpc_id
  subnet_ids  = module.vpc.private_subnets
  
  db_instance_class = var.db_instance_class
  db_allocated_storage = var.db_allocated_storage
  db_max_allocated_storage = var.db_max_allocated_storage
  
  security_group_ids = [module.security_groups.rds_sg_id]
  
  backup_retention_period = var.backup_retention_period
  backup_window          = var.backup_window
  maintenance_window     = var.maintenance_window
  
  monitoring_interval = var.monitoring_interval
  performance_insights_enabled = var.performance_insights_enabled
  
  deletion_protection = var.environment == "production" ? true : false
}

# ElastiCache Redis
module "redis" {
  source = "./modules/redis"

  environment = var.environment
  vpc_id      = module.vpc.vpc_id
  subnet_ids  = module.vpc.private_subnets
  
  node_type = var.redis_node_type
  num_cache_nodes = var.redis_num_cache_nodes
  
  security_group_ids = [module.security_groups.redis_sg_id]
  
  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token_enabled = true
}

# Application Load Balancer
module "alb" {
  source = "terraform-aws-modules/alb/aws"
  version = "~> 8.0"

  name = "${var.project_name}-${var.environment}-alb"

  load_balancer_type = "application"

  vpc_id             = module.vpc.vpc_id
  subnets            = module.vpc.public_subnets
  security_groups    = [module.security_groups.alb_sg_id]

  # Access logs
  access_logs = {
    bucket  = aws_s3_bucket.alb_logs.id
    prefix  = "${var.project_name}-${var.environment}-alb"
    enabled = true
  }

  target_groups = [
    {
      name_prefix      = "app-"
      backend_protocol = "HTTP"
      backend_port     = 80
      target_type      = "instance"
      health_check = {
        enabled             = true
        healthy_threshold   = 2
        interval            = 30
        matcher             = "200"
        path                = "/health"
        port                = "traffic-port"
        protocol            = "HTTP"
        timeout             = 5
        unhealthy_threshold = 2
      }
    }
  ]

  https_listeners = [
    {
      port               = 443
      protocol           = "HTTPS"
      certificate_arn    = module.acm.acm_certificate_arn
      target_group_index = 0
    }
  ]

  http_tcp_listeners = [
    {
      port        = 80
      protocol    = "HTTP"
      action_type = "redirect"
      redirect = {
        port        = "443"
        protocol    = "HTTPS"
        status_code = "HTTP_301"
      }
    }
  ]

  tags = {
    Environment = var.environment
  }
}

# S3 Bucket for ALB logs
resource "aws_s3_bucket" "alb_logs" {
  bucket        = "${var.project_name}-${var.environment}-alb-logs-${random_string.bucket_suffix.result}"
  force_destroy = var.environment != "production"
}

resource "aws_s3_bucket_lifecycle_configuration" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id

  rule {
    id     = "log_retention"
    status = "Enabled"

    expiration {
      days = var.alb_log_retention_days
    }

    noncurrent_version_expiration {
      noncurrent_days = 7
    }
  }
}

resource "aws_s3_bucket_public_access_block" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "random_string" "bucket_suffix" {
  length  = 8
  special = false
  upper   = false
}

# ACM Certificate
module "acm" {
  source = "terraform-aws-modules/acm/aws"
  version = "~> 4.0"

  domain_name  = var.domain_name
  zone_id      = var.route53_zone_id

  subject_alternative_names = [
    "*.${var.domain_name}",
  ]

  wait_for_validation = true

  tags = {
    Name = var.domain_name
  }
}

# Route53 Records
resource "aws_route53_record" "app" {
  zone_id = var.route53_zone_id
  name    = var.environment == "production" ? var.domain_name : "${var.environment}.${var.domain_name}"
  type    = "A"

  alias {
    name                   = module.alb.lb_dns_name
    zone_id                = module.alb.lb_zone_id
    evaluate_target_health = true
  }
}

# CloudWatch Log Groups
resource "aws_cloudwatch_log_group" "application" {
  name              = "/aws/eks/${var.project_name}-${var.environment}/application"
  retention_in_days = var.log_retention_days

  tags = {
    Environment = var.environment
  }
}

resource "aws_cloudwatch_log_group" "nginx" {
  name              = "/aws/eks/${var.project_name}-${var.environment}/nginx"
  retention_in_days = var.log_retention_days

  tags = {
    Environment = var.environment
  }
}

# WAF Web ACL
resource "aws_wafv2_web_acl" "app" {
  count = var.enable_waf ? 1 : 0

  name  = "${var.project_name}-${var.environment}-waf"
  scope = "REGIONAL"

  default_action {
    allow {}
  }

  # AWS Managed Rules
  rule {
    name     = "AWS-AWSManagedRulesCommonRuleSet"
    priority = 1

    override_action {
      none {}
    }

    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesCommonRuleSet"
        vendor_name = "AWS"
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                 = "CommonRuleSetMetric"
      sampled_requests_enabled    = true
    }
  }

  # Rate limiting rule
  rule {
    name     = "RateLimitRule"
    priority = 2

    action {
      block {}
    }

    statement {
      rate_based_statement {
        limit              = 2000
        aggregate_key_type = "IP"
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                 = "RateLimitRuleMetric"
      sampled_requests_enabled    = true
    }
  }

  tags = {
    Environment = var.environment
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                 = "${var.project_name}-${var.environment}-waf"
    sampled_requests_enabled    = true
  }
}

# Associate WAF with ALB
resource "aws_wafv2_web_acl_association" "app" {
  count = var.enable_waf ? 1 : 0

  resource_arn = module.alb.lb_arn
  web_acl_arn  = aws_wafv2_web_acl.app[0].arn
}

# Kubernetes configuration
provider "kubernetes" {
  host                   = module.eks.cluster_endpoint
  cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)

  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "aws"
    args        = ["eks", "get-token", "--cluster-name", module.eks.cluster_name]
  }
}

provider "helm" {
  kubernetes {
    host                   = module.eks.cluster_endpoint
    cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)

    exec {
      api_version = "client.authentication.k8s.io/v1beta1"
      command     = "aws"
      args        = ["eks", "get-token", "--cluster-name", module.eks.cluster_name]
    }
  }
}

# Helm Charts
module "monitoring" {
  source = "./modules/monitoring"

  cluster_name = module.eks.cluster_name
  environment  = var.environment
  
  depends_on = [module.eks]
}

module "ingress" {
  source = "./modules/ingress"

  cluster_name = module.eks.cluster_name
  environment  = var.environment
  alb_arn      = module.alb.lb_arn
  
  depends_on = [module.eks]
}