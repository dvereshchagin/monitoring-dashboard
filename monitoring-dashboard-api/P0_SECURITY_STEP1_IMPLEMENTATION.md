# P0 Security Step 1: Техническая реализация (внедрено)

## Что реализовано
1. Feature-flag авторизация для HTTP/WS через Bearer token.
2. Auth API для удобной работы с токеном из UI/ручек:
   - `POST /api/v1/auth/login`
   - `POST /api/v1/auth/logout`
   - `GET /api/v1/auth/status`
3. Защита endpoint'ов:
   - `GET /`
   - `GET /ws`
   - `GET /api/metrics/history`
   - `GET /api/v1/metrics/history`
   - `POST /api/v1/screenshots/dashboard`
4. Строгая проверка WebSocket origin по allowlist `ALLOWED_ORIGINS`.
5. Проверка токена до `WebSocket Upgrade`.
6. Поддержка токена на фронтенде:
   - `localStorage['monitoring_auth_token']`
   - query param `?token=...` (автосохранение в localStorage)
   - автоматический `POST /api/v1/auth/login` для установки HttpOnly cookie
   - Authorization header для fetch-запросов.
7. Middleware теперь принимает токен из:
   - `Authorization: Bearer ...`
   - cookie `monitoring_auth_token`
   - query `?token=...`

## Измененные файлы
- `pkg/config/config.go`
- `internal/interfaces/http/middleware/auth.go`
- `internal/interfaces/http/router.go`
- `internal/interfaces/http/handler/websocket_handler.go`
- `cmd/server/main.go`
- `web/static/js/websocket.js`
- `internal/interfaces/http/handler/auth_api_handler.go`

## Новые env-переменные
- `AUTH_ENABLED=false`
- `AUTH_BEARER_TOKEN=`

Важно:
- если `AUTH_ENABLED=true`, то `AUTH_BEARER_TOKEN` обязателен (иначе приложение не стартует).

## Как включить у себя
1. Добавьте в `.env`:
```env
AUTH_ENABLED=true
AUTH_BEARER_TOKEN=super-secret-token
ALLOWED_ORIGINS=http://localhost:8080,http://127.0.0.1:8080
```

2. Откройте dashboard с токеном:
```text
http://localhost:8080/?token=super-secret-token
```
Токен сохранится и будет использован для auth-cookie автоматически.

3. Для ручной проверки API:
```bash
curl -H "Authorization: Bearer super-secret-token" \
  "http://localhost:8080/api/v1/metrics/history?type=cpu&duration=1h"
```

4. Логин через auth endpoint (альтернатива header):
```bash
curl -i -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"token":"super-secret-token"}'
```

4. Проверка WS (браузер):
- токен берется автоматически из `localStorage` или из `?token=`.

## Что это дает прямо сейчас
- Нельзя читать dashboard/API/WS без токена.
- Нельзя подключаться к WS с чужого origin.
- Можно постепенно включать в окружениях через флаг `AUTH_ENABLED`.

## Ограничения текущего шага
- Это baseline (общий Bearer token), не per-user auth.
- Нет JWT/JWKS и RBAC ролей на этом шаге.

## Следующий шаг (P0 Step 2)
- заменить статический token на JWT validation (`iss`, `aud`, `exp`, `nbf`, подпись через JWKS);
- добавить role-based middleware (`viewer/operator/admin`).
