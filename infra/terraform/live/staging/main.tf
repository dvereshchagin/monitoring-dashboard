locals {
  common_tags = merge(var.tags, {
    Project     = var.project_name
    Environment = var.environment
    ManagedBy   = "terraform"
  })

  screenshots_bucket_name         = var.s3_screenshots_bucket_name != "" ? var.s3_screenshots_bucket_name : "${var.project_name}-${var.environment}-screenshots-${data.aws_caller_identity.current.account_id}"
  screenshots_replica_bucket_name = var.s3_replica_bucket_name != "" ? var.s3_replica_bucket_name : "${local.screenshots_bucket_name}-dr"
}

data "aws_caller_identity" "current" {}

module "network" {
  source = "../../modules/network"

  project_name         = var.project_name
  environment          = var.environment
  vpc_cidr             = var.vpc_cidr
  availability_zones   = var.availability_zones
  public_subnet_cidrs  = var.public_subnet_cidrs
  private_subnet_cidrs = var.private_subnet_cidrs
  enable_nat_gateway   = var.enable_nat_gateway
  nat_gateway_mode     = var.nat_gateway_mode
  tags                 = local.common_tags
}

module "eks" {
  source = "../../modules/eks"

  project_name              = var.project_name
  environment               = var.environment
  cluster_version           = var.cluster_version
  subnet_ids                = module.network.private_subnet_ids
  node_instance_types       = var.node_instance_types
  node_desired_size         = var.node_desired_size
  node_min_size             = var.node_min_size
  node_max_size             = var.node_max_size
  enable_cluster_autoscaler = var.enable_cluster_autoscaler
  tags                      = local.common_tags
}

module "rds" {
  source = "../../modules/rds"

  project_name               = var.project_name
  environment                = var.environment
  vpc_id                     = module.network.vpc_id
  subnet_ids                 = module.network.private_subnet_ids
  allowed_security_group_ids = [module.eks.cluster_security_group_id]
  db_name                    = var.db_name
  db_username                = var.db_username
  db_password                = var.db_password
  instance_class             = var.db_instance_class
  allocated_storage          = var.db_allocated_storage
  max_allocated_storage      = var.db_max_allocated_storage
  multi_az                   = var.db_multi_az
  backup_retention_period    = var.db_backup_retention_days
  deletion_protection        = var.db_deletion_protection
  apply_immediately          = var.db_apply_immediately
  tags                       = local.common_tags
}

module "ecr" {
  source = "../../modules/ecr"

  repository_name = var.ecr_repository_name
  tags            = local.common_tags
}

# ArgoCD deployment for GitOps
module "argocd" {
  count = var.argocd_enabled ? 1 : 0

  source = "../../modules/argocd"

  project_name     = var.project_name
  environment      = var.environment
  namespace        = "argocd"
  release_name     = "argocd"
  chart_repository = "https://argoproj.github.io/argo-helm"
  chart_version    = var.argocd_chart_version

  repo_url         = var.argocd_repo_url
  repo_secret_name = "argocd-repo-creds"

  server_service_type = "ClusterIP"
  ingress_class_name  = "alb"

  ingress_hosts = var.argocd_ingress_host != "" ? [
    {
      host = var.argocd_ingress_host
      paths = [
        {
          path     = "/"
          pathType = "Prefix"
        }
      ]
    }
  ] : []

  ingress_annotations = var.argocd_ingress_host != "" ? {
    "alb.ingress.kubernetes.io/backend-protocol"     = "HTTPS"
    "alb.ingress.kubernetes.io/healthcheck-path"     = "/healthz"
    "alb.ingress.kubernetes.io/healthcheck-protocol" = "HTTPS"
    "external-dns.alpha.kubernetes.io/hostname"      = var.argocd_ingress_host
  } : {}

  depends_on = [
    module.eks
  ]
}

# Kubernetes Secret for ArgoCD Git credentials
resource "kubernetes_secret" "argocd_repo_creds" {
  count = var.argocd_enabled && var.argocd_repo_url != "" ? 1 : 0

  metadata {
    name      = "argocd-repo-creds"
    namespace = "argocd"
    labels = {
      "argocd.argoproj.io/secret-type" = "repository"
    }
  }

  data = {
    url      = var.argocd_repo_url
    username = var.argocd_repo_username
    password = var.argocd_repo_password
  }

  type = "Opaque"

  depends_on = [
    module.argocd
  ]
}

module "screenshots_s3" {
  source = "../../modules/s3"

  project_name = var.project_name
  environment  = var.environment
  bucket_name  = local.screenshots_bucket_name
  tags         = local.common_tags
}

module "screenshots_s3_replica" {
  count = var.s3_replication_enabled ? 1 : 0

  source = "../../modules/s3"

  providers = {
    aws = aws.dr
  }

  project_name = var.project_name
  environment  = "${var.environment}-dr"
  bucket_name  = local.screenshots_replica_bucket_name
  tags         = local.common_tags
}

data "aws_iam_policy_document" "s3_replication_assume" {
  count = var.s3_replication_enabled ? 1 : 0

  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["s3.amazonaws.com"]
    }

    actions = ["sts:AssumeRole"]
  }
}

data "aws_iam_policy_document" "s3_replication" {
  count = var.s3_replication_enabled ? 1 : 0

  statement {
    effect = "Allow"
    actions = [
      "s3:GetReplicationConfiguration",
      "s3:ListBucket",
    ]
    resources = [module.screenshots_s3.bucket_arn]
  }

  statement {
    effect = "Allow"
    actions = [
      "s3:GetObjectVersionForReplication",
      "s3:GetObjectVersionAcl",
      "s3:GetObjectVersionTagging",
    ]
    resources = ["${module.screenshots_s3.bucket_arn}/*"]
  }

  statement {
    effect = "Allow"
    actions = [
      "s3:ReplicateObject",
      "s3:ReplicateDelete",
      "s3:ReplicateTags",
      "s3:ObjectOwnerOverrideToBucketOwner",
    ]
    resources = ["${module.screenshots_s3_replica[0].bucket_arn}/*"]
  }
}

resource "aws_iam_policy" "s3_replication" {
  count = var.s3_replication_enabled ? 1 : 0

  name        = "${var.project_name}-${var.environment}-s3-replication"
  description = "Permissions for S3 cross-region replication of screenshots bucket"
  policy      = data.aws_iam_policy_document.s3_replication[0].json
  tags        = local.common_tags
}

resource "aws_iam_role" "s3_replication" {
  count = var.s3_replication_enabled ? 1 : 0

  name               = "${var.project_name}-${var.environment}-s3-replication"
  assume_role_policy = data.aws_iam_policy_document.s3_replication_assume[0].json
  tags               = local.common_tags
}

resource "aws_iam_role_policy_attachment" "s3_replication" {
  count = var.s3_replication_enabled ? 1 : 0

  role       = aws_iam_role.s3_replication[0].name
  policy_arn = aws_iam_policy.s3_replication[0].arn
}

resource "aws_s3_bucket_replication_configuration" "screenshots" {
  count = var.s3_replication_enabled ? 1 : 0

  role   = aws_iam_role.s3_replication[0].arn
  bucket = module.screenshots_s3.bucket_name

  rule {
    id     = "replicate-all"
    status = "Enabled"

    filter {}

    destination {
      bucket        = module.screenshots_s3_replica[0].bucket_arn
      storage_class = "STANDARD"
    }

    delete_marker_replication {
      status = "Enabled"
    }
  }

  depends_on = [
    module.screenshots_s3,
    module.screenshots_s3_replica,
    aws_iam_role_policy_attachment.s3_replication,
  ]
}

module "route53" {
  count = var.route53_enabled ? 1 : 0

  source = "../../modules/route53"

  zone_names = var.route53_delegated_zones
  tags       = local.common_tags
}

locals {
  external_dns_enabled = var.route53_enabled && var.external_dns_enabled
  route53_zone_arns    = var.route53_enabled ? values(module.route53[0].zone_arns) : []
}

data "aws_iam_policy_document" "external_dns_assume" {
  count = local.external_dns_enabled ? 1 : 0

  statement {
    effect = "Allow"

    principals {
      type        = "Federated"
      identifiers = [module.eks.oidc_provider_arn]
    }

    actions = ["sts:AssumeRoleWithWebIdentity"]

    condition {
      test     = "StringEquals"
      variable = "${module.eks.oidc_provider_url}:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "${module.eks.oidc_provider_url}:sub"
      values = [
        "system:serviceaccount:${var.external_dns_namespace}:${var.external_dns_service_account_name}"
      ]
    }
  }
}

data "aws_iam_policy_document" "external_dns" {
  count = local.external_dns_enabled ? 1 : 0

  statement {
    effect = "Allow"
    actions = [
      "route53:ChangeResourceRecordSets",
      "route53:ListResourceRecordSets",
      "route53:GetHostedZone",
    ]
    resources = local.route53_zone_arns
  }

  statement {
    effect = "Allow"
    actions = [
      "route53:ListHostedZones",
      "route53:ListHostedZonesByName",
      "route53:ListTagsForResource",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_policy" "external_dns" {
  count = local.external_dns_enabled ? 1 : 0

  name        = "${var.project_name}-${var.environment}-external-dns"
  description = "Permissions for external-dns to manage delegated Route53 zones"
  policy      = data.aws_iam_policy_document.external_dns[0].json
  tags        = local.common_tags
}

resource "aws_iam_role" "external_dns" {
  count = local.external_dns_enabled ? 1 : 0

  name               = "${var.project_name}-${var.environment}-external-dns"
  assume_role_policy = data.aws_iam_policy_document.external_dns_assume[0].json
  tags               = local.common_tags
}

resource "aws_iam_role_policy_attachment" "external_dns" {
  count = local.external_dns_enabled ? 1 : 0

  role       = aws_iam_role.external_dns[0].name
  policy_arn = aws_iam_policy.external_dns[0].arn
}
