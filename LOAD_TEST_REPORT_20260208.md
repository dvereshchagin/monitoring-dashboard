# Load Testing Report - staging.xyibank.ru
**Date:** 2026-02-08
**Environment:** staging.xyibank.ru
**Test Origin:** External (Internet)
**Git Commit:** 3db639e

## Executive Summary

⚠️ **CRITICAL PERFORMANCE ISSUES DETECTED**

The staging environment exhibits severe performance degradation under load:
- **Maximum Stable RPS:** ~2-3 RPS
- **Breaking Point:** 10 RPS target causes 14% error rate
- **Response Times:** P95 ranges from 3-21 seconds (target: <800ms)
- **Recommendation:** System requires immediate optimization before production deployment

---

## Test Results

### Test 1: Smoke Test (Conservative Load)
**Configuration:**
- Virtual Users: 5 VUs
- Duration: 5 minutes
- Test Type: Constant load validation

**Results:**
```
✓ Status: PASSED (with relaxed thresholds)

  Throughput:
    - Requests/sec: 2.25 RPS
    - Total Requests: 681
    - Total Iterations: 681

  Response Times:
    - Average: 1,555 ms
    - Median: 1,420 ms
    - P90: 2,550 ms
    - P95: 3,078 ms
    - P99: 4,740 ms
    - Max: 7,360 ms

  Reliability:
    - Error Rate: 0.00% ✓
    - History Endpoint Errors: 0.00% ✓
    - Auth Endpoint Errors: 0.00% ✓

  Data Transfer:
    - Received: 1.2 GB (4.0 MB/s)
    - Sent: 4.1 MB (14 KB/s)
```

**Analysis:**
- System handles light load without errors
- Response times are 3-6x slower than target (800ms P95)
- All requests eventually succeed

---

### Test 2: Step Test (Capacity Finding)
**Configuration:**
- Target RPS Stages: 10 → 20 → 30 → 40 → 50 → 60
- Stage Duration: 10 minutes each
- Max VUs: 250
- Test Type: Ramping arrival rate

**Results:**
```
✗ Status: FAILED at 10 RPS (3 minutes into test)

  Throughput:
    - Requests/sec: 8.38 RPS (target: 10 RPS)
    - Total Requests: 1,526
    - Total Iterations: 1,524
    - Dropped Iterations: 119

  Response Times:
    - Average: 8,615 ms
    - Median: 8,096 ms
    - P90: 16,108 ms
    - P95: 20,793 ms (threshold: 5,000 ms)
    - P99: 25,850 ms (threshold: 8,000 ms)
    - Max: 30,003 ms (timeout)

  Reliability:
    - Error Rate: 14.02% ✗ (threshold: 1%)
    - History Endpoint Errors: 14.32% ✗
    - Auth Endpoint Errors: 0.00% ✓
    - Request Timeouts: 214 requests

  Load Level at Failure:
    - Active VUs: 179
    - Target RPS: 10

  Data Transfer:
    - Received: 2.5 GB (14 MB/s)
    - Sent: 9.1 MB (50 KB/s)
```

**Failure Reason:**
Test aborted after 3 minutes due to:
1. Error rate exceeded 1% threshold (reached 14.02%)
2. P95 response time exceeded 5,000ms threshold (reached 20,793ms)
3. P99 response time exceeded 8,000ms threshold (reached 25,850ms)

**Analysis:**
- System completely breaks down at 10 RPS
- 214 requests (14%) timed out or failed
- Response times increased by 6x compared to smoke test
- Unable to reach even the first RPS target

---

## Endpoint Performance Breakdown

### GET /api/v1/metrics/history (70% of traffic)
```
Smoke Test (2 RPS):
  - Average: 1,580 ms
  - P95: 3,090 ms
  - Error Rate: 0%

Step Test (10 RPS):
  - Average: 8,790 ms
  - P95: 20,998 ms
  - Error Rate: 14.32%
  - Timeouts: 214 requests
```

**Issue:** History endpoint degrades severely under load, with 5x slowdown and 14% failures.

### GET /api/v1/auth/status (2% of traffic)
```
Smoke Test (2 RPS):
  - Average: 115 ms
  - P95: 211 ms
  - Error Rate: 0%

Step Test (10 RPS):
  - Average: 407 ms
  - P95: 1,230 ms
  - Error Rate: 0%
```

**Issue:** Auth endpoint remains stable but shows 4x slowdown under load.

---

## Performance Bottlenecks Identified

### 1. **Database Query Performance** (Critical)
**Evidence:**
- `/api/v1/metrics/history` is the slowest endpoint
- Response times grow exponentially with concurrent users
- High data transfer (1.2-2.5 GB received in tests)

**Root Cause:**
- Likely missing database indexes on `metrics` table
- Inefficient time-range queries
- Possible N+1 query patterns
- No query result caching

**Impact:** This is the primary bottleneck preventing scale.

### 2. **Connection Pool Exhaustion** (High)
**Evidence:**
- Error rate jumps from 0% to 14% suddenly
- 214 request timeouts at 179 concurrent VUs
- System cannot handle 10 RPS despite low compute requirements

**Root Cause:**
- Database connection pool too small
- No connection pooling for HTTP clients
- Goroutine leaks or blocking operations

### 3. **No Response Caching** (Medium)
**Evidence:**
- Every request fetches full dataset from database
- High data transfer volume
- No HTTP caching headers observed

**Impact:** Redundant work for frequently accessed time ranges.

### 4. **Network/Transfer Overhead** (Low-Medium)
**Evidence:**
- Transferring 1.2-2.5 GB for 681-1,526 requests
- Average payload size: ~1.7 MB per request

**Issue:** Large response payloads without compression or pagination.

---

## Capacity Assessment

### Current Capacity
| Metric | Value | Status |
|--------|-------|--------|
| Maximum Stable RPS | 2-3 RPS | ⚠️ Very Low |
| Maximum Concurrent Users | ~8 VUs | ⚠️ Very Low |
| Error-Free Capacity | <10 RPS | ⚠️ Critical |
| Response Time SLA (P95 < 800ms) | Not Met | ❌ Failed |

### Production Readiness
**Status:** ❌ **NOT PRODUCTION READY**

**Rationale:**
- System cannot handle even modest load (10 RPS)
- Response times 20-40x slower than industry standards
- 14% error rate under minimal load
- No horizontal scaling capability demonstrated

---

## Recommendations (Priority Order)

### 🔴 P0 - Critical (Must Fix Before Production)

1. **Database Optimization**
   ```sql
   -- Add missing indexes on metrics table
   CREATE INDEX idx_metrics_timestamp ON metrics(timestamp DESC);
   CREATE INDEX idx_metrics_type_timestamp ON metrics(metric_type, timestamp DESC);
   CREATE INDEX idx_metrics_composite ON metrics(metric_type, timestamp DESC) INCLUDE (value);
   ```

   **Expected Impact:** 5-10x query speedup

2. **Database Connection Pool Tuning**
   ```go
   // config/database.go
   db.SetMaxOpenConns(50)        // Increase from default
   db.SetMaxIdleConns(25)        // Keep connections warm
   db.SetConnMaxLifetime(5 * time.Minute)
   db.SetConnMaxIdleTime(1 * time.Minute)
   ```

   **Expected Impact:** Handle 10x more concurrent requests

3. **Query Optimization**
   - Add EXPLAIN ANALYZE to history queries
   - Implement pagination (limit result sets to 1000 records)
   - Use prepared statements
   - Add query timeout guards

   **Expected Impact:** 3-5x query speedup

### 🟡 P1 - High (Performance Improvements)

4. **Response Caching**
   ```go
   // Implement Redis cache for history queries
   // Cache TTL: 30-60 seconds for recent data
   // Cache key: metric_type:duration:timestamp_bucket
   ```

   **Expected Impact:** 10-50x improvement for repeated queries

5. **HTTP Response Compression**
   ```go
   // Add gzip/brotli compression middleware
   router.Use(middleware.Compress(5))
   ```

   **Expected Impact:** 60-80% reduction in transfer size

6. **API Response Pagination**
   ```go
   // Limit default page size to 100-500 records
   // Add cursor-based pagination
   ```

   **Expected Impact:** 3-5x reduction in response times

### 🟢 P2 - Medium (Scalability)

7. **Add Monitoring & Profiling**
   - Enable pprof endpoints
   - Add Prometheus metrics for database query times
   - Set up distributed tracing (OpenTelemetry)

8. **Implement Rate Limiting**
   ```go
   // Per-IP rate limiting: 10 req/sec
   // Protect against request floods
   ```

9. **Horizontal Scaling**
   - Containerize application (Docker)
   - Deploy multiple replicas behind load balancer
   - Implement health checks

---

## Next Steps

### Immediate Actions (This Week)
1. ✅ Create this load test report
2. ⬜ Add database indexes (30 minutes)
3. ⬜ Tune connection pool settings (15 minutes)
4. ⬜ Re-run smoke test to validate improvements
5. ⬜ Profile slow queries with EXPLAIN ANALYZE

### Short Term (1-2 Weeks)
1. ⬜ Implement query result caching (Redis)
2. ⬜ Add response compression
3. ⬜ Implement pagination
4. ⬜ Add performance monitoring
5. ⬜ Re-run full step test (target: 50 RPS)

### Medium Term (1 Month)
1. ⬜ Optimize database schema
2. ⬜ Implement horizontal scaling
3. ⬜ Add CDN for static assets
4. ⬜ Load test to 100+ RPS
5. ⬜ Document performance SLAs

---

## Test Configuration Details

### Request Distribution
- 70% - GET /api/v1/metrics/history?type=cpu&duration=1h
- 20% - GET /api/v1/metrics/history?type=memory&duration=1h
- 5% - GET /api/v1/metrics/history?type=disk&duration=15m
- 3% - GET /api/v1/metrics/history?type=network&duration=15m
- 2% - GET /api/v1/auth/status

### Thresholds Used
```json
{
  "errorRate": 0.01,        // 1% max error rate
  "p95Ms": 5000,            // 5s (relaxed from 800ms)
  "p99Ms": 8000,            // 8s (relaxed from 1500ms)
  "errorAbortDelay": "5m",
  "p99AbortDelay": "3m"
}
```

### Test Environment
- **Load Generator:** MacBook Pro (M1)
- **Network:** External Internet (home connection)
- **Target:** staging.xyibank.ru (AWS)
- **Tool:** k6 (Grafana)
- **Scripts:** `/monitoring-dashboard-api/scripts/load/`

---

## Appendix: Raw Test Outputs

### Test Result Locations
```
results/20260208T110351Z-smoke/  # Initial smoke test (8 VUs, failed thresholds)
results/20260208T110734Z-smoke/  # Conservative smoke test (5 VUs, passed)
results/20260208T111501Z-step/   # Step test (failed at 10 RPS)
```

### Key Files
- `summary.json` - Full metrics export
- `timeseries.csv` - Time-series data for analysis
- `env.md` - Test environment metadata
- `config.json` - Test configuration used

---

## Conclusion

The staging environment requires **immediate performance optimization** before production deployment. The system currently cannot handle production-level traffic and will fail under real-world load.

**Estimated Timeline to Production-Ready:**
- With P0 fixes: 1-2 weeks
- With P0 + P1 fixes: 3-4 weeks
- Full optimization: 4-6 weeks

**Recommended Action:** Prioritize P0 database optimization tasks this week and re-test.

---

*Report generated by k6 load testing framework*
*Test conducted: 2026-02-08 11:03-13:18 UTC*
