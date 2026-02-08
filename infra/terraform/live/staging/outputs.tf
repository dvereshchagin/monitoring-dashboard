output "vpc_id" {
  value = module.network.vpc_id
}

output "eks_cluster_name" {
  value = module.eks.cluster_name
}

output "eks_cluster_endpoint" {
  value = module.eks.cluster_endpoint
}

output "eks_oidc_provider_arn" {
  value = module.eks.oidc_provider_arn
}

output "cluster_autoscaler_role_arn" {
  value = module.eks.cluster_autoscaler_role_arn
}

output "rds_endpoint" {
  value = module.rds.db_endpoint
}

output "rds_port" {
  value = module.rds.db_port
}

output "ecr_repository_url" {
  value = module.ecr.repository_url
}

output "s3_screenshots_bucket_name" {
  value = module.screenshots_s3.bucket_name
}

output "s3_screenshots_replica_bucket_name" {
  value = var.s3_replication_enabled ? module.screenshots_s3_replica[0].bucket_name : null
}

output "s3_replication_role_arn" {
  value = var.s3_replication_enabled ? aws_iam_role.s3_replication[0].arn : null
}

output "route53_zone_ids" {
  value = var.route53_enabled ? module.route53[0].zone_ids : {}
}

output "route53_name_servers" {
  value = var.route53_enabled ? module.route53[0].name_servers : {}
}

output "external_dns_role_arn" {
  value = local.external_dns_enabled ? aws_iam_role.external_dns[0].arn : null
}

output "argocd_namespace" {
  value       = var.argocd_enabled ? module.argocd[0].namespace : null
  description = "Namespace where ArgoCD is deployed"
}

output "argocd_server_url" {
  value       = var.argocd_enabled ? module.argocd[0].server_url : null
  description = "URL to access ArgoCD server UI"
}
