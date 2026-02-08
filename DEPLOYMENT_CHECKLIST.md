# Deployment Checklist - Performance Optimizations

**Commit:** `0a25111` - feat: add performance optimizations for 100+ RPS support
**Date:** 2026-02-08
**Target:** staging.xyibank.ru → production

---

## 📋 Pre-Deployment Checklist

### 1. Code Review & Testing
- [x] Code committed and pushed to main
- [x] Application builds successfully (`go build`)
- [ ] Unit tests pass (`go test ./...`)
- [ ] Load tests show expected improvements
- [ ] Documentation complete (3 guides created)

### 2. Database Migration Ready
- [x] Migration 003 created (`003_covering_indexes_optimization.sql`)
- [ ] Migration tested on local database
- [ ] Migration backup plan prepared
- [ ] Rollback script ready

### 3. Infrastructure Prerequisites
- [ ] Redis instance available (ElastiCache or Docker)
- [ ] Database has enough storage for indexes
- [ ] Connection pool limits increased in RDS/PostgreSQL
- [ ] Monitoring/alerting configured

### 4. Configuration Files
- [ ] `.env` updated with Redis config
- [ ] Helm values updated (if using Kubernetes)
- [ ] Environment variables documented
- [ ] Secrets properly configured

---

## 🚀 Staging Deployment Steps

### Step 1: Backup Current State (5 min)

```bash
# 1. Backup database
pg_dump -h staging-db-host -U postgres -d monitoring > backup_$(date +%Y%m%d_%H%M%S).sql

# 2. Save current pod logs (if K8s)
kubectl logs -n staging deployment/monitoring-dashboard > logs_pre_deploy.txt

# 3. Note current metrics
curl https://staging.xyibank.ru/api/v1/auth/status
```

### Step 2: Apply Database Migration (10 min)

```bash
# Connect to staging database
psql -h <staging-db-host> -U postgres -d monitoring

# Check current indexes
\d+ metrics

# Apply migration
\i monitoring-dashboard-api/internal/infrastructure/persistence/postgres/migrations/003_covering_indexes_optimization.sql

# Verify indexes created
SELECT indexname, indexdef
FROM pg_indexes
WHERE tablename = 'metrics'
ORDER BY indexname;

# Expected output:
# - idx_metrics_type_time_covering
# - idx_metrics_type_time_id
# - idx_metrics_recent
# - metrics_hourly (materialized view)

# Check index sizes
SELECT
    schemaname,
    tablename,
    indexname,
    pg_size_pretty(pg_relation_size(indexname::regclass)) as index_size
FROM pg_indexes
WHERE tablename = 'metrics';
```

**⏱ Expected time:** 2-5 minutes depending on table size
**⚠️ Note:** Indexes created with CONCURRENTLY to avoid locking

### Step 3: Deploy Redis Cache (5 min)

**Option A: Docker (for staging)**
```bash
# SSH to staging server
ssh staging.xyibank.ru

# Start Redis container
docker run -d \
  --name redis-cache \
  --restart unless-stopped \
  -p 6379:6379 \
  redis:7-alpine \
  redis-server \
    --maxmemory 1gb \
    --maxmemory-policy allkeys-lru \
    --save "" \
    --appendonly no

# Verify Redis is running
docker ps | grep redis-cache
redis-cli ping  # Should return PONG
```

**Option B: AWS ElastiCache (for production)**
```bash
# Use existing ElastiCache cluster or create new:
# Instance type: cache.t3.small (for staging) or cache.r6g.large (for prod)
# Engine: Redis 7.x
# Node count: 1 (staging) or 2+ with replication (prod)

# Get endpoint
aws elasticache describe-cache-clusters \
  --cache-cluster-id monitoring-redis-staging \
  --show-cache-node-info \
  --query 'CacheClusters[0].CacheNodes[0].Endpoint' \
  --output text
```

### Step 4: Update Environment Variables (2 min)

Create/update `.env` on staging server:

```bash
# Database Connection Pool
DB_MAX_OPEN_CONNS=100
DB_MAX_IDLE_CONNS=50

# Redis Cache
REDIS_ENABLED=true
REDIS_HOST=localhost              # or ElastiCache endpoint
REDIS_PORT=6379
REDIS_PASSWORD=                    # empty for local, set for ElastiCache
REDIS_DB=0
REDIS_CACHE_TTL=60s
REDIS_POOL_SIZE=100
REDIS_MIN_IDLE_CONNS=20
REDIS_DIAL_TIMEOUT=5s
REDIS_READ_TIMEOUT=3s
REDIS_WRITE_TIMEOUT=3s

# Server Timeouts (already in config.go, but can override)
SERVER_READ_TIMEOUT=30s
SERVER_WRITE_TIMEOUT=30s
SERVER_IDLE_TIMEOUT=120s
```

### Step 5: Build and Deploy Application (10 min)

**Option A: Direct deployment**
```bash
# On staging server
cd /app/monitoring-dashboard-api

# Pull latest code
git pull origin main

# Build
go build -o bin/monitoring-dashboard-api ./cmd/monitoring-dashboard-api

# Stop current process
sudo systemctl stop monitoring-dashboard

# Start new version
sudo systemctl start monitoring-dashboard

# Check status
sudo systemctl status monitoring-dashboard
```

**Option B: Docker deployment**
```bash
# Build Docker image
docker build -t monitoring-dashboard-api:v2.0.0-perf .

# Stop old container
docker stop monitoring-dashboard-api
docker rm monitoring-dashboard-api

# Start new container
docker run -d \
  --name monitoring-dashboard-api \
  --restart unless-stopped \
  -p 8080:8080 \
  --env-file .env \
  --link redis-cache:redis \
  monitoring-dashboard-api:v2.0.0-perf

# Check logs
docker logs -f monitoring-dashboard-api
```

**Option C: Kubernetes deployment**
```bash
# Update Helm values
helm upgrade monitoring-dashboard ./infra/helm/monitoring-dashboard \
  --namespace staging \
  --set image.tag=v2.0.0-perf \
  --set redis.enabled=true \
  --set redis.host=redis-cache \
  --set database.maxOpenConns=100 \
  --set database.maxIdleConns=50

# Watch rollout
kubectl rollout status deployment/monitoring-dashboard -n staging

# Check pods
kubectl get pods -n staging
```

### Step 6: Verify Deployment (5 min)

```bash
# 1. Health check
curl https://staging.xyibank.ru/api/v1/auth/status
# Expected: {"auth_enabled":false,"authenticated":true}

# 2. Check application logs
# Look for Redis initialization message
tail -f /var/log/monitoring-dashboard/app.log | grep -i redis
# Expected: "Redis cache initialized successfully"

# 3. Test metrics endpoint
curl https://staging.xyibank.ru/api/v1/metrics/history?type=cpu\&duration=1h
# Should return quickly (<500ms)

# 4. Verify Redis cache working
redis-cli
> KEYS metrics:*
# Should show cached keys after first request

> INFO stats
# Check keyspace_hits / keyspace_misses ratio

# 5. Check database connections
psql -h <db-host> -U postgres -d monitoring -c \
  "SELECT count(*) FROM pg_stat_activity WHERE datname = 'monitoring';"
# Should be < 100
```

### Step 7: Performance Testing (15 min)

```bash
cd monitoring-dashboard-api

# Smoke test (light load)
./scripts/load/run_k6.sh smoke ./scripts/load/config/staging-external.json

# Expected results:
# ✅ P95 < 500ms
# ✅ 0% errors
# ✅ ~2-5 RPS handled smoothly

# Step test (find max capacity)
./scripts/load/run_k6.sh step ./scripts/load/config/staging-external.json

# Expected results:
# ✅ Handles 100+ RPS
# ✅ P95 < 800ms at 100 RPS
# ✅ <1% error rate
```

### Step 8: Monitor for Issues (30 min)

**Watch these metrics:**

1. **Application Logs**
```bash
tail -f /var/log/monitoring-dashboard/app.log | grep -E "(ERROR|WARN|Redis|Database)"
```

2. **Redis Stats**
```bash
watch -n 5 'redis-cli INFO stats | grep -E "(hits|misses|keys|memory)"'
```

3. **Database Connections**
```bash
watch -n 5 'psql -h <db-host> -U postgres -d monitoring -c "SELECT count(*), state FROM pg_stat_activity WHERE datname = '\''monitoring'\'' GROUP BY state;"'
```

4. **Response Times**
```bash
# Simple monitoring script
while true; do
  curl -w "\n%{time_total}s\n" -o /dev/null -s https://staging.xyibank.ru/api/v1/metrics/history?type=cpu\&duration=1h
  sleep 5
done
```

---

## 🔄 Rollback Plan

If issues occur, rollback immediately:

### Quick Rollback (5 min)

```bash
# 1. Revert to previous version
git revert 0a25111
git push origin main

# 2. Redeploy old version
# (Use same deployment method as Step 5)

# 3. Disable Redis (if causing issues)
# Update .env:
REDIS_ENABLED=false

# Restart application
sudo systemctl restart monitoring-dashboard

# 4. Monitor for stability
# Check logs, metrics, response times
```

### Full Rollback (15 min)

```bash
# 1. Drop new indexes (if causing issues)
psql -h <db-host> -U postgres -d monitoring <<EOF
DROP MATERIALIZED VIEW IF EXISTS metrics_hourly;
DROP INDEX IF EXISTS idx_metrics_recent;
DROP INDEX IF EXISTS idx_metrics_type_time_id;
DROP INDEX IF EXISTS idx_metrics_type_time_covering;
EOF

# 2. Restore from backup
psql -h <db-host> -U postgres -d monitoring < backup_YYYYMMDD_HHMMSS.sql

# 3. Revert code and redeploy

# 4. Stop Redis
docker stop redis-cache
docker rm redis-cache
```

---

## 📊 Success Metrics

### Immediate (0-1 hour)
- [ ] Application starts without errors
- [ ] Health check returns 200 OK
- [ ] Redis connection successful
- [ ] Database migration completed
- [ ] No 5xx errors in logs

### Short-term (1-24 hours)
- [ ] P95 response time < 500ms
- [ ] Redis cache hit rate > 70%
- [ ] Error rate < 1%
- [ ] Database connections < 100
- [ ] Load test passes at 100 RPS

### Long-term (1-7 days)
- [ ] System stable under production load
- [ ] No memory leaks
- [ ] No connection pool exhaustion
- [ ] Cache invalidation working correctly
- [ ] Monitoring alerts not triggered

---

## 🐛 Common Issues & Solutions

### Issue 1: Redis Connection Failed
**Symptoms:** Log message "Failed to connect to Redis"

**Solution:**
```bash
# Check Redis is running
docker ps | grep redis-cache

# Check Redis logs
docker logs redis-cache

# Test connection
redis-cli ping

# Temporary fix: Disable Redis
REDIS_ENABLED=false
```

### Issue 2: Database Migration Too Slow
**Symptoms:** Migration taking > 10 minutes

**Solution:**
```bash
# Cancel current migration
# Press Ctrl+C in psql

# Check table size
SELECT pg_size_pretty(pg_total_relation_size('metrics'));

# If > 10GB, create indexes CONCURRENTLY (already in migration)
# But may need to increase maintenance_work_mem:
ALTER SYSTEM SET maintenance_work_mem = '1GB';
SELECT pg_reload_conf();

# Retry migration
```

### Issue 3: Too Many Database Connections
**Symptoms:** Error "too many connections for database"

**Solution:**
```bash
# Check current connections
SELECT count(*) FROM pg_stat_activity WHERE datname = 'monitoring';

# Kill idle connections
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE state = 'idle'
  AND state_change < NOW() - INTERVAL '5 minutes';

# Temporarily reduce MaxOpenConns
DB_MAX_OPEN_CONNS=50  # instead of 100
```

### Issue 4: High Memory Usage
**Symptoms:** OOM errors, high memory usage

**Solution:**
```bash
# Check memory usage
free -h
docker stats

# Reduce Redis maxmemory
redis-cli CONFIG SET maxmemory 512mb

# Reduce connection pools
DB_MAX_OPEN_CONNS=50
REDIS_POOL_SIZE=50
```

---

## 📞 Emergency Contacts

**On-Call Engineer:** [Your Name]
**Database Admin:** [DBA Name]
**DevOps:** [DevOps Name]

**Escalation:**
1. Check #monitoring-alerts Slack channel
2. Review logs in CloudWatch/Grafana
3. If critical, page on-call immediately

---

## ✅ Post-Deployment Tasks

- [ ] Update runbook with new procedures
- [ ] Document Redis maintenance tasks
- [ ] Set up CloudWatch alarms for new metrics
- [ ] Update team on changes in daily standup
- [ ] Schedule load test on production (gradually)
- [ ] Create postmortem if issues occurred

---

**Deployment Status:** ⏳ PENDING
**Started:** _____
**Completed:** _____
**Deployed By:** _____
**Notes:**

---

*This checklist ensures safe, repeatable deployments with minimal downtime.*
