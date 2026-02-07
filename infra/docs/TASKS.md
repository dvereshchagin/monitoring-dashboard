# Implementation Decomposition

## Phase 1 - Repo scaffolding

- [x] Add Terraform module structure (`network`, `eks`, `rds`, `ecr`)
- [x] Add Terraform live environments (`staging`, `prod`)
- [x] Add Terraform backend bootstrap (`S3 + DynamoDB`)
- [x] Add Helm chart with staging/prod/preview values
- [x] Add Kubernetes bootstrap manifest for External Secrets
- [x] Add preview namespace preparation script

## Phase 2 - CI/CD and branch contour

- [x] Add PR CI workflow
- [x] Add preview deploy workflow for feature branches
- [x] Add manual preview cleanup workflow
- [x] Add staging deploy workflow from `main`
- [x] Add manual production deploy workflow
- [x] Disable old legacy docker-compose workflow (manual notice only)

## Phase 3 - App readiness for Kubernetes

- [x] Add `/healthz` and `/readyz` endpoints for probes
- [ ] Add integration tests for health endpoints
- [ ] Add DB readiness check for `/readyz` if strict readiness is required

## Phase 4 - Environment hardening (manual follow-up)

- [ ] Create AWS accounts/roles and OIDC trust for GitHub
- [ ] Install ALB controller in each cluster
- [ ] Install External Secrets Operator in each cluster
- [ ] Create AWS Secrets Manager secrets (`/monitoring-dashboard/staging/app`, `/monitoring-dashboard/prod/app`)
- [ ] Set GitHub Environment variables and approvals
- [ ] Configure Route53 + ACM + ALB DNS records

## Phase 5 - Operational readiness

- [ ] Add DB migration strategy in deploy workflow (job or controlled step)
- [ ] Add smoke tests post-deploy (`/healthz`, `/readyz`, `/api/v1/metrics/history`)
- [ ] Add rollback runbook and automation checks
- [ ] Add cost guardrails for preview environments
