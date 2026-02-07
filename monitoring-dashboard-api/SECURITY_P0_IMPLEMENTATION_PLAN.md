# P0 План реализации: Безопасность и аудит (Monitoring Dashboard)

## 1. Цель P0
Закрыть критические риски безопасности до production-ready уровня:
- запретить неавторизованный доступ к dashboard/API/WS;
- ограничить доступ по origin и ролям;
- обеспечить трассируемость административных действий (audit trail);
- добавить базовую защиту от abuse (rate limit, secure headers);
- сделать проверяемую модель контроля через тесты и чек-листы аудита.

## 2. Scope на ближайший релиз (P0)
Входит:
- AuthN (аутентификация пользователей).
- AuthZ (RBAC для API/UI/WS).
- WebSocket security (`CheckOrigin`, auth handshake).
- API security middleware (JWT/session validation + role checks).
- Audit logging для чувствительных операций.
- Security headers + базовый CORS policy.
- Расширенный rate limiting (не только screenshot API).
- Security tests (unit/integration/e2e) и go-live checklist.

Не входит (перенос в P1+):
- полноценная SIEM-интеграция;
- SAST/DAST enterprise pipeline;
- fine-grained ABAC;
- секретное хранилище уровня Vault в полном объеме.

## 3. Варианты реализации (рекомендуемый + альтернативы)

## Вариант A (рекомендуемый): Managed IdP + JWT в приложении
Описание:
- IdP: `Auth0`/`Okta`.
- Приложение валидирует JWT access token.
- Роли/permissions приходят в claims.

Плюсы:
- быстрый time-to-market;
- меньше эксплуатационной нагрузки;
- хорошая документация и enterprise features.

Минусы:
- vendor lock-in;
- регулярные расходы.

Когда выбирать:
- если нужен быстрый запуск и нет ресурсов на поддержку собственного IdP.

## Вариант B: Self-hosted Keycloak + JWT
Описание:
- `Keycloak` как свой IdP.
- JWT валидация на стороне сервиса.

Плюсы:
- полный контроль над IAM;
- без привязки к SaaS-провайдеру.

Минусы:
- нужна эксплуатация Keycloak (backup, HA, обновления).

Когда выбирать:
- если важен контроль и on-prem требования.

## Вариант C: Reverse proxy auth (OIDC) + internal headers
Описание:
- `oauth2-proxy`/`Traefik ForwardAuth` перед приложением.
- Приложение доверяет подписанным заголовкам от прокси.

Плюсы:
- быстрое внедрение для UI;
- меньше кода auth внутри сервиса.

Минусы:
- сложнее защищать прямой доступ к сервису;
- аккуратнее с trust boundary.

Когда выбирать:
- если есть зрелый ingress/proxy слой и нужно минимально менять код.

## 4. Решение для текущего проекта (предложение)
Рекомендуется начать с Варианта B (Keycloak) или A (Auth0) и одинакового кода в приложении:
- внутри Go сервиса реализовать стандартный JWT middleware + RBAC;
- provider-agnostic конфиг (`ISSUER_URL`, `JWKS_URL`, `AUDIENCE`);
- не завязывать бизнес-логику на конкретный IdP.

Это позволит позже переключаться между managed и self-hosted без переписывания handler/usecase.

## 5. Детальный план работ по спринтам

## Sprint 1 (1-1.5 недели): Базовая security-рамка
Задачи:
1. Добавить security config в `pkg/config/config.go`:
   - `AUTH_ENABLED`
   - `AUTH_ISSUER_URL`
   - `AUTH_JWKS_URL`
   - `AUTH_AUDIENCE`
   - `AUTH_REQUIRED_ROLES`
   - `ALLOWED_ORIGINS` (уже есть, расширить использование)
2. Реализовать middleware JWT validation:
   - проверка подписи через JWKS;
   - проверка `iss`, `aud`, `exp`, `nbf`;
   - извлечение `sub`, `email`, `roles` в context.
3. Реализовать middleware RBAC:
   - `viewer`: read endpoints;
   - `operator`: acknowledge/re-run ops;
   - `admin`: admin/config endpoints.
4. Закрыть роуты авторизацией:
   - dashboard `/`
   - `/api/*`
   - `/ws`
5. Обновить WebSocket handler:
   - строгий `CheckOrigin` из allowlist;
   - отказ при отсутствии/невалидном токене.

Артефакты:
- `internal/interfaces/http/middleware/auth.go`
- `internal/interfaces/http/middleware/rbac.go`
- правки `internal/interfaces/http/router.go`
- правки `internal/interfaces/http/handler/websocket_handler.go`

Критерии приемки:
- без токена доступ к защищенным endpoint невозможен;
- токен с неверной ролью получает `403`;
- WS handshake с неразрешенного origin блокируется.

## Sprint 2 (1 неделя): Audit trail и hardening
Задачи:
1. Ввести audit события:
   - login success/fail (если применимо);
   - вызовы admin-операций;
   - действия по инцидентам (ack/resolved);
   - критические security события (forbidden/invalid token).
2. Формат audit log (JSON):
   - `timestamp`, `actor_id`, `actor_email`, `action`, `resource`, `result`, `ip`, `user_agent`, `trace_id`.
3. Хранилище аудита:
   - старт: postgres таблица `audit_events` + structured logs;
   - ротация/retention по сроку.
4. Security headers middleware:
   - `X-Content-Type-Options: nosniff`
   - `X-Frame-Options: DENY`
   - `Referrer-Policy: no-referrer`
   - `Content-Security-Policy` (базовый профиль)
5. CORS policy только на доверенные origins.

Артефакты:
- миграция `audit_events`
- `internal/infrastructure/persistence/postgres/audit_repository_impl.go`
- `internal/interfaces/http/middleware/security_headers.go`

Критерии приемки:
- каждое admin-действие имеет audit запись;
- headers присутствуют на UI/API ответах;
- CORS не пропускает посторонние origin.

## Sprint 3 (0.5-1 неделя): Abuse protection и quality gates
Задачи:
1. Rate limiting middleware для `/api/*` и `/ws` handshake.
2. IP + actor-based лимиты (мягкий и жесткий порог).
3. Тесты безопасности:
   - unit: JWT claims validation, role mapping;
   - integration: protected routes, forbidden scenarios;
   - e2e: WS auth+origin проверки.
4. Обновить CI:
   - прогон security test suite;
   - check конфигурации prod профиля.

Артефакты:
- `internal/interfaces/http/middleware/rate_limit.go`
- `internal/interfaces/http/handler/*_test.go`
- updates в `.github/workflows/ci-cd.yml`

Критерии приемки:
- brute-force и burst-запросы получают `429`;
- security tests обязательны в CI;
- есть runbook реагирования на security-инцидент.

## 6. Матрица доступа (RBAC v1)
- `viewer`:
  - `GET /`
  - `GET /api/metrics/history`
  - `GET/WS /ws` (read stream)
- `operator`:
  - все права `viewer`
  - операции acknowledge/retry (когда появятся endpoint)
- `admin`:
  - все права `operator`
  - конфигурация алертов/безопасности/пользователей (будущие admin endpoint)

## 7. Изменения в архитектуре (чтобы не сломать Clean Architecture)
- Auth/RBAC/Audit остаются в `interfaces` + `infrastructure` слоях.
- UseCase слой получает actor context как входные данные, но не знает про JWT.
- Domain слой не содержит кода конкретных security-провайдеров.

## 8. Технические риски и как их снизить
1. Риск: поломка существующего UI/WS из-за mandatory auth.
   - Митигировать: feature flag `AUTH_ENABLED`, staged rollout.
2. Риск: неверная настройка JWKS/issuer и массовые `401`.
   - Митигировать: health-check auth provider + fallback алерты.
3. Риск: слишком строгий CSP ломает статику/Chart.js.
   - Митигировать: сначала report-only режим CSP.
4. Риск: audit лог переполняет БД.
   - Митигировать: retention + индексы + async insert (по мере роста).

## 9. Оценка трудозатрат
- Sprint 1: 5-7 рабочих дней.
- Sprint 2: 4-5 рабочих дней.
- Sprint 3: 3-4 рабочих дня.
Итого: ~3 недели на P0 security baseline (1-2 инженера).

## 10. Definition of Done (P0 Security)
- Все чувствительные endpoint защищены auth+rbac.
- WS защищен по origin и токену.
- Audit trail покрывает admin/security события.
- Security headers/CORS/rate-limit включены.
- Security тесты green в CI.
- Есть короткий runbook: как реагировать на 401/403 spikes, rate-limit spikes, invalid token incidents.

## 11. Что можно улучшить дополнительно (после P0)
1. Добавить MFA и step-up auth для admin-действий.
2. Вынести audit в отдельное хранилище/stream (Loki/ELK/Kafka).
3. Добавить секреты через Vault/Cloud Secret Manager.
4. Подключить DAST (OWASP ZAP) в nightly pipeline.
5. Реализовать tenant isolation, если появится multi-tenant модель.

