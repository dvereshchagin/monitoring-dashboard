# Performance Optimizations for 100+ RPS

Этот документ описывает все оптимизации для поддержки 100+ RPS на staging/production окружении.

## ✅ Реализованные оптимизации

### 1. Database Connection Pool (P0 - CRITICAL)
**Файл:** `pkg/config/config.go`

**Изменения:**
- MaxOpenConns: 25 → 100
- MaxIdleConns: 5 → 50
- ConnMaxIdleTime: 10m → 2m
- ReadTimeout: 10s → 30s
- WriteTimeout: 10s → 30s

**Эффект:** 4x рост capacity, обработка до 100 одновременных соединений

---

### 2. Query Pagination (P0 - CRITICAL)
**Файл:** `internal/infrastructure/persistence/postgres/metric_repository_impl.go`

**Изменения:**
- Добавлен LIMIT 5000 в FindByTimeRange()
- Предотвращает большие выборки
- Защита от OOM

**Эффект:** Постоянное время ответа, независимо от размера таблицы

---

### 3. Covering Indexes (P0 - CRITICAL)
**Файл:** `migrations/003_covering_indexes_optimization.sql`

**Новые индексы:**
```sql
-- Covering index - включает value, unit, metadata
CREATE INDEX idx_metrics_type_time_covering
    ON metrics(metric_type, collected_at DESC)
    INCLUDE (metric_name, value, unit, metadata);

-- Partial index для свежих данных (7 дней)
CREATE INDEX idx_metrics_recent
    ON metrics(metric_type, collected_at DESC)
    WHERE collected_at > NOW() - INTERVAL '7 days';

-- Materialized view для почасовой агрегации
CREATE MATERIALIZED VIEW metrics_hourly AS ...
```

**Эффект:** 5-10x ускорение запросов истории

---

### 4. Redis Caching Layer (P1 - HIGH)
**Файлы:**
- `internal/infrastructure/cache/redis/redis_cache.go`
- `internal/application/port/cache.go`
- `internal/application/usecase/get_historical_metrics_cached.go`

**Функциональность:**
- Кеширование результатов /api/v1/metrics/history
- TTL: 60 секунд (настраивается)
- Автоматическая инвалидация
- Connection pool: 100 соединений

**Эффект:** 10-50x ускорение для повторных запросов, снижение нагрузки на БД на 80%

---

### 5. HTTP Compression (P1 - HIGH)
**Файл:** `internal/interfaces/http/middleware/compression.go`

**Функциональность:**
- Gzip compression level 5
- Pool для переиспользования компрессоров
- Автоматическое определение content-type
- Минимальный размер: 1KB

**Эффект:** 60-80% снижение размера ответов, экономия bandwidth

---

### 6. Rate Limiting (P1 - HIGH)
**Файл:** `internal/interfaces/http/middleware/rate_limiter.go`

**Функциональность:**
- Per-IP rate limiting
- Лимит: 100 req/sec на IP
- Burst: 200 запросов
- Автоматическая очистка старых лимитеров

**Эффект:** Защита от DDoS, стабильная работа под нагрузкой

---

## 📋 Шаги по внедрению

### Шаг 1: Обновить зависимости

```bash
cd monitoring-dashboard-api

# Добавить Redis client
go get github.com/redis/go-redis/v9

# Добавить rate limiting
go get golang.org/x/time/rate

# Обновить зависимости
go mod tidy
```

### Шаг 2: Применить миграции БД

```bash
# Применить новую миграцию с covering indexes
goose -dir internal/infrastructure/persistence/postgres/migrations postgres "your-dsn" up

# Или вручную:
psql -U postgres -d monitoring -f internal/infrastructure/persistence/postgres/migrations/003_covering_indexes_optimization.sql

# Проверить индексы
psql -U postgres -d monitoring -c "\d+ metrics"
```

### Шаг 3: Настроить Redis

**Вариант A: Docker (для разработки)**
```bash
docker run -d \
  --name redis-cache \
  -p 6379:6379 \
  redis:7-alpine \
  redis-server --maxmemory 512mb --maxmemory-policy allkeys-lru
```

**Вариант B: Production (AWS ElastiCache / Managed Redis)**
```bash
# Настроить в infra/terraform/modules/elasticache/
# Или использовать существующий Redis кластер
```

### Шаг 4: Обновить environment variables

**Staging (.env.staging):**
```bash
# Database Connection Pool
DB_MAX_OPEN_CONNS=100
DB_MAX_IDLE_CONNS=50

# Redis Cache
REDIS_ENABLED=true
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_CACHE_TTL=60s
REDIS_POOL_SIZE=100
REDIS_MIN_IDLE_CONNS=20

# Server Timeouts
SERVER_READ_TIMEOUT=30s
SERVER_WRITE_TIMEOUT=30s
SERVER_IDLE_TIMEOUT=120s
```

**Production (.env.production):**
```bash
# Database Connection Pool (выше для production)
DB_MAX_OPEN_CONNS=200
DB_MAX_IDLE_CONNS=100

# Redis Cache (используйте managed Redis)
REDIS_ENABLED=true
REDIS_HOST=your-elasticache-endpoint.amazonaws.com
REDIS_PORT=6379
REDIS_PASSWORD=your-redis-password
REDIS_DB=0
REDIS_CACHE_TTL=60s
REDIS_POOL_SIZE=200
REDIS_MIN_IDLE_CONNS=50

# Server Timeouts
SERVER_READ_TIMEOUT=30s
SERVER_WRITE_TIMEOUT=30s
SERVER_IDLE_TIMEOUT=120s
```

### Шаг 5: Обновить main.go для интеграции компонентов

**Добавить в cmd/monitoring-dashboard-api/main.go:**

```go
import (
	"github.com/dreschagin/monitoring-dashboard/internal/infrastructure/cache/redis"
	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/http/middleware"
)

func main() {
	// ... существующий код ...

	// Initialize Redis cache (опционально)
	var cache port.Cache
	if cfg.Redis.Enabled {
		redisCache, err := redis.NewRedisCache(
			cfg.Redis.Host,
			cfg.Redis.Port,
			cfg.Redis.Password,
			cfg.Redis.DB,
			cfg.Redis.CacheTTL,
			cfg.Redis.PoolSize,
			cfg.Redis.MinIdleConns,
			cfg.Redis.DialTimeout,
			cfg.Redis.ReadTimeout,
			cfg.Redis.WriteTimeout,
		)
		if err != nil {
			log.Warn("Failed to initialize Redis cache, continuing without cache", err)
		} else {
			cache = redisCache
			defer cache.Close()
			log.Info("Redis cache initialized successfully")
		}
	}

	// Initialize use cases with cache
	var getHistoricalMetricsUC interface{}
	if cache != nil {
		getHistoricalMetricsUC = usecase.NewGetHistoricalMetricsCachedUseCase(
			metricRepo,
			aggregator,
			cache,
			log,
		)
	} else {
		getHistoricalMetricsUC = usecase.NewGetHistoricalMetricsUseCase(
			metricRepo,
			aggregator,
			log,
		)
	}

	// Add middleware to router
	rateLimiter := middleware.NewIPRateLimiter(100, 200) // 100 RPS, burst 200

	// В router.go добавить:
	r.Use(middleware.Compression)
	r.Use(middleware.RateLimit(rateLimiter))
}
```

### Шаг 6: Обновить router.go

**Файл:** `internal/interfaces/http/router.go`

```go
import (
	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/http/middleware"
)

func NewRouter(/* ... */) *chi.Mux {
	r := chi.NewRouter()

	// Add new middleware (порядок важен!)
	r.Use(middleware.RateLimit(rateLimiter))  // Сначала rate limiting
	r.Use(middleware.Compression)              // Потом compression
	r.Use(corsMiddleware)                      // Затем CORS
	r.Use(loggingMiddleware)                   // И logging

	// ... остальные routes
}
```

---

## 🧪 Тестирование оптимизаций

### Тест 1: Проверка индексов

```sql
-- Проверить, что индексы созданы
SELECT indexname, indexdef
FROM pg_indexes
WHERE tablename = 'metrics'
ORDER BY indexname;

-- Проверить размер индексов
SELECT
    schemaname,
    tablename,
    indexname,
    pg_size_pretty(pg_relation_size(indexname::regclass)) as index_size
FROM pg_indexes
WHERE tablename = 'metrics';

-- Проверить использование индексов
EXPLAIN ANALYZE
SELECT * FROM metrics
WHERE metric_type = 'cpu'
  AND collected_at BETWEEN NOW() - INTERVAL '1 hour' AND NOW()
ORDER BY collected_at DESC
LIMIT 5000;
```

**Ожидаемый результат:** Должен использоваться `idx_metrics_type_time_covering`

### Тест 2: Проверка Redis

```bash
# Подключиться к Redis
redis-cli

# Проверить ключи
KEYS metrics:history:*

# Проверить TTL ключа
TTL metrics:history:cpu:1h:1234567890

# Проверить размер значения
STRLEN metrics:history:cpu:1h:1234567890

# Мониторинг команд в реальном времени
MONITOR
```

### Тест 3: Load Testing

```bash
cd monitoring-dashboard-api

# Smoke test (5 VUs, 5 минут)
make load-test-smoke

# Step test (10 → 60 RPS)
make load-test-step

# Soak test (50 RPS, 1 час)
SOAK_RPS=50 make load-test-soak
```

**Ожидаемые результаты:**
- ✅ Smoke test: P95 < 500ms, 0% errors
- ✅ Step test: Выдерживает 100+ RPS, P95 < 800ms, <1% errors
- ✅ Soak test: Стабильная работа 1+ часов

### Тест 4: Проверка compression

```bash
# Без compression
curl -v https://staging.xyibank.ru/api/v1/metrics/history?type=cpu\&duration=1h

# С compression
curl -v -H "Accept-Encoding: gzip" https://staging.xyibank.ru/api/v1/metrics/history?type=cpu\&duration=1h | gunzip

# Сравнить размеры в заголовке Content-Length
```

---

## 📊 Ожидаемые улучшения

| Метрика | До оптимизаций | После оптимизаций | Улучшение |
|---------|---------------|-------------------|-----------|
| Max RPS | 2-3 RPS | 100+ RPS | **40x** |
| P95 Response Time | 3,000 ms | <500 ms | **6x быстрее** |
| P99 Response Time | 4,700 ms | <800 ms | **6x быстрее** |
| Error Rate @ 10 RPS | 14% | <1% | **14x надежнее** |
| DB Load | 100% | 20% | **5x снижение** |
| Bandwidth Usage | 100% | 20-40% | **3-5x экономия** |
| Cache Hit Rate | 0% | 70-90% | **Новая метрика** |
| Concurrent Users | ~8 | 200+ | **25x** |

---

## 🔄 Обслуживание

### Ежедневные задачи

**1. Мониторинг Redis:**
```bash
# Проверить memory usage
redis-cli INFO memory

# Проверить hit rate
redis-cli INFO stats | grep keyspace
```

**2. Мониторинг БД:**
```sql
-- Размер таблицы и индексов
SELECT
    pg_size_pretty(pg_total_relation_size('metrics')) as total_size,
    pg_size_pretty(pg_relation_size('metrics')) as table_size,
    pg_size_pretty(pg_total_relation_size('metrics') - pg_relation_size('metrics')) as indexes_size;

-- Активные соединения
SELECT count(*) FROM pg_stat_activity WHERE datname = 'monitoring';
```

### Еженедельные задачи

**1. VACUUM ANALYZE:**
```sql
VACUUM ANALYZE metrics;
```

**2. Обновить materialized view:**
```sql
SELECT refresh_metrics_hourly();
```

**3. Проверить медленные запросы:**
```sql
SELECT
    calls,
    mean_exec_time,
    query
FROM pg_stat_statements
WHERE query LIKE '%metrics%'
ORDER BY mean_exec_time DESC
LIMIT 10;
```

### Ежемесячные задачи

**1. Очистка старых данных:**
```sql
-- Удалить данные старше 30 дней
DELETE FROM metrics WHERE collected_at < NOW() - INTERVAL '30 days';
VACUUM FULL metrics;
```

**2. Анализ индексов:**
```sql
-- Неиспользуемые индексы
SELECT * FROM pg_stat_user_indexes WHERE idx_scan = 0;
```

---

## 🚨 Troubleshooting

### Проблема: Redis недоступен

**Симптомы:**
- Логи: "Failed to connect to Redis"
- Приложение работает, но медленнее

**Решение:**
```bash
# Проверить Redis
docker ps | grep redis
redis-cli ping

# Перезапустить Redis
docker restart redis-cache

# Временно отключить в .env
REDIS_ENABLED=false
```

### Проблема: Высокая нагрузка на БД

**Симптомы:**
- Медленные запросы
- Connection pool exhausted

**Решение:**
```sql
-- Убить долгие запросы
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE state = 'active'
  AND query_start < NOW() - INTERVAL '30 seconds';

-- Увеличить connection pool
-- В .env:
DB_MAX_OPEN_CONNS=200
```

### Проблема: Rate limit слишком агрессивный

**Симптомы:**
- Много 429 ошибок
- Легитимные пользователи заблокированы

**Решение:**
```go
// В main.go увеличить лимиты:
rateLimiter := middleware.NewIPRateLimiter(200, 400) // Было 100, 200
```

---

## 📈 Следующие шаги (Future Optimizations)

### P2 - Medium Priority

1. **Horizontal Scaling**
   - Dockerize приложение
   - Deploy 3+ replicas
   - Load balancer (AWS ALB)

2. **Database Read Replicas**
   - Read replica для history queries
   - Primary для writes только
   - pgBouncer connection pooler

3. **CDN для статики**
   - CloudFront / CloudFlare
   - Кеширование JS/CSS/images
   - Edge caching для API

4. **Metrics & Monitoring**
   - Prometheus + Grafana
   - Алерты на высокую latency
   - Dashboard с метриками cache hit rate

---

## ✅ Checklist перед деплоем

- [ ] Миграции 003 применены на staging
- [ ] Redis настроен и доступен
- [ ] Environment variables обновлены
- [ ] go.mod обновлен с новыми зависимостями
- [ ] main.go обновлен для инициализации cache и middleware
- [ ] router.go обновлен с новыми middleware
- [ ] Load tests пройдены (100+ RPS)
- [ ] Smoke test пройден (P95 < 500ms)
- [ ] Monitoring настроен (CloudWatch/Prometheus)
- [ ] Runbook обновлен с новыми процедурами
- [ ] Team проинформирован о изменениях

---

**Документ создан:** 2026-02-08
**Последнее обновление:** 2026-02-08
**Автор:** Performance Optimization Team
