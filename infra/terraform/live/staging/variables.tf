variable "aws_region" {
  type = string
}

variable "project_name" {
  type = string
}

variable "environment" {
  type = string
}

variable "vpc_cidr" {
  type = string
}

variable "availability_zones" {
  type = list(string)
}

variable "public_subnet_cidrs" {
  type = list(string)
}

variable "private_subnet_cidrs" {
  type = list(string)
}

variable "enable_nat_gateway" {
  type    = bool
  default = true
}

variable "nat_gateway_mode" {
  type    = string
  default = "single"
}

variable "cluster_version" {
  type    = string
  default = "1.30"
}

variable "node_instance_types" {
  type    = list(string)
  default = ["t3.small"]
}

variable "node_desired_size" {
  type    = number
  default = 1
}

variable "node_min_size" {
  type    = number
  default = 1
}

variable "node_max_size" {
  type    = number
  default = 2
}

variable "enable_cluster_autoscaler" {
  type    = bool
  default = true
}

variable "db_name" {
  type = string
}

variable "db_username" {
  type = string
}

variable "db_password" {
  type      = string
  sensitive = true
}

variable "db_instance_class" {
  type    = string
  default = "db.t3.micro"
}

variable "db_allocated_storage" {
  type    = number
  default = 20
}

variable "db_max_allocated_storage" {
  type    = number
  default = 20
}

variable "db_multi_az" {
  type    = bool
  default = true
}

variable "db_backup_retention_days" {
  type    = number
  default = 1
}

variable "db_deletion_protection" {
  type    = bool
  default = true
}

variable "db_apply_immediately" {
  type    = bool
  default = false
}

variable "ecr_repository_name" {
  type    = string
  default = "monitoring-dashboard-api"
}

variable "s3_screenshots_bucket_name" {
  type    = string
  default = ""
}

variable "s3_replication_enabled" {
  type    = bool
  default = false
}

variable "s3_replication_region" {
  type    = string
  default = "eu-central-1"

  validation {
    condition     = !var.s3_replication_enabled || var.s3_replication_region != var.aws_region
    error_message = "s3_replication_region must be different from aws_region when replication is enabled."
  }
}

variable "s3_replica_bucket_name" {
  type    = string
  default = ""

  validation {
    condition     = var.s3_replica_bucket_name == "" || can(regex("^[a-z0-9.-]{3,63}$", var.s3_replica_bucket_name))
    error_message = "s3_replica_bucket_name must be empty or a valid S3 bucket name."
  }
}

variable "route53_enabled" {
  type    = bool
  default = false
}

variable "route53_delegated_zones" {
  type    = list(string)
  default = []
}

variable "external_dns_enabled" {
  type    = bool
  default = false
}

variable "external_dns_namespace" {
  type    = string
  default = "kube-system"
}

variable "external_dns_service_account_name" {
  type    = string
  default = "external-dns"
}

variable "tags" {
  type    = map(string)
  default = {}
}

# ArgoCD Configuration
variable "argocd_enabled" {
  type        = bool
  default     = false
  description = "Enable ArgoCD deployment via Terraform"
}

variable "argocd_repo_url" {
  type        = string
  default     = ""
  description = "Git repository URL for ArgoCD (current repo)"
}

variable "argocd_repo_username" {
  type        = string
  default     = ""
  sensitive   = true
  description = "Git username or 'git' for token-based auth"
}

variable "argocd_repo_password" {
  type        = string
  default     = ""
  sensitive   = true
  description = "GitHub Personal Access Token"
}

variable "argocd_ingress_host" {
  type        = string
  default     = ""
  description = "ArgoCD UI hostname (e.g., argocd-staging.xyibank.ru)"
}

variable "argocd_chart_version" {
  type        = string
  default     = "7.7.12"
  description = "ArgoCD Helm chart version"
}
