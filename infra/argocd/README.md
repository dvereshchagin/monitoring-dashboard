# ArgoCD GitOps Configuration

This directory contains ArgoCD Application manifests for managing the monitoring-dashboard deployment using GitOps principles.

## Directory Structure

```
argocd/
├── applications/
│   ├── staging/
│   │   └── monitoring-dashboard.yaml  # Main application for staging
│   ├── prod/
│   │   └── (future production apps)
│   └── preview/
│       └── applicationset.yaml        # Auto-deploy preview environments for PRs
├── app-of-apps/
│   ├── staging.yaml                   # App-of-Apps pattern for staging
│   └── (future prod.yaml)
└── README.md
```

## Quick Start

### Prerequisites

1. ArgoCD installed in the cluster (via Terraform in `infra/terraform/live/staging/`)
2. GitHub Personal Access Token for private repository access
3. Access to the Kubernetes cluster

### Initial Setup

1. **Update repository URLs** in all manifests:
   ```bash
   # Replace YOUR_ORG with your GitHub organization/username
   find infra/argocd -name "*.yaml" -type f -exec sed -i '' 's/YOUR_ORG/your-org-name/g' {} +
   ```

2. **Create GitHub token secret** (for ApplicationSet):
   ```bash
   kubectl create secret generic github-token \
     -n argocd \
     --from-literal=token=ghp_YOUR_GITHUB_PAT_TOKEN
   ```

3. **Update image tags** in `applications/staging/monitoring-dashboard.yaml`:
   - Replace `latest` with actual image tags from your ECR repository
   - Example: `abc123def456` (commit SHA)

### Deployment

#### Deploy Staging Environment

```bash
# Apply the app-of-apps pattern (recommended)
kubectl apply -f infra/argocd/app-of-apps/staging.yaml

# This will automatically create the monitoring-dashboard-staging Application
```

Or deploy the application directly:

```bash
kubectl apply -f infra/argocd/applications/staging/monitoring-dashboard.yaml
```

#### Deploy Preview Environments

```bash
# Apply the ApplicationSet
kubectl apply -f infra/argocd/applications/preview/applicationset.yaml

# Preview apps will be automatically created for each open PR
```

### Verify Deployment

```bash
# Check Applications
kubectl -n argocd get applications

# Check ApplicationSets
kubectl -n argocd get applicationsets

# View Application details
kubectl -n argocd get application monitoring-dashboard-staging -o yaml

# Check sync status
kubectl -n argocd get application monitoring-dashboard-staging -o jsonpath='{.status.sync.status}'
```

## GitOps Workflow

### How It Works

1. **Build & Push**: GitHub Actions builds Docker images and pushes to ECR
2. **Update Manifest**: GitHub Actions updates the image tag in `applications/staging/monitoring-dashboard.yaml`
3. **Git Commit**: Changes are committed and pushed to the repository
4. **Auto-Sync**: ArgoCD detects the change and automatically syncs the application
5. **Deploy**: New pods are rolled out with the updated image

### CI/CD Pipeline

The GitOps workflow is implemented in `.github/workflows/gitops-update-staging.yml`:

```yaml
# Simplified workflow steps:
1. Run tests
2. Build Docker images
3. Push images to ECR
4. Update ArgoCD Application manifest with new image tag
5. Commit and push manifest changes
6. ArgoCD auto-syncs the changes
```

### Manual Image Tag Update

To deploy a specific image version:

```bash
# Method 1: Edit manifest directly
vim infra/argocd/applications/staging/monitoring-dashboard.yaml
# Update spec.source.helm.parameters[*].value for image.tag
git add .
git commit -m "chore: update staging image to abc123"
git push

# Method 2: Use yq
yq eval '(.spec.source.helm.parameters[] | select(.name == "image.tag").value) = "abc123"' \
  -i infra/argocd/applications/staging/monitoring-dashboard.yaml
git add .
git commit -m "chore: update staging image to abc123"
git push
```

## Application Configuration

### Staging Application

**File**: `applications/staging/monitoring-dashboard.yaml`

Key features:
- **Automated sync**: Changes are automatically applied
- **Self-heal**: Manual changes in the cluster are reverted
- **Prune**: Removed resources are deleted
- **Namespace creation**: Auto-creates `monitoring-dashboard-staging` namespace
- **Replicas ignore**: HPA-managed replicas are ignored to prevent conflicts

Helm values override:
```yaml
parameters:
  - name: image.repository
    value: "729665432048.dkr.ecr.eu-north-1.amazonaws.com/monitoring-dashboard-api"
  - name: image.tag
    value: "abc123def456"  # Updated by CI/CD
```

### Preview Environments

**File**: `applications/preview/applicationset.yaml`

Automatically creates preview environments for each open PR:
- **PR detection**: Scans GitHub for open PRs every 60 seconds
- **Dynamic namespace**: Each PR gets its own namespace (`pr-123`)
- **Dynamic ingress**: `pr-123.preview.xyibank.ru`
- **Auto-cleanup**: Environments are deleted when PRs are closed

## ArgoCD UI Access

### Login

```bash
# Get admin password
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d && echo

# Access UI
# Staging: https://argocd-staging.xyibank.ru
# Username: admin
# Password: <from command above>
```

### Common Operations

**Sync manually**:
```bash
# CLI
argocd app sync monitoring-dashboard-staging

# Or use the UI: Click "Sync" button
```

**View application details**:
```bash
argocd app get monitoring-dashboard-staging
```

**View logs**:
```bash
argocd app logs monitoring-dashboard-staging
```

**Rollback**:
```bash
# Find previous commit
git log --oneline infra/argocd/applications/staging/monitoring-dashboard.yaml

# Revert
git revert <commit-hash>
git push

# ArgoCD will auto-sync to the reverted version
```

## Testing

### Test Auto-Sync

```bash
# Make a change
echo "# Test" >> README.md
git add README.md
git commit -m "test: validate auto-sync"
git push origin main

# Wait for GitHub Actions to complete
# Wait for ArgoCD to sync (~1-2 minutes)

# Verify
kubectl -n monitoring-dashboard-staging get pods -w
```

### Test Self-Heal

```bash
# Manually scale deployment
kubectl -n monitoring-dashboard-staging scale deployment monitoring-dashboard-staging --replicas=10

# Wait 30-60 seconds
# ArgoCD will automatically restore replicas to the value in Git

kubectl -n monitoring-dashboard-staging get deployment monitoring-dashboard-staging \
  -o jsonpath='{.spec.replicas}'
# Should return: 2
```

### Test Preview Environment

```bash
# Create PR
git checkout -b test-preview
echo "# Test" >> README.md
git add .
git commit -m "test: preview env"
git push origin test-preview

# Open PR in GitHub

# Verify Application created
kubectl -n argocd get applications | grep pr-

# Verify namespace
kubectl get ns | grep pr-

# Close PR → Application should be deleted automatically
```

## Troubleshooting

### Application stuck in "OutOfSync"

```bash
# Check sync status
kubectl -n argocd get application monitoring-dashboard-staging -o yaml | grep -A 10 status

# Force sync
argocd app sync monitoring-dashboard-staging --force
```

### Application "Progressing" for too long

```bash
# Check pod status
kubectl -n monitoring-dashboard-staging get pods

# Check events
kubectl -n monitoring-dashboard-staging get events --sort-by='.lastTimestamp'

# Check ArgoCD logs
kubectl -n argocd logs -l app.kubernetes.io/name=argocd-application-controller
```

### Image pull errors

```bash
# Verify image exists in ECR
aws ecr describe-images \
  --repository-name monitoring-dashboard-api \
  --image-ids imageTag=abc123

# Check pod events
kubectl -n monitoring-dashboard-staging describe pod <pod-name>
```

### ApplicationSet not creating PRs

```bash
# Check ApplicationSet status
kubectl -n argocd get applicationset preview-environments -o yaml

# Check ApplicationSet controller logs
kubectl -n argocd logs -l app.kubernetes.io/name=argocd-applicationset-controller

# Verify GitHub token secret
kubectl -n argocd get secret github-token -o jsonpath='{.data.token}' | base64 -d
```

## Best Practices

1. **Always use Git as source of truth**
   - Never make manual changes to the cluster
   - All changes must go through Git commits

2. **Use meaningful commit messages**
   ```bash
   # Good
   git commit -m "chore(gitops): update staging image to abc123

   Built from commit: abc123def456
   Workflow: https://github.com/org/repo/actions/runs/123"

   # Bad
   git commit -m "update"
   ```

3. **Pin image tags**
   - Never use `latest` in production
   - Use commit SHAs or semantic versions

4. **Monitor sync status**
   - Set up alerts for sync failures
   - Check ArgoCD UI regularly

5. **Test in staging first**
   - Always deploy to staging before production
   - Validate functionality before promoting

## Security

### Secrets Management

- **Never commit secrets** to Git
- Use External Secrets Operator (already configured in Helm chart)
- Secrets are stored in AWS Parameter Store
- ArgoCD does not manage secrets directly

### RBAC

ArgoCD uses Kubernetes RBAC:
```bash
# View ArgoCD service account permissions
kubectl -n argocd get sa argocd-application-controller -o yaml
kubectl -n argocd get clusterrolebinding | grep argocd
```

## Production Rollout

When ready for production:

1. Copy staging configuration to prod:
   ```bash
   cp infra/argocd/applications/staging/monitoring-dashboard.yaml \
      infra/argocd/applications/prod/monitoring-dashboard.yaml
   ```

2. Update prod configuration:
   - Change namespace to `monitoring-dashboard-prod`
   - Change ingress host to `prod.xyibank.ru`
   - **Disable auto-sync** for production:
     ```yaml
     syncPolicy:
       automated: null  # Require manual sync
     ```

3. Create prod app-of-apps:
   ```bash
   cp infra/argocd/app-of-apps/staging.yaml \
      infra/argocd/app-of-apps/prod.yaml
   # Update path to applications/prod
   ```

4. Deploy ArgoCD to production cluster using Terraform

5. Apply prod app-of-apps

## Resources

- [ArgoCD Documentation](https://argo-cd.readthedocs.io/)
- [ArgoCD Best Practices](https://argo-cd.readthedocs.io/en/stable/user-guide/best_practices/)
- [GitOps Principles](https://opengitops.dev/)
- [App-of-Apps Pattern](https://argo-cd.readthedocs.io/en/stable/operator-manual/cluster-bootstrapping/)
