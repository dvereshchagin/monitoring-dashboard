# Infrastructure (Learning Setup)

This repository contains enterprise infrastructure setup for Monitoring Dashboard with:

- AWS + Terraform - Infrastructure as Code
- EKS (Kubernetes) - Container orchestration
- Helm deployment - Application packaging
- Docker + ECR - Container registry
- GitHub Actions - CI/CD pipeline
- Feature branch preview environments

## 📚 Documentation

**New to Terraform?** Start here:
- **[TERRAFORM_INDEX.md](./docs/TERRAFORM_INDEX.md)** - Navigation guide with learning plan
- **[TERRAFORM_EXPLAINED.md](./docs/TERRAFORM_EXPLAINED.md)** - Complete Terraform explanation with examples
- **[TERRAFORM_CHEATSHEET.md](./docs/TERRAFORM_CHEATSHEET.md)** - Quick reference for commands and syntax
- **[TERRAFORM_PROJECT_GUIDE.md](./docs/TERRAFORM_PROJECT_GUIDE.md)** - Project-specific architecture and workflows

## Environment model

- `staging`
- `prod`
- Feature branch preview namespaces in `staging`: `pr-<number>`

## Folder structure

- `infra/terraform/bootstrap` - creates Terraform state backend (`S3 + DynamoDB`)
- `infra/terraform/modules` - reusable modules (`network`, `eks`, `rds`, `ecr`)
- `infra/terraform/live/staging` - staging environment composition
- `infra/terraform/live/prod` - production environment composition
- `infra/helm/monitoring-dashboard` - application Helm chart
- `infra/k8s/bootstrap` - bootstrap manifests (for External Secrets)
- `infra/scripts/prepare_preview_namespace.sh` - guardrails for PR preview namespaces
- `infra/docs/TASKS.md` - decomposed implementation checklist

## GitHub workflows

- `.github/workflows/ci-pr.yml` - PR checks
- `.github/workflows/deploy-preview.yml` - deploy preview for feature PRs
- `.github/workflows/cleanup-preview.yml` - manual preview cleanup
- `.github/workflows/deploy-staging.yml` - auto deploy staging from `main`
- `.github/workflows/deploy-prod.yml` - manual deploy to prod

## Required GitHub variables

Repository or Environment variables:

- `AWS_REGION`
- `ECR_REPOSITORY`
- `STAGING_EKS_CLUSTER_NAME`
- `PROD_EKS_CLUSTER_NAME`
- `AWS_ROLE_ARN_STAGING`
- `AWS_ROLE_ARN_PROD`
- `STAGING_HOST`
- `PROD_HOST`
- `PREVIEW_BASE_DOMAIN`

## Required Kubernetes prerequisites

Install these in each EKS cluster:

1. AWS Load Balancer Controller
2. External Secrets Operator
3. `ClusterSecretStore` from `infra/k8s/bootstrap/external-secrets/cluster-secret-store.yaml`

## Terraform quickstart

1. Bootstrap backend:

```bash
cd infra/terraform/bootstrap
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform apply
```

2. Configure and deploy staging:

```bash
cd infra/terraform/live/staging
cp backend.hcl.example backend.hcl
cp terraform.tfvars.example terraform.tfvars
# set db_password and other real values
terraform init -backend-config=backend.hcl
terraform plan
terraform apply
```

3. Configure and deploy production (same pattern in `live/prod`).

## Feature branch contour

- Branch naming: `feature/<ticket>-<slug>`
- PR to `main` runs `ci-pr.yml`
- For non-fork PRs, preview deployment creates:
  - namespace: `pr-<number>`
  - release: `monitoring-dashboard-pr-<number>`
  - URL: `https://pr-<number>.<PREVIEW_BASE_DOMAIN>`
- Cleanup is manual via `cleanup-preview.yml`

## Security notes for learning mode

- Do not commit real credentials; keep secret values only in AWS Secrets Manager and GitHub Environments.
- Use OIDC federation for GitHub Actions roles (no long-lived AWS keys).
