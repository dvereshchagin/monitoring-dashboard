# Quick Start Guide

## Prerequisites

Убедитесь что установлены:

```bash
# PostgreSQL
brew install postgresql@14  # macOS
# или
sudo apt-get install postgresql  # Ubuntu/Debian

# Запустите PostgreSQL
brew services start postgresql@14  # macOS
# или
sudo systemctl start postgresql  # Linux
```

## Быстрый запуск (5 минут)

### 1. Создайте базу данных

```bash
createdb monitoring
```

Или через psql:

```bash
psql -U postgres
CREATE DATABASE monitoring;
\q
```

### 2. Запустите миграции

```bash
make migrate
```

Или вручную:

```bash
psql -U postgres -d monitoring -f internal/infrastructure/persistence/postgres/migrations/001_init.sql
psql -U postgres -d monitoring -f internal/infrastructure/persistence/postgres/migrations/002_indexes.sql
```

### 3. Запустите приложение

```bash
make run
```

Или соберите и запустите бинарь:

```bash
make build
./bin/monitoring-dashboard
```

### 4. Откройте dashboard

```
http://localhost:8080
```

## Что вы увидите

✅ **4 метрики в реальном времени:**
- CPU Usage (%)
- Memory Usage (%)
- Disk Usage (%)
- Network Sent (KB/s)

✅ **Обновление каждые 2 секунды** через WebSocket

✅ **Исторические графики** за последний час

✅ **Статус подключения** (Connected/Disconnected)

## Проверка работы

### 1. Проверьте логи

```bash
# Должны увидеть:
[INFO] Starting Monitoring Dashboard
[INFO] Database connected successfully
[INFO] WebSocket hub started
[INFO] Metrics collector started interval=2s
[INFO] HTTP server starting port=8080
[INFO] Dashboard available at http://localhost:8080
```

### 2. Проверьте базу данных

```bash
psql -U postgres -d monitoring -c "SELECT COUNT(*) FROM metrics;"
psql -U postgres -d monitoring -c "SELECT metric_type, COUNT(*) FROM metrics GROUP BY metric_type;"
```

Через минуту должны увидеть ~120 записей (4 метрики × 30 секунд / 2 = 60 записей в минуту).

### 3. Проверьте WebSocket

Откройте DevTools → Network → WS:

```
ws://localhost:8080/ws [101 Switching Protocols]
```

В Messages должны приходить JSON snapshots каждые 2 секунды.

### 4. Проверьте API

```bash
curl "http://localhost:8080/api/metrics/history?type=cpu&duration=1h" | jq
```

## Troubleshooting

### Ошибка подключения к БД

```bash
# Проверьте что PostgreSQL запущен
pg_isready

# Проверьте настройки в .env
cat .env
```

### Порт 8080 занят

Измените в `.env`:

```bash
SERVER_PORT=8081
```

### Метрики не обновляются

Проверьте что нет ошибок в логах и что collector запущен.

## Next Steps

После запуска:

1. ⭐ Откройте несколько вкладок браузера - все будут обновляться синхронно
2. ⭐ Нагрузите систему (откройте много приложений) - увидите рост метрик
3. ⭐ Изучите код в `internal/` - Clean Architecture в действии
4. ⭐ Прочитайте `claude.md` - правила разработки проекта
5. ⭐ Ознакомьтесь с планом в `.claude/plans/` - детальный план архитектуры

## Architecture Overview

```
Domain Layer (Core)
  ↑ depends on
Application Layer (Use Cases)
  ↑ depends on
Infrastructure Layer (DB, Collectors, WebSocket)
  ↑ depends on
Interfaces Layer (HTTP, Views)
```

Все зависимости направлены **внутрь** к Domain слою.

Enjoy! 🚀
