# ArgoCD GitOps Testing & Validation Guide

This document provides comprehensive testing procedures for validating the ArgoCD GitOps implementation.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Test 1: Auto-Sync](#test-1-auto-sync)
- [Test 2: Self-Heal](#test-2-self-heal)
- [Test 3: Preview Environments](#test-3-preview-environments)
- [Test 4: Rollback via Git](#test-4-rollback-via-git)
- [Test 5: Manual Sync](#test-5-manual-sync)
- [Test 6: Sync Wave](#test-6-sync-wave)
- [Validation Checklist](#validation-checklist)
- [Performance Testing](#performance-testing)
- [Disaster Recovery Testing](#disaster-recovery-testing)

## Prerequisites

Before running tests, ensure:

```bash
# 1. ArgoCD is deployed
kubectl -n argocd get pods
# All pods should be Running

# 2. ArgoCD CLI is installed (optional but recommended)
brew install argocd
# or
curl -sSL -o argocd https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
chmod +x argocd
sudo mv argocd /usr/local/bin/

# 3. Login to ArgoCD (CLI)
argocd login argocd-staging.xyibank.ru
# Username: admin
# Password: (from kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)

# 4. Configure kubectl for staging cluster
aws eks update-kubeconfig --name monitoring-dashboard-staging --region eu-north-1
```

## Test 1: Auto-Sync

**Purpose**: Verify that ArgoCD automatically detects and syncs changes from Git.

### Steps

```bash
# 1. Check current application status
kubectl -n argocd get application monitoring-dashboard-staging -o jsonpath='{.status.sync.status}'
# Expected: Synced

# 2. Check current image tag
kubectl -n monitoring-dashboard-staging get deployment monitoring-dashboard-staging \
  -o jsonpath='{.spec.template.spec.containers[0].image}'
# Note the current image tag

# 3. Make a test change in the repository
git checkout -b test/auto-sync
echo "# Auto-sync test $(date)" >> README.md
git add README.md
git commit -m "test: validate auto-sync"
git push origin test/auto-sync

# Create and merge PR (or push directly to main if allowed)

# 4. Wait for GitHub Actions workflow to complete
# Monitor at: https://github.com/YOUR_ORG/monitoring-dashboard/actions

# 5. Wait for ArgoCD to detect changes (up to 3 minutes by default)
watch -n 5 'kubectl -n argocd get application monitoring-dashboard-staging -o jsonpath="{.status.sync.status}"'

# 6. Verify new pods are created
kubectl -n monitoring-dashboard-staging get pods -w

# 7. Check new image tag
kubectl -n monitoring-dashboard-staging get deployment monitoring-dashboard-staging \
  -o jsonpath='{.spec.template.spec.containers[0].image}'
# Should show new image tag

# 8. Verify application health
kubectl -n argocd get application monitoring-dashboard-staging \
  -o jsonpath='{.status.health.status}'
# Expected: Healthy
```

### Expected Results

- ✅ GitHub Actions workflow completes successfully
- ✅ ArgoCD Application manifest updated with new image tag
- ✅ ArgoCD detects change within 3 minutes
- ✅ Application status: Synced
- ✅ Health status: Healthy
- ✅ New pods running with updated image

### Troubleshooting

**Issue**: ArgoCD not detecting changes

```bash
# Force refresh
argocd app get monitoring-dashboard-staging --refresh

# Check ArgoCD application controller logs
kubectl -n argocd logs -l app.kubernetes.io/name=argocd-application-controller --tail=100

# Check repository connection
argocd repo list
```

**Issue**: Sync failing

```bash
# View sync error details
argocd app get monitoring-dashboard-staging

# View application events
kubectl -n argocd describe application monitoring-dashboard-staging

# Force sync
argocd app sync monitoring-dashboard-staging
```

## Test 2: Self-Heal

**Purpose**: Verify that ArgoCD automatically reverts manual changes made directly to the cluster.

### Steps

```bash
# 1. Check current replica count (from Git)
yq eval '.spec.source.helm.parameters[] | select(.name == "replicaCount")' \
  infra/argocd/applications/staging/monitoring-dashboard.yaml
# Or check values-staging.yaml: should be 2

# 2. Manually scale deployment (direct cluster modification)
kubectl -n monitoring-dashboard-staging scale deployment monitoring-dashboard-staging --replicas=10

# 3. Verify manual change
kubectl -n monitoring-dashboard-staging get deployment monitoring-dashboard-staging \
  -o jsonpath='{.spec.replicas}'
# Should show: 10

# 4. Wait for ArgoCD self-heal (typically 30-60 seconds)
echo "Waiting for self-heal..."
sleep 60

# 5. Check if replicas restored to Git value
kubectl -n monitoring-dashboard-staging get deployment monitoring-dashboard-staging \
  -o jsonpath='{.spec.replicas}'
# Should show: 2 (restored from Git)

# 6. Check ArgoCD application status
kubectl -n argocd get application monitoring-dashboard-staging \
  -o jsonpath='{.status.sync.status}'
# Should be: Synced

# 7. View self-heal event in ArgoCD
argocd app get monitoring-dashboard-staging | grep -A 5 "Last Sync"
```

### Expected Results

- ✅ Manual scale change detected
- ✅ ArgoCD auto-heals within 60 seconds
- ✅ Replicas restored to value from Git (2)
- ✅ Application remains in Synced status
- ✅ No manual intervention required

### Advanced Self-Heal Tests

**Test annotations change:**

```bash
# Add annotation
kubectl -n monitoring-dashboard-staging annotate deployment monitoring-dashboard-staging test=manual

# Wait 60 seconds
sleep 60

# Verify annotation removed
kubectl -n monitoring-dashboard-staging get deployment monitoring-dashboard-staging \
  -o jsonpath='{.metadata.annotations}' | grep -c "test" || echo "Annotation removed ✅"
```

**Test label change:**

```bash
# Change label
kubectl -n monitoring-dashboard-staging label deployment monitoring-dashboard-staging test=manual --overwrite

# Wait 60 seconds
sleep 60

# Verify label reverted
kubectl -n monitoring-dashboard-staging get deployment monitoring-dashboard-staging \
  -o jsonpath='{.metadata.labels}' | grep -c "test" && echo "Label NOT reverted ❌" || echo "Label reverted ✅"
```

### Troubleshooting

**Issue**: Self-heal not working

```bash
# Check if automated sync is enabled
kubectl -n argocd get application monitoring-dashboard-staging \
  -o jsonpath='{.spec.syncPolicy.automated}'
# Should show: {"allowEmpty":false,"prune":true,"selfHeal":true}

# Check self-heal interval (default: 5s)
kubectl -n argocd get configmap argocd-cm -o yaml | grep timeout.reconciliation

# Manually trigger sync
argocd app sync monitoring-dashboard-staging
```

## Test 3: Preview Environments

**Purpose**: Verify that preview environments are automatically created for pull requests.

### Steps

```bash
# 1. Verify ApplicationSet exists
kubectl -n argocd get applicationset preview-environments

# 2. Check current applications
kubectl -n argocd get applications | grep pr-
# Should show no preview apps initially

# 3. Create test branch and PR
git checkout -b test/preview-env-$(date +%s)
echo "# Preview environment test" >> README.md
git add README.md
git commit -m "test: preview environment"
git push origin HEAD

# 4. Create PR in GitHub UI
# https://github.com/YOUR_ORG/monitoring-dashboard/compare/main...test/preview-env-XXXXXX

# 5. Wait for ApplicationSet to detect PR (default: 60 seconds)
echo "Waiting for ApplicationSet to detect PR..."
sleep 70

# 6. Check if preview Application created
kubectl -n argocd get applications | grep pr-
# Should show: monitoring-dashboard-pr-XXX

# 7. Get PR number from GitHub
PR_NUMBER=<PR_NUMBER_FROM_GITHUB>

# 8. Verify preview namespace created
kubectl get namespace pr-${PR_NUMBER}

# 9. Check preview application sync status
kubectl -n argocd get application monitoring-dashboard-pr-${PR_NUMBER} \
  -o jsonpath='{.status.sync.status}'
# Expected: Synced

# 10. Verify preview pods running
kubectl -n pr-${PR_NUMBER} get pods

# 11. Check preview ingress
kubectl -n pr-${PR_NUMBER} get ingress
# Should show: pr-${PR_NUMBER}.preview.xyibank.ru

# 12. Test preview URL
curl -I https://pr-${PR_NUMBER}.preview.xyibank.ru
# Expected: HTTP 200

# 13. Close the PR in GitHub

# 14. Wait for ApplicationSet to detect PR closure (60 seconds)
sleep 70

# 15. Verify Application deleted
kubectl -n argocd get applications | grep pr-${PR_NUMBER}
# Should show: No resources found

# 16. Verify namespace deleted
kubectl get namespace pr-${PR_NUMBER}
# Should show: Error from server (NotFound)
```

### Expected Results

- ✅ ApplicationSet detects new PR within 60 seconds
- ✅ Preview Application created automatically
- ✅ Preview namespace created
- ✅ Preview pods running
- ✅ Preview ingress accessible
- ✅ Application deleted when PR closed
- ✅ Namespace cleaned up

### Troubleshooting

**Issue**: ApplicationSet not detecting PRs

```bash
# Check ApplicationSet status
kubectl -n argocd get applicationset preview-environments -o yaml

# Check ApplicationSet controller logs
kubectl -n argocd logs -l app.kubernetes.io/name=argocd-applicationset-controller --tail=50

# Verify GitHub token secret
kubectl -n argocd get secret github-token
kubectl -n argocd get secret github-token -o jsonpath='{.data.token}' | base64 -d | wc -c
# Should be > 0

# Test GitHub API access
TOKEN=$(kubectl -n argocd get secret github-token -o jsonpath='{.data.token}' | base64 -d)
curl -H "Authorization: token $TOKEN" \
  https://api.github.com/repos/YOUR_ORG/monitoring-dashboard/pulls
```

**Issue**: Preview environment not accessible

```bash
# Check DNS resolution
dig pr-${PR_NUMBER}.preview.xyibank.ru

# Check ingress
kubectl -n pr-${PR_NUMBER} describe ingress

# Check External DNS logs
kubectl -n kube-system logs -l app.kubernetes.io/name=external-dns
```

## Test 4: Rollback via Git

**Purpose**: Verify that rolling back a Git commit triggers automatic rollback of the deployment.

### Steps

```bash
# 1. Note current image tag
CURRENT_TAG=$(kubectl -n monitoring-dashboard-staging get deployment monitoring-dashboard-staging \
  -o jsonpath='{.spec.template.spec.containers[0].image}' | cut -d: -f2)
echo "Current tag: $CURRENT_TAG"

# 2. View recent image tag updates
git log --oneline -5 infra/argocd/applications/staging/monitoring-dashboard.yaml

# 3. Identify previous version commit
git log --oneline infra/argocd/applications/staging/monitoring-dashboard.yaml | head -3
# Select the second commit (previous version)

# 4. Get previous image tag
PREV_COMMIT=$(git log --oneline infra/argocd/applications/staging/monitoring-dashboard.yaml | head -2 | tail -1 | cut -d' ' -f1)
PREV_TAG=$(git show ${PREV_COMMIT}:infra/argocd/applications/staging/monitoring-dashboard.yaml | \
  yq eval '.spec.source.helm.parameters[] | select(.name == "image.tag").value')
echo "Previous tag: $PREV_TAG"

# 5. Revert to previous version
LATEST_COMMIT=$(git log --oneline infra/argocd/applications/staging/monitoring-dashboard.yaml | head -1 | cut -d' ' -f1)
git revert $LATEST_COMMIT --no-edit

# 6. Push rollback
git push origin main

# 7. Wait for ArgoCD to detect and sync
echo "Waiting for ArgoCD to sync rollback..."
watch -n 5 'kubectl -n argocd get application monitoring-dashboard-staging -o jsonpath="{.status.sync.status}"'

# 8. Verify pods rolling back
kubectl -n monitoring-dashboard-staging get pods -w

# 9. Verify old image tag restored
NEW_TAG=$(kubectl -n monitoring-dashboard-staging get deployment monitoring-dashboard-staging \
  -o jsonpath='{.spec.template.spec.containers[0].image}' | cut -d: -f2)
echo "Rolled back to: $NEW_TAG"
[ "$NEW_TAG" == "$PREV_TAG" ] && echo "✅ Rollback successful" || echo "❌ Rollback failed"

# 10. Verify application health
kubectl -n argocd get application monitoring-dashboard-staging \
  -o jsonpath='{.status.health.status}'
# Expected: Healthy
```

### Expected Results

- ✅ Git revert created successfully
- ✅ ArgoCD detected change
- ✅ Deployment rolled back to previous version
- ✅ Pods running with previous image
- ✅ Application health: Healthy
- ✅ Zero manual intervention

### Rollback Time Measurement

```bash
# Measure time from git push to pod ready
time (
  git revert HEAD --no-edit
  git push origin main
  echo "Git pushed, waiting for rollback..."
  until kubectl -n monitoring-dashboard-staging get pods -l app=monitoring-dashboard \
    -o jsonpath='{.items[0].status.containerStatuses[0].image}' | grep -q "$PREV_TAG"; do
    sleep 5
  done
  echo "Rollback complete"
)
```

### Troubleshooting

**Issue**: Rollback not triggering

```bash
# Force ArgoCD refresh
argocd app get monitoring-dashboard-staging --refresh

# Check sync status
argocd app get monitoring-dashboard-staging

# Manual sync if needed
argocd app sync monitoring-dashboard-staging
```

## Test 5: Manual Sync

**Purpose**: Verify manual sync capability (important for production).

### Steps

```bash
# 1. Disable auto-sync temporarily
kubectl -n argocd patch application monitoring-dashboard-staging --type=json \
  -p='[{"op": "remove", "path": "/spec/syncPolicy/automated"}]'

# 2. Make a change
echo "# Manual sync test" >> README.md
git add README.md
git commit -m "test: manual sync"
git push origin main

# Wait for GitHub Actions
sleep 120

# 3. Verify Application OutOfSync
kubectl -n argocd get application monitoring-dashboard-staging \
  -o jsonpath='{.status.sync.status}'
# Expected: OutOfSync

# 4. Manually trigger sync via CLI
argocd app sync monitoring-dashboard-staging

# 5. Wait for sync to complete
argocd app wait monitoring-dashboard-staging --health

# 6. Verify Synced status
kubectl -n argocd get application monitoring-dashboard-staging \
  -o jsonpath='{.status.sync.status}'
# Expected: Synced

# 7. Re-enable auto-sync
kubectl -n argocd patch application monitoring-dashboard-staging --type=json \
  -p='[{"op": "add", "path": "/spec/syncPolicy/automated", "value": {"prune": true, "selfHeal": true, "allowEmpty": false}}]'
```

### Expected Results

- ✅ Auto-sync can be disabled
- ✅ Application goes OutOfSync when changes pushed
- ✅ Manual sync works via CLI
- ✅ Application syncs successfully
- ✅ Auto-sync can be re-enabled

## Test 6: Sync Wave

**Purpose**: Verify controlled rollout and sync ordering (if using sync waves).

```bash
# Check if sync waves are configured
kubectl -n argocd get application monitoring-dashboard-staging -o yaml | grep "argocd.argoproj.io/sync-wave"

# If sync waves configured:
# - Verify resources sync in order
# - Verify dependencies respected
# - Check sync wave annotations on resources
```

## Validation Checklist

After deploying ArgoCD to staging, complete this checklist:

### ArgoCD Installation

- [ ] ArgoCD namespace created: `kubectl get ns argocd`
- [ ] ArgoCD pods running: `kubectl -n argocd get pods`
- [ ] ArgoCD UI accessible: https://argocd-staging.xyibank.ru
- [ ] ArgoCD admin login works
- [ ] ArgoCD ingress configured with ALB
- [ ] External DNS created DNS record

### Repository Configuration

- [ ] Repository credentials configured
- [ ] ArgoCD can access Git repository
- [ ] Repository connection status: Connected
- [ ] GitHub token secret exists for ApplicationSet

### Application Deployment

- [ ] `staging-apps` Application exists
- [ ] `monitoring-dashboard-staging` Application created
- [ ] Application status: Synced
- [ ] Application health: Healthy
- [ ] All resources created in namespace
- [ ] Pods running: `kubectl -n monitoring-dashboard-staging get pods`

### GitOps Workflow

- [ ] GitHub Actions workflow `gitops-update-staging.yml` exists
- [ ] Old workflow backed up as `_backup_deploy-staging.yml.disabled`
- [ ] Workflow has write permissions to repository
- [ ] Workflow successfully updates ArgoCD manifest
- [ ] Workflow commits are signed by github-actions[bot]

### Auto-Sync

- [ ] Test 1 passes: Auto-sync works
- [ ] ArgoCD detects changes within 3 minutes
- [ ] New deployments roll out automatically
- [ ] Application returns to Synced status

### Self-Heal

- [ ] Test 2 passes: Self-heal works
- [ ] Manual changes are reverted within 60 seconds
- [ ] Replicas restored to Git value
- [ ] Application stability maintained

### Preview Environments

- [ ] ApplicationSet deployed
- [ ] Test 3 passes: Preview envs work
- [ ] PR creates preview application
- [ ] Preview namespace created
- [ ] Preview ingress accessible
- [ ] Closing PR deletes preview resources

### Rollback

- [ ] Test 4 passes: Git rollback works
- [ ] Revert commit triggers rollback
- [ ] Previous version restored
- [ ] Rollback completes within acceptable time

### Monitoring & Observability

- [ ] ArgoCD metrics available
- [ ] Application sync status visible
- [ ] Application health status tracked
- [ ] Git commit history visible in UI
- [ ] Sync history available

### Documentation

- [ ] `infra/argocd/README.md` created
- [ ] `TESTING_GITOPS.md` available
- [ ] Team trained on GitOps workflow
- [ ] Runbook for troubleshooting created

## Performance Testing

### Sync Time Measurement

```bash
# Measure time from git push to deployment ready
./scripts/measure-gitops-sync-time.sh
```

Create `scripts/measure-gitops-sync-time.sh`:

```bash
#!/bin/bash
set -e

START_TIME=$(date +%s)

# Make change
echo "# Perf test $(date)" >> README.md
git add README.md
git commit -m "perf: measure sync time"
COMMIT_SHA=$(git rev-parse HEAD)
git push origin main

echo "Waiting for GitHub Actions..."
# Wait for Actions
sleep 120

echo "Waiting for ArgoCD sync..."
# Wait for new image tag to appear
until kubectl -n monitoring-dashboard-staging get deployment monitoring-dashboard-staging \
  -o jsonpath='{.spec.template.spec.containers[0].image}' | grep -q "${COMMIT_SHA:0:7}"; do
  sleep 5
done

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo "✅ GitOps sync completed in ${DURATION} seconds"
echo "   Git push → Deployment ready: ${DURATION}s"
```

**Target Metrics:**
- Git push to ArgoCD sync: < 3 minutes
- ArgoCD sync to pods ready: < 2 minutes
- Total time (push to ready): < 5 minutes

## Disaster Recovery Testing

### Test 1: ArgoCD Failure Recovery

```bash
# Simulate ArgoCD failure
kubectl -n argocd delete pod -l app.kubernetes.io/name=argocd-server

# Verify auto-recovery
kubectl -n argocd get pods -w

# Verify applications still work
kubectl -n argocd get applications
```

### Test 2: Git Repository Unavailable

```bash
# Simulate by temporarily revoking GitHub token
# (Don't actually do this in production!)

# Verify ArgoCD shows connection error
argocd repo list

# Restore token and verify recovery
```

### Test 3: Full ArgoCD Reinstall

```bash
# Delete ArgoCD (CAUTION: Test only!)
kubectl delete namespace argocd

# Reinstall via Terraform
cd infra/terraform/live/staging
terraform apply -target=module.argocd

# Reapply Applications
kubectl apply -f infra/argocd/app-of-apps/staging.yaml

# Verify all resources recreated
```

## Continuous Testing

Add automated tests to CI/CD:

```yaml
# .github/workflows/test-gitops.yml
name: Test GitOps
on:
  schedule:
    - cron: '0 */6 * * *'  # Every 6 hours
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Check ArgoCD health
        run: |
          argocd app get monitoring-dashboard-staging --health
      - name: Check sync status
        run: |
          STATUS=$(argocd app get monitoring-dashboard-staging -o json | jq -r '.status.sync.status')
          [ "$STATUS" == "Synced" ] || exit 1
```

## Troubleshooting Common Issues

### Issue: Application Stuck in Progressing

```bash
# Check events
kubectl -n argocd describe application monitoring-dashboard-staging

# Check pods
kubectl -n monitoring-dashboard-staging get pods
kubectl -n monitoring-dashboard-staging describe pod <pod-name>

# Check logs
kubectl -n monitoring-dashboard-staging logs <pod-name>
```

### Issue: Sync Failing

```bash
# View detailed error
argocd app get monitoring-dashboard-staging

# Dry run
argocd app sync monitoring-dashboard-staging --dry-run

# Force sync
argocd app sync monitoring-dashboard-staging --force
```

### Issue: Self-Heal Not Working

```bash
# Check sync policy
kubectl -n argocd get application monitoring-dashboard-staging \
  -o jsonpath='{.spec.syncPolicy.automated}'

# Check controller logs
kubectl -n argocd logs -l app.kubernetes.io/name=argocd-application-controller
```

## Summary

After completing all tests, you should have confidence that:

1. ✅ ArgoCD is properly configured and operational
2. ✅ GitOps workflow successfully replaces push-based deployment
3. ✅ Auto-sync keeps cluster in sync with Git
4. ✅ Self-heal prevents configuration drift
5. ✅ Preview environments work automatically
6. ✅ Rollback via Git is reliable
7. ✅ System is resilient to failures

**Next Steps:**
- Run tests weekly to ensure continued operation
- Monitor ArgoCD metrics
- Train team on GitOps workflow
- Plan production rollout after 1-2 weeks of stable staging operation
