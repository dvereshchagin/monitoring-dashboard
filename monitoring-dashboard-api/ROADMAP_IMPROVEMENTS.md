# План улучшений Monitoring Dashboard

## 1. Что уже есть (база)
- Сбор системных метрик (CPU/Memory/Disk/Network) каждые 2 секунды.
- Real-time доставка через WebSocket.
- Хранение истории в PostgreSQL.
- Базовые алерты (warning/critical) и отображение на UI.
- CI/CD pipeline (lint/test/build/release).
- Заложена поддержка S3-скриншотов (use case + handler + storage).

## 2. Ключевые наблюдения по текущему состоянию
- Нет автотестов (`*_test.go` отсутствуют).
- `WebSocket CheckOrigin` сейчас разрешает любой origin.
- В конфиге есть `METRICS_RETENTION_DAYS`, но нет запущенного фонового cleanup job.
- Скриншотный API реализован в коде, но не подключен в `main/router` (нет роутинга/DI).
- Исторические графики в UI только по CPU и Memory.
- Нет аутентификации/авторизации и аудита действий.

## 3. Приоритетный roadmap по фичам

### Фича A. Безопасность и доступ (P0)
**Что добавить**
- AuthN/AuthZ: вход через SSO/OAuth2 (или JWT для self-hosted).
- RBAC: роли `viewer`, `operator`, `admin`.
- Ограничение origin и CORS policy для WebSocket/API.
- Rate limiting для публичных API (не только screenshot endpoint).
- Audit log для административных действий.

**Сервисы/инструменты**
- `Keycloak` или `Auth0`/`Okta`.
- `Traefik`/`Nginx` для TLS termination + базовая WAF-политика.

**Критерии готовности**
- Неавторизованный пользователь не получает доступ к dashboard/API.
- WebSocket соединение принимается только с allowlist-origin.
- Есть трассируемый журнал действий администратора.

### Фича B. Надежный алертинг и инциденты (P0)
**Что добавить**
- Alert rules в конфиге/БД (порог, окно, cooldown, severity).
- Каналы уведомлений: Telegram, Slack, Email, PagerDuty.
- Дедупликация алертов и suppress/flapping control.
- Страница active incidents + acknowledge/resolve flow.

**Сервисы/инструменты**
- `Alertmanager` (или встроенный alert dispatcher + adapters).
- SMTP provider (Mailgun/SES) и webhook интеграции.

**Критерии готовности**
- Критический алерт доходит минимум в 2 канала.
- Повторяющиеся события не спамят дежурного.
- У инцидента есть lifecycle: open/ack/resolved.

### Фича C. Мульти-хост мониторинг (P1)
**Что добавить**
- Поддержка нескольких хостов/агентов вместо локального single-node режима.
- Inventory: группы серверов, теги, окружения (`prod/stage/dev`).
- Переключение хоста/группы на UI + агрегированные сводки.

**Сервисы/инструменты**
- Легковесный agent (Go) + gRPC/HTTP ingest endpoint.
- Message broker при росте нагрузки (`NATS`/`Kafka` — по необходимости).

**Критерии готовности**
- Dashboard отображает минимум 20+ хостов с фильтрами.
- Потеря одного агента не ломает сбор остальных.

### Фича D. Data lifecycle и производительность (P1)
**Что добавить**
- Retention worker (удаление/архивация старых метрик).
- Downsampling (raw 2s -> 1m/5m агрегаты) для длинных периодов.
- Партиционирование таблиц по дате.
- Оптимизация API истории для диапазонов 24h/7d/30d.

**Сервисы/инструменты**
- PostgreSQL partitioning + materialized views.
- Опционально: `TimescaleDB` как следующий шаг.

**Критерии готовности**
- Стабильное время ответа API истории на диапазоне 30 дней.
- Размер БД контролируемо растет по retention-политике.

### Фича E. Product UX и аналитика (P1)
**Что добавить**
- Дашборд-конструктор: виджеты, drag-and-drop, сохраненные layouts.
- Графики по всем метрикам + сравнение периодов.
- Drill-down до хоста/метрики/событий.
- Экспорт отчетов (PNG/PDF/CSV), использование S3 screenshot pipeline.

**Сервисы/инструменты**
- Chart.js уже есть, можно расширять без смены стека.
- Headless browser job для scheduled reports.

**Критерии готовности**
- Пользователь может собрать и сохранить кастомный dashboard.
- Доступен экспорт отчета за период по расписанию.

### Фича F. Наблюдаемость самого сервиса (P1)
**Что добавить**
- Метрики приложения (`/metrics`): latency, error rate, WS clients, queue/backpressure.
- Distributed tracing для API/DB вызовов.
- Централизованный structured logging.

**Сервисы/инструменты**
- `Prometheus + Grafana`.
- `OpenTelemetry + Tempo/Jaeger`.
- `Loki` или ELK для логов.

**Критерии готовности**
- Есть SLI/SLO (availability, p95 latency, alert delivery latency).
- Проблема в проде диагностируется по traces+logs без ручного дебага.

### Фича G. Качество и скорость разработки (P0)
**Что добавить**
- Unit tests: domain services/use cases.
- Integration tests: PostgreSQL repository + WebSocket handlers.
- Contract tests для API.
- Pre-commit hooks и покрытие критических сценариев.

**Сервисы/инструменты**
- `testcontainers-go` для интеграционных тестов.
- `k6` для нагрузочных сценариев WebSocket/API.

**Критерии готовности**
- Минимальный baseline покрытия по critical path.
- PR не проходит без тестов и green CI.

### Фича H. Deploy и эксплуатация (P2)
**Что добавить**
- Dockerfile + docker-compose для локального окружения.
- Helm chart/Terraform для repeatable deploy.
- Blue/Green или Canary rollout.
- Backup/restore runbook для БД и S3.

**Сервисы/инструменты**
- `Kubernetes` (если нужен масштаб), иначе systemd + reverse proxy.
- `pgBackRest`/managed snapshots.

**Критерии готовности**
- Развертывание воспроизводимо и документировано.
- RTO/RPO цели определены и проверены.

## 4. План внедрения на 90 дней

### Этап 1 (Недели 1-3) — стабилизация ядра
- Подключить screenshot API в DI/router и закрыть E2E сценарий.
- Включить строгий origin-check для WebSocket.
- Добавить retention worker и cron-like cleanup.
- Запустить пакет unit tests на domain/usecase.

### Этап 2 (Недели 4-7) — безопасность и алертинг
- Внедрить AuthN/AuthZ + RBAC.
- Реализовать Alert rules + cooldown/dedup.
- Добавить каналы Slack/Telegram/Email.
- Сделать страницу active incidents.

### Этап 3 (Недели 8-10) — scale и observability
- Ввести multi-host ingest (v1) и inventory.
- Добавить `/metrics`, traces, dashboard для SLI/SLO.
- Оптимизировать исторические запросы (downsampling/partitioning).

### Этап 4 (Недели 11-13) — продуктовые улучшения
- Расширить UI: все метрики на графиках + фильтры/группы.
- Добавить scheduled reports и экспорт.
- Провести нагрузочное тестирование и hardening.

## 5. Ближайший actionable backlog (следующий спринт)
1. Подключить `ScreenshotAPIHandler` в `cmd/server/main.go` и `internal/interfaces/http/router.go`.
2. Исправить `WebSocket CheckOrigin` на allowlist из `ALLOWED_ORIGINS`.
3. Добавить use case и job для `DeleteOlderThan` по `METRICS_RETENTION_DAYS`.
4. Создать первые unit tests: `metric_validator`, `metric_aggregator`, `collect_metrics`.
5. Добавить интеграционный тест для `/api/metrics/history` и `/ws`.
6. Добавить health/readiness endpoints (`/healthz`, `/readyz`).

## 6. Рекомендуемый стек сервисов (минимум)
- Мониторинг и графики: `Prometheus + Grafana`.
- Логи: `Loki`.
- Трейсинг: `OpenTelemetry + Tempo`.
- Уведомления: `Alertmanager + Slack/Telegram`.
- Auth: `Keycloak` (self-hosted) или `Auth0` (managed).

