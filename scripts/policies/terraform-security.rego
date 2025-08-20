# Terraform Security Policies for Alchemorsel v3
# OPA/Conftest policies for Infrastructure as Code security

package terraform.security

import rego.v1

# AWS S3 bucket security
deny contains msg if {
    input.resource.aws_s3_bucket[name]
    bucket := input.resource.aws_s3_bucket[name]
    not bucket.server_side_encryption_configuration
    msg := sprintf("S3 bucket '%s' must have server-side encryption enabled", [name])
}

deny contains msg if {
    input.resource.aws_s3_bucket_public_access_block[name]
    pab := input.resource.aws_s3_bucket_public_access_block[name]
    pab.block_public_acls != true
    msg := sprintf("S3 bucket '%s' must block public ACLs", [name])
}

deny contains msg if {
    input.resource.aws_s3_bucket_public_access_block[name]
    pab := input.resource.aws_s3_bucket_public_access_block[name]
    pab.block_public_policy != true
    msg := sprintf("S3 bucket '%s' must block public policies", [name])
}

# AWS RDS security
deny contains msg if {
    input.resource.aws_db_instance[name]
    db := input.resource.aws_db_instance[name]
    db.storage_encrypted != true
    msg := sprintf("RDS instance '%s' must have storage encryption enabled", [name])
}

deny contains msg if {
    input.resource.aws_db_instance[name]
    db := input.resource.aws_db_instance[name]
    db.publicly_accessible == true
    msg := sprintf("RDS instance '%s' should not be publicly accessible", [name])
}

deny contains msg if {
    input.resource.aws_db_instance[name]
    db := input.resource.aws_db_instance[name]
    not db.backup_retention_period
    msg := sprintf("RDS instance '%s' must have backup retention period configured", [name])
}

deny contains msg if {
    input.resource.aws_db_instance[name]
    db := input.resource.aws_db_instance[name]
    db.backup_retention_period < 7
    msg := sprintf("RDS instance '%s' must have backup retention period of at least 7 days", [name])
}

# AWS EKS security
deny contains msg if {
    input.resource.aws_eks_cluster[name]
    cluster := input.resource.aws_eks_cluster[name]
    not cluster.encryption_config
    msg := sprintf("EKS cluster '%s' must have encryption configuration", [name])
}

deny contains msg if {
    input.resource.aws_eks_cluster[name]
    cluster := input.resource.aws_eks_cluster[name]
    cluster.endpoint_config[0].public_access == true
    cluster.endpoint_config[0].public_access_cidrs[_] == "0.0.0.0/0"
    msg := sprintf("EKS cluster '%s' should not allow public access from all IPs", [name])
}

# AWS VPC security
deny contains msg if {
    input.resource.aws_vpc[name]
    vpc := input.resource.aws_vpc[name]
    vpc.enable_dns_support != true
    msg := sprintf("VPC '%s' must have DNS support enabled", [name])
}

deny contains msg if {
    input.resource.aws_vpc[name]
    vpc := input.resource.aws_vpc[name]
    vpc.enable_dns_hostnames != true
    msg := sprintf("VPC '%s' must have DNS hostnames enabled", [name])
}

# AWS Security Group rules
deny contains msg if {
    input.resource.aws_security_group[name]
    sg := input.resource.aws_security_group[name]
    rule := sg.ingress[_]
    rule.from_port == 22
    rule.cidr_blocks[_] == "0.0.0.0/0"
    msg := sprintf("Security group '%s' should not allow SSH access from all IPs", [name])
}

deny contains msg if {
    input.resource.aws_security_group[name]
    sg := input.resource.aws_security_group[name]
    rule := sg.ingress[_]
    rule.from_port == 3389
    rule.cidr_blocks[_] == "0.0.0.0/0"
    msg := sprintf("Security group '%s' should not allow RDP access from all IPs", [name])
}

deny contains msg if {
    input.resource.aws_security_group[name]
    sg := input.resource.aws_security_group[name]
    rule := sg.ingress[_]
    rule.from_port == 0
    rule.to_port == 65535
    rule.cidr_blocks[_] == "0.0.0.0/0"
    msg := sprintf("Security group '%s' should not allow all traffic from all IPs", [name])
}

# AWS ElastiCache security
deny contains msg if {
    input.resource.aws_elasticache_subnet_group[name]
    not input.resource.aws_elasticache_replication_group[name].at_rest_encryption_enabled
    msg := sprintf("ElastiCache '%s' must have encryption at rest enabled", [name])
}

deny contains msg if {
    input.resource.aws_elasticache_replication_group[name]
    cache := input.resource.aws_elasticache_replication_group[name]
    cache.transit_encryption_enabled != true
    msg := sprintf("ElastiCache '%s' must have encryption in transit enabled", [name])
}

# AWS IAM security
deny contains msg if {
    input.resource.aws_iam_policy[name]
    policy := input.resource.aws_iam_policy[name]
    statement := json.unmarshal(policy.policy).Statement[_]
    statement.Effect == "Allow"
    statement.Action == "*"
    statement.Resource == "*"
    msg := sprintf("IAM policy '%s' should not grant full access to all resources", [name])
}

# AWS KMS security
deny contains msg if {
    input.resource.aws_kms_key[name]
    key := input.resource.aws_kms_key[name]
    key.enable_key_rotation != true
    msg := sprintf("KMS key '%s' must have key rotation enabled", [name])
}

# AWS ALB security
deny contains msg if {
    input.resource.aws_lb[name]
    alb := input.resource.aws_lb[name]
    alb.load_balancer_type == "application"
    not alb.access_logs
    msg := sprintf("Application Load Balancer '%s' should have access logs enabled", [name])
}

# Terraform state security
deny contains msg if {
    input.terraform[0].backend.s3
    backend := input.terraform[0].backend.s3
    backend.encrypt != true
    msg := "Terraform state must be encrypted"
}

deny contains msg if {
    input.terraform[0].backend.s3
    backend := input.terraform[0].backend.s3
    not backend.dynamodb_table
    msg := "Terraform state must use DynamoDB for state locking"
}

# Resource tagging requirements
required_tags := ["Environment", "Project", "ManagedBy"]

warn contains msg if {
    resource_types := {"aws_instance", "aws_s3_bucket", "aws_rds_instance", "aws_eks_cluster"}
    resource_type := resource_types[_]
    input.resource[resource_type][name]
    resource := input.resource[resource_type][name]
    
    missing_tags := [tag | tag := required_tags[_]; not resource.tags[tag]]
    count(missing_tags) > 0
    
    msg := sprintf("%s '%s' is missing required tags: %v", [resource_type, name, missing_tags])
}

# Environment-specific rules
deny contains msg if {
    input.variable.environment.default == "production"
    input.resource.aws_instance[name]
    instance := input.resource.aws_instance[name]
    instance.monitoring != true
    msg := sprintf("EC2 instance '%s' must have detailed monitoring enabled in production", [name])
}

deny contains msg if {
    input.variable.environment.default == "production"
    input.resource.aws_db_instance[name]
    db := input.resource.aws_db_instance[name]
    db.deletion_protection != true
    msg := sprintf("RDS instance '%s' must have deletion protection enabled in production", [name])
}