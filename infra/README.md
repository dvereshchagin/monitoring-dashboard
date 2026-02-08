# Infrastructure (Learning Setup)

This repository contains enterprise infrastructure setup for Monitoring Dashboard with:

- AWS + Terraform - Infrastructure as Code
- EKS (Kubernetes) - Container orchestration
- **ArgoCD** - GitOps continuous delivery ✨ NEW
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
- **[INTERVIEW_INFRA_GUIDE.md](./docs/INTERVIEW_INFRA_GUIDE.md)** - Overview, Terraform/K8s/Helm for beginners, dependency diagrams, interview prep

## Environment model

- `staging`
- `prod`
- Feature branch preview namespaces in `staging`: `pr-<number>`

## Folder structure

- `infra/terraform/bootstrap` - creates Terraform state backend (`S3 + DynamoDB`)
- `infra/terraform/modules` - reusable modules (`network`, `eks`, `rds`, `ecr`, `argocd`)
- `infra/terraform/live/staging` - staging environment composition
- `infra/terraform/live/prod` - production environment composition
- `infra/helm/monitoring-dashboard` - application Helm chart
- `infra/argocd` - **ArgoCD GitOps manifests and documentation** ✨ NEW
- `infra/k8s/bootstrap` - bootstrap manifests (for External Secrets)
- `infra/scripts/prepare_preview_namespace.sh` - guardrails for PR preview namespaces
- `infra/scripts/install_cluster_autoscaling.sh` - installs `metrics-server` and `cluster-autoscaler`
- `infra/docs/TASKS.md` - decomposed implementation checklist

## GitOps with ArgoCD ✨ NEW

**ArgoCD** provides GitOps continuous delivery for the monitoring-dashboard application.

### What is GitOps?

GitOps uses Git as the single source of truth for declarative infrastructure and applications. Changes are made via Git commits, and ArgoCD automatically syncs them to the cluster.

### Benefits

- 🔄 **Auto-Sync**: Changes from Git are automatically deployed
- 🛡️ **Self-Heal**: Manual cluster changes are automatically reverted
- 🎯 **Simple Rollback**: Revert Git commits to rollback deployments
- 🔍 **Audit Trail**: Complete history of all changes in Git
- 🚀 **Preview Environments**: Automatic PR-based preview environments

### Documentation

Complete ArgoCD documentation is available in [`infra/argocd/`](./argocd/):

- **[README.md](./argocd/README.md)** - Usage guide and quick start
- **[DEPLOYMENT_GUIDE.md](./argocd/DEPLOYMENT_GUIDE.md)** - Step-by-step deployment instructions
- **[TESTING_GITOPS.md](./argocd/TESTING_GITOPS.md)** - Comprehensive testing procedures
- **[SUMMARY.md](./argocd/SUMMARY.md)** - Implementation overview

### Quick Start

1. **Deploy ArgoCD** via Terraform (see [DEPLOYMENT_GUIDE.md](./argocd/DEPLOYMENT_GUIDE.md)):
   ```bash
   cd infra/terraform/live/staging
   # Configure argocd_enabled = true in terraform.tfvars
   terraform apply
   ```

2. **Apply ArgoCD Applications**:
   ```bash
   kubectl apply -f infra/argocd/app-of-apps/staging.yaml
   ```

3. **Access ArgoCD UI**:
   - URL: https://argocd-staging.xyibank.ru
   - Username: admin
   - Password: `kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d`

### ArgoCD Workflow

```
Developer → Git Push → GitHub Actions (build images) →
→ Update ArgoCD manifest → Git Commit →
→ ArgoCD detects change → Syncs to cluster → Deployed
```

**Key Difference**: GitHub Actions only builds images and updates manifests. ArgoCD handles all cluster deployments.

## GitHub workflows

- `.github/workflows/ci-pr.yml` - PR checks
- `.github/workflows/deploy-preview.yml` - deploy preview for feature PRs (legacy)
- `.github/workflows/cleanup-preview.yml` - manual preview cleanup
- `.github/workflows/gitops-update-staging.yml` - **GitOps: Build and update ArgoCD manifest** ✨ NEW
- `.github/workflows/_backup_deploy-staging.yml.disabled` - (backup of old push-based workflow)
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
- `STAGING_ALB_CERTIFICATE_ARN`
- `PROD_ALB_CERTIFICATE_ARN`
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

## One-command staging deploy

Run from repository root:

```bash
./infra/scripts/deploy_staging.sh
```

Optional overrides:

```bash
AWS_ACCOUNT_ID=729665432048 \
AWS_REGION=eu-north-1 \
EKS_CLUSTER_NAME=monitoring-dashboard-staging \
STAGING_HOST=staging.xyibank.ru \
ENABLE_HTTPS=true \
ALB_CERTIFICATE_ARN=arn:aws:acm:eu-north-1:123456789012:certificate/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx \
IMAGE_TAG=staging-manual-001 \
./infra/scripts/deploy_staging.sh
```

What it does:
- builds and pushes `linux/amd64` image to ECR
- updates Helm release `monitoring-dashboard-staging`
- runs DB schema job in cluster (creates `metrics` table/indexes if missing)
- prints domain and ALB URL

## Autoscaling (pods + nodes)

### Pod autoscaling (HPA)

- HPA is enabled for staging/prod via Helm values.
- CPU and memory utilization targets are configured in:
  - `infra/helm/monitoring-dashboard/values-staging.yaml`
  - `infra/helm/monitoring-dashboard/values-prod.yaml`

### Node autoscaling (EKS node group)

- Terraform configures EKS node group min/max sizes.
- Terraform also creates IRSA role for `cluster-autoscaler` and adds auto-discovery tags to node group resources.

After `terraform apply`, install autoscaling add-ons:

```bash
./infra/scripts/install_cluster_autoscaling.sh
```

Optional overrides:

```bash
AWS_REGION=eu-north-1 \
EKS_CLUSTER_NAME=monitoring-dashboard-staging \
TERRAFORM_DIR=infra/terraform/live/staging \
./infra/scripts/install_cluster_autoscaling.sh
```

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
