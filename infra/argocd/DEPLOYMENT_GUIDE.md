# ArgoCD Deployment Guide

This guide provides step-by-step instructions for deploying ArgoCD to the staging environment using the GitOps approach.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Phase 1: Deploy ArgoCD via Terraform](#phase-1-deploy-argocd-via-terraform)
3. [Phase 2: Configure ArgoCD Applications](#phase-2-configure-argocd-applications)
4. [Phase 3: Verify GitOps Workflow](#phase-3-verify-gitops-workflow)
5. [Phase 4: Production Rollout](#phase-4-production-rollout)
6. [Rollback Procedures](#rollback-procedures)

## Prerequisites

### Required Tools

```bash
# Terraform
terraform --version  # >= 1.5.0

# AWS CLI
aws --version  # >= 2.0.0

# kubectl
kubectl version --client  # >= 1.28.0

# Helm
helm version  # >= 3.12.0

# yq (for manifest updates)
yq --version  # >= 4.0.0

# ArgoCD CLI (optional but recommended)
argocd version  # >= 2.8.0
```

### AWS Credentials

```bash
# Configure AWS credentials
aws configure
# Or use AWS SSO
aws sso login --profile your-profile

# Verify access to staging account
aws sts get-caller-identity
```

### GitHub Personal Access Token

Create a GitHub PAT with the following scopes:
- `repo` (Full control of private repositories)

```bash
# Create token at: https://github.com/settings/tokens/new
# Save it securely - you'll need it for terraform.tfvars
```

### Repository Access

```bash
# Clone repository
git clone https://github.com/YOUR_ORG/monitoring-dashboard.git
cd monitoring-dashboard

# Ensure you're on main branch
git checkout main
git pull origin main
```

## Phase 1: Deploy ArgoCD via Terraform

### Step 1.1: Update Repository URLs

```bash
# Update YOUR_ORG placeholder in all ArgoCD manifests
find infra/argocd -name "*.yaml" -type f -exec sed -i '' 's/YOUR_ORG/your-github-org/g' {} +

# Verify changes
grep "repoURL" infra/argocd/applications/staging/monitoring-dashboard.yaml
# Should show your actual GitHub URL
```

### Step 1.2: Configure Terraform Variables

```bash
cd infra/terraform/live/staging

# Copy example tfvars
cp terraform.tfvars.example terraform.tfvars

# Edit terraform.tfvars
vim terraform.tfvars
```

Add/update the following variables:

```hcl
# ArgoCD GitOps Configuration
argocd_enabled          = true
argocd_repo_url         = "https://github.com/your-org/monitoring-dashboard.git"
argocd_repo_username    = "git"
argocd_repo_password    = "ghp_YOUR_GITHUB_PAT_TOKEN"  # Replace with actual token
argocd_ingress_host     = "argocd-staging.xyibank.ru"
argocd_chart_version    = "7.7.12"
```

**Security Note:** Never commit the actual `terraform.tfvars` file with secrets!

```bash
# Verify tfvars is gitignored
git check-ignore terraform.tfvars
# Should output: terraform.tfvars
```

### Step 1.3: Initialize Terraform

```bash
# Initialize Terraform (downloads new providers: kubernetes, helm)
terraform init

# Expected output:
# - Downloading hashicorp/kubernetes
# - Downloading hashicorp/helm
```

### Step 1.4: Plan Terraform Changes

```bash
# Create execution plan
terraform plan -out=tfplan-argocd

# Review the plan carefully:
# Expected resources to be created:
# - module.argocd[0].helm_release.this
# - module.argocd[0].kubernetes_namespace.this
# - kubernetes_secret.argocd_repo_creds[0]
# - ~10-15 resources total
```

### Step 1.5: Apply Terraform

```bash
# Apply the plan
terraform apply tfplan-argocd

# Confirm when prompted: yes

# Expected output:
# - Creating argocd namespace
# - Installing ArgoCD Helm chart
# - Creating repository credentials secret
# - Apply complete! Resources: XX added, 0 changed, 0 destroyed.

# Note: This will take 3-5 minutes
```

### Step 1.6: Verify ArgoCD Installation

```bash
# Configure kubectl
aws eks update-kubeconfig --name monitoring-dashboard-staging --region eu-north-1

# Check ArgoCD namespace
kubectl get namespace argocd
# Should show: argocd   Active   XXs

# Check ArgoCD pods
kubectl -n argocd get pods
# All pods should be Running:
# - argocd-application-controller
# - argocd-dex-server
# - argocd-redis
# - argocd-repo-server
# - argocd-server
# - argocd-applicationset-controller

# Wait for all pods to be ready
kubectl -n argocd wait --for=condition=Ready pods --all --timeout=5m

# Check ArgoCD ingress
kubectl -n argocd get ingress argocd-server
# Should show ALB hostname

# Get ALB URL
ALB_URL=$(kubectl -n argocd get ingress argocd-server \
  -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')
echo "ArgoCD ALB: $ALB_URL"

# Wait for DNS propagation (External DNS creates the record)
echo "Waiting for DNS propagation..."
for i in {1..30}; do
  if dig +short argocd-staging.xyibank.ru | grep -q .; then
    echo "DNS record created!"
    break
  fi
  sleep 10
done

# Test ArgoCD health endpoint
curl -k https://$ALB_URL/healthz
# Expected: ok
```

### Step 1.7: Access ArgoCD UI

```bash
# Get admin password
ARGOCD_PASSWORD=$(kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d)
echo "ArgoCD Admin Password: $ARGOCD_PASSWORD"

# Save password securely
echo $ARGOCD_PASSWORD > ~/.argocd-staging-password
chmod 600 ~/.argocd-staging-password

# Open ArgoCD UI
echo "ArgoCD UI: https://argocd-staging.xyibank.ru"
echo "Username: admin"
echo "Password: $ARGOCD_PASSWORD"

# Or use CLI login
argocd login argocd-staging.xyibank.ru \
  --username admin \
  --password "$ARGOCD_PASSWORD" \
  --insecure
```

### Step 1.8: Verify Outputs

```bash
# Check Terraform outputs
terraform output argocd_namespace
# Expected: argocd

terraform output argocd_server_url
# Expected: https://argocd-staging.xyibank.ru
```

## Phase 2: Configure ArgoCD Applications

### Step 2.1: Create GitHub Token Secret

```bash
# Create secret for ApplicationSet (PR-based preview environments)
kubectl create secret generic github-token \
  -n argocd \
  --from-literal=token=ghp_YOUR_GITHUB_PAT_TOKEN

# Verify secret created
kubectl -n argocd get secret github-token
```

### Step 2.2: Update Image Tags

Before deploying the application, update the image tags in the Application manifest:

```bash
cd ../../../../..  # Back to repository root

# Check current images in ECR
aws ecr describe-images \
  --repository-name monitoring-dashboard-api \
  --query 'imageDetails[*].[imageTags[0],imagePushedAt]' \
  --output table

# Select a valid image tag (e.g., latest commit SHA)
CURRENT_TAG="<your-current-image-tag>"

# Update Application manifest
yq eval "(.spec.source.helm.parameters[] | select(.name == \"image.tag\").value) = \"$CURRENT_TAG\"" \
  -i infra/argocd/applications/staging/monitoring-dashboard.yaml

yq eval "(.spec.source.helm.parameters[] | select(.name == \"releaseAnalyzer.image.tag\").value) = \"${CURRENT_TAG}-release-analyzer\"" \
  -i infra/argocd/applications/staging/monitoring-dashboard.yaml

yq eval "(.spec.source.helm.parameters[] | select(.name == \"gateway.image.tag\").value) = \"${CURRENT_TAG}-gateway\"" \
  -i infra/argocd/applications/staging/monitoring-dashboard.yaml

# Verify changes
grep "value:" infra/argocd/applications/staging/monitoring-dashboard.yaml
```

### Step 2.3: Commit and Push Changes

```bash
# Commit ArgoCD manifests
git add infra/argocd/
git commit -m "feat(gitops): configure ArgoCD for staging

- Add Application manifest for monitoring-dashboard-staging
- Add app-of-apps pattern for staging
- Add ApplicationSet for preview environments
- Update image tags to $CURRENT_TAG"

git push origin main
```

### Step 2.4: Deploy App-of-Apps

```bash
# Apply app-of-apps (recommended approach)
kubectl apply -f infra/argocd/app-of-apps/staging.yaml

# Verify app-of-apps created
kubectl -n argocd get application staging-apps
# Expected: NAME=staging-apps, SYNC STATUS=Synced, HEALTH STATUS=Healthy

# Wait for child application to be created
sleep 10

# Verify monitoring-dashboard-staging Application created
kubectl -n argocd get application monitoring-dashboard-staging
```

Alternative: Deploy application directly (skip app-of-apps):

```bash
kubectl apply -f infra/argocd/applications/staging/monitoring-dashboard.yaml
```

### Step 2.5: Deploy ApplicationSet (Preview Environments)

```bash
# Apply ApplicationSet
kubectl apply -f infra/argocd/applications/preview/applicationset.yaml

# Verify ApplicationSet created
kubectl -n argocd get applicationset preview-environments
```

### Step 2.6: Monitor Initial Sync

```bash
# Watch application sync
watch -n 5 'kubectl -n argocd get application monitoring-dashboard-staging'

# Or use ArgoCD CLI
argocd app get monitoring-dashboard-staging --watch

# Expected progression:
# 1. SYNC STATUS: OutOfSync → Syncing → Synced
# 2. HEALTH STATUS: Missing → Progressing → Healthy

# Wait for sync to complete (typically 2-5 minutes)
argocd app wait monitoring-dashboard-staging --health --timeout 600
```

### Step 2.7: Verify Application Deployment

```bash
# Check application status
kubectl -n argocd get application monitoring-dashboard-staging -o yaml | yq eval '.status'

# Check namespace created
kubectl get namespace monitoring-dashboard-staging

# Check pods running
kubectl -n monitoring-dashboard-staging get pods
# All pods should be Running

# Check services
kubectl -n monitoring-dashboard-staging get svc

# Check ingress
kubectl -n monitoring-dashboard-staging get ingress

# Test application endpoint
curl -I https://staging.xyibank.ru
# Expected: HTTP 200 or 302
```

## Phase 3: Verify GitOps Workflow

### Step 3.1: Test Auto-Sync

```bash
# Make a test change
echo "# GitOps test $(date)" >> README.md
git add README.md
git commit -m "test: validate gitops auto-sync"
git push origin main

# Monitor GitHub Actions
echo "GitHub Actions: https://github.com/YOUR_ORG/monitoring-dashboard/actions"

# Wait for workflow to complete (~2-3 minutes)
# The workflow will:
# 1. Run tests
# 2. Build images
# 3. Push to ECR
# 4. Update ArgoCD manifest
# 5. Commit and push

# Pull latest changes
git pull origin main

# Verify manifest updated
git log --oneline -1 infra/argocd/applications/staging/monitoring-dashboard.yaml
# Should show: "chore(gitops): update staging image to XXXXXX"

# Monitor ArgoCD sync
watch -n 5 'kubectl -n argocd get application monitoring-dashboard-staging'

# Wait for new pods
kubectl -n monitoring-dashboard-staging get pods -w

# Verify new image deployed
kubectl -n monitoring-dashboard-staging get deployment monitoring-dashboard-staging \
  -o jsonpath='{.spec.template.spec.containers[0].image}'
```

### Step 3.2: Test Self-Heal

```bash
# Manually scale deployment
kubectl -n monitoring-dashboard-staging scale deployment monitoring-dashboard-staging --replicas=10

# Check current replicas
kubectl -n monitoring-dashboard-staging get deployment monitoring-dashboard-staging \
  -o jsonpath='{.spec.replicas}'
# Should show: 10

# Wait 60 seconds for self-heal
echo "Waiting for self-heal..."
sleep 60

# Check replicas restored
kubectl -n monitoring-dashboard-staging get deployment monitoring-dashboard-staging \
  -o jsonpath='{.spec.replicas}'
# Should show: 2 (from values-staging.yaml)

echo "✅ Self-heal verified!"
```

### Step 3.3: Test Preview Environment

```bash
# Create test PR
git checkout -b test/preview-validation
echo "# Preview test" >> README.md
git add README.md
git commit -m "test: preview environment"
git push origin test/preview-validation

# Create PR in GitHub UI
gh pr create --title "Test: Preview Environment" --body "Testing ArgoCD preview environments"

# Wait for ApplicationSet to detect PR
echo "Waiting for ApplicationSet..."
sleep 70

# Get PR number
PR_NUMBER=$(gh pr view --json number -q .number)

# Verify preview application created
kubectl -n argocd get application monitoring-dashboard-pr-${PR_NUMBER}

# Check preview namespace
kubectl get namespace pr-${PR_NUMBER}

# Check preview pods
kubectl -n pr-${PR_NUMBER} get pods

# Close PR
gh pr close ${PR_NUMBER}

# Verify cleanup
sleep 70
kubectl -n argocd get application | grep pr-${PR_NUMBER} || echo "✅ Preview app cleaned up"
```

## Phase 4: Production Rollout

**IMPORTANT:** Only proceed after 1-2 weeks of stable staging operation!

### Prerequisites for Production

- [ ] Staging has been running ArgoCD successfully for 1-2 weeks
- [ ] All tests pass consistently
- [ ] Team is trained on GitOps workflow
- [ ] Incident response procedures documented
- [ ] Rollback procedures tested

### Production Deployment Steps

1. **Create Production ArgoCD Module**

```bash
cd infra/terraform/live/prod

# Add same provider configuration as staging
vim providers.tf
# Copy kubernetes and helm providers from staging/providers.tf

# Add ArgoCD variables
vim variables.tf
# Copy ArgoCD variables from staging/variables.tf

# Integrate ArgoCD module
vim main.tf
# Copy ArgoCD module and kubernetes_secret from staging/main.tf

# Add outputs
vim outputs.tf
# Copy ArgoCD outputs from staging/outputs.tf

# Configure variables
vim terraform.tfvars
argocd_enabled = true
argocd_repo_url = "https://github.com/your-org/monitoring-dashboard.git"
argocd_repo_username = "git"
argocd_repo_password = "ghp_YOUR_PROD_PAT"
argocd_ingress_host = "argocd-prod.xyibank.ru"
```

2. **Deploy ArgoCD to Production**

```bash
terraform init
terraform plan -out=tfplan-argocd-prod
terraform apply tfplan-argocd-prod
```

3. **Create Production Application Manifest**

```bash
cd ../../../../..  # Back to repo root

# Copy staging manifest to prod
cp infra/argocd/applications/staging/monitoring-dashboard.yaml \
   infra/argocd/applications/prod/monitoring-dashboard.yaml

# Edit for production
vim infra/argocd/applications/prod/monitoring-dashboard.yaml
```

Update:
- `metadata.name`: `monitoring-dashboard-prod`
- `spec.source.helm.valueFiles`: Add `values-prod.yaml`
- `spec.destination.namespace`: `monitoring-dashboard-prod`
- `spec.syncPolicy.automated`: **REMOVE** (manual sync for production!)

```yaml
syncPolicy:
  # No automated sync for production!
  syncOptions:
    - CreateNamespace=true
  retry:
    limit: 5
```

4. **Create Production App-of-Apps**

```bash
cp infra/argocd/app-of-apps/staging.yaml \
   infra/argocd/app-of-apps/prod.yaml

vim infra/argocd/app-of-apps/prod.yaml
```

Update:
- `metadata.name`: `prod-apps`
- `spec.source.path`: `infra/argocd/applications/prod`

5. **Commit and Deploy**

```bash
git add infra/argocd/applications/prod/
git add infra/argocd/app-of-apps/prod.yaml
git commit -m "feat(gitops): add ArgoCD production configuration"
git push origin main

# Apply to production cluster
kubectl apply -f infra/argocd/app-of-apps/prod.yaml

# Manually sync production (no auto-sync!)
argocd app sync monitoring-dashboard-prod
```

## Rollback Procedures

### Rollback ArgoCD Installation

If ArgoCD causes issues:

```bash
cd infra/terraform/live/staging

# Disable ArgoCD
vim terraform.tfvars
# Set: argocd_enabled = false

# Apply changes
terraform apply

# Re-enable old deployment workflow
cd ../../../..
mv .github/workflows/_backup_deploy-staging.yml.disabled \
   .github/workflows/deploy-staging.yml

git add .github/workflows/
git commit -m "rollback: restore push-based deployment"
git push origin main
```

### Rollback Application Version

```bash
# View commit history
git log --oneline infra/argocd/applications/staging/monitoring-dashboard.yaml

# Revert to previous version
git revert HEAD
git push origin main

# ArgoCD will auto-sync the rollback
```

### Emergency Manual Deployment

If ArgoCD is completely broken:

```bash
# Use old workflow manually
cd .github/workflows
mv _backup_deploy-staging.yml.disabled deploy-staging.yml
git add .
git commit -m "emergency: restore manual deployment"
git push origin main
```

## Troubleshooting

### Issue: ArgoCD Pods Not Starting

```bash
# Check pod logs
kubectl -n argocd logs -l app.kubernetes.io/name=argocd-server

# Check events
kubectl -n argocd get events --sort-by='.lastTimestamp'

# Describe pod
kubectl -n argocd describe pod -l app.kubernetes.io/name=argocd-server
```

### Issue: Application Not Syncing

```bash
# Check repository connection
argocd repo list

# Force refresh
argocd app get monitoring-dashboard-staging --refresh

# Check sync errors
argocd app get monitoring-dashboard-staging

# Manual sync
argocd app sync monitoring-dashboard-staging
```

### Issue: Cannot Access ArgoCD UI

```bash
# Check ingress
kubectl -n argocd describe ingress argocd-server

# Check ALB
aws elbv2 describe-load-balancers --query 'LoadBalancers[?contains(LoadBalancerName, `argocd`)]'

# Check DNS
dig argocd-staging.xyibank.ru

# Port forward as backup
kubectl -n argocd port-forward svc/argocd-server 8080:443
# Access at: https://localhost:8080
```

## Next Steps

After successful deployment:

1. **Monitor for 1-2 weeks** in staging
2. **Run all tests** from `TESTING_GITOPS.md` regularly
3. **Train team** on GitOps workflow
4. **Document incidents** and resolutions
5. **Plan production rollout** after validation period
6. **Set up monitoring** and alerts for ArgoCD

## Resources

- [ArgoCD Documentation](https://argo-cd.readthedocs.io/)
- [GitOps Principles](https://opengitops.dev/)
- [Testing Guide](./TESTING_GITOPS.md)
- [ArgoCD Best Practices](https://argo-cd.readthedocs.io/en/stable/user-guide/best_practices/)
