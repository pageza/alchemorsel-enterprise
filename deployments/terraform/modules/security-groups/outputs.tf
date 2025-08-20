# Security Groups Module Outputs

output "alb_sg_id" {
  description = "Application Load Balancer security group ID"
  value       = aws_security_group.alb.id
}

output "rds_sg_id" {
  description = "RDS security group ID"
  value       = aws_security_group.rds.id
}

output "redis_sg_id" {
  description = "Redis security group ID"
  value       = aws_security_group.redis.id
}

output "eks_nodes_sg_id" {
  description = "EKS nodes security group ID"
  value       = aws_security_group.eks_nodes.id
}

output "bastion_sg_id" {
  description = "Bastion host security group ID"
  value       = aws_security_group.bastion.id
}

output "ollama_sg_id" {
  description = "Ollama security group ID"
  value       = aws_security_group.ollama.id
}

output "monitoring_sg_id" {
  description = "Monitoring security group ID"
  value       = aws_security_group.monitoring.id
}