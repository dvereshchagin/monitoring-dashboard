# ⚡️ Быстрый старт - 3 команды

## Проблема с Docker?

Если команды Docker зависают, попробуйте:
1. Открыть Docker Desktop вручную
2. Подождать полной загрузки (значок перестанет мигать)
3. В терминале выполнить: `docker ps`

Если `docker ps` работает - Docker готов!

---

## Запуск - Вариант A (Рекомендуется)

### 1. Запустите PostgreSQL в терминале

```bash
docker run --rm --name monitoring-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=monitoring \
  -p 5432:5432 \
  postgres:16-alpine
```

Оставьте этот терминал открытым!

### 2. В новом терминале примените миграции

```bash
cd monitoring-dashboard-api
make migrate
```

Или вручную:
```bash
cd monitoring-dashboard-api
goose -dir internal/infrastructure/persistence/postgres/migrations \
  postgres "host=localhost port=5432 user=postgres password=postgres dbname=monitoring sslmode=disable" up
```

### 3. Запустите приложение

```bash
cd monitoring-dashboard-api
make run
```

Или вручную:
```bash
cd monitoring-dashboard-api
templ generate
go run cmd/server/main.go
```

### 4. Откройте браузер

```
http://localhost:8080
```

---

## Запуск - Вариант B (Docker Compose)

Если Docker работает нормально:

```bash
# В корне проекта
docker compose up --build
```

Откройте: http://localhost:8080

---

## Что делать если...

### goose не установлен

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

### templ не установлен

```bash
go install github.com/a-h/templ/cmd/templ@latest
```

### PostgreSQL уже занят (порт 5432)

Остановите существующий контейнер:
```bash
docker stop monitoring-postgres
docker rm monitoring-postgres
```

Или используйте другой порт:
```bash
docker run --rm --name monitoring-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=monitoring \
  -p 5433:5432 \
  postgres:16-alpine
```

И обновите `.env`:
```bash
DB_PORT=5433
```

### Приложение не видит статические файлы

Проверьте путь в `internal/interfaces/http/router.go`:
```go
// Должно быть относительно места запуска
fs := http.FileServer(http.Dir("../monitoring-dashboard-web-ui/static"))
```

---

## Проверка работы

### 1. PostgreSQL

```bash
docker exec monitoring-postgres pg_isready -U postgres
# Ожидаемый ответ: postgres:5432 - accepting connections
```

### 2. Приложение

```bash
curl http://localhost:8080
# Должен вернуть HTML страницу
```

### 3. API

```bash
curl "http://localhost:8080/api/v1/metrics/history?type=cpu&duration=1h"
# Должен вернуть JSON с метриками
```

### 4. WebSocket

Откройте http://localhost:8080 в браузере и откройте DevTools (F12):
- Console -> не должно быть ошибок WebSocket
- Network -> WS -> должно быть подключение к ws://localhost:8080/ws

---

## Архитектура метрик

```
Каждые 2 секунды:
  1. Collectors собирают метрики (CPU, RAM, Disk, Network)
  2. UseCase валидирует и сохраняет в PostgreSQL
  3. WebSocket Hub отправляет клиентам
  4. Frontend обновляет UI
```

---

## Полезная информация

**Порты:**
- API/Frontend: 8080
- PostgreSQL: 5432

**Логи:**
```bash
# Docker Compose
docker compose logs -f app

# Локальный запуск
# Логи выводятся в терминал где запущен go run
```

**Остановка:**
```bash
# PostgreSQL (Ctrl+C в терминале где он запущен)
# Или
docker stop monitoring-postgres

# Приложение (Ctrl+C)

# Docker Compose
docker compose down
```

---

## Всё готово! 🎉

Теперь вы можете:
- ✅ Видеть real-time метрики системы
- ✅ Просматривать графики CPU и Memory
- ✅ Получать alerts при высокой нагрузке
- ✅ Использовать REST API для получения исторических данных

Наслаждайтесь мониторингом! 📊
# GitOps Test Sun Feb  8 15:25:43 EET 2026
