# Monitoring Dashboard - Инструкция по запуску

## Быстрый старт с Docker (рекомендуется)

### Требования
- Docker Desktop
- Docker Compose

### Запуск

1. **Убедитесь, что Docker запущен**

2. **Запустите проект одной командой:**

```bash
./docker-start.sh
```

Или вручную:

```bash
docker-compose up --build -d
```

3. **Откройте дашборд:**

```
http://localhost:8080
```

### Полезные команды

```bash
# Просмотр логов
docker-compose logs -f app

# Остановка
docker-compose down

# Остановка с удалением данных
docker-compose down -v

# Перезапуск
docker-compose restart app

# Подключение к базе данных
docker-compose exec postgres psql -U postgres -d monitoring
```

---

## Локальная разработка (без Docker)

### Требования
- Go 1.25+
- PostgreSQL 16+
- templ CLI

### Установка зависимостей

```bash
cd monitoring-dashboard-api

# Установка Go зависимостей
go mod download

# Установка templ
go install github.com/a-h/templ/cmd/templ@latest

# Установка goose для миграций
go install github.com/pressly/goose/v3/cmd/goose@latest
```

### Настройка PostgreSQL

```bash
# Запуск PostgreSQL (macOS)
brew services start postgresql@16

# Создание базы данных
createdb monitoring

# Применение миграций
cd monitoring-dashboard-api
make migrate
```

### Запуск приложения

```bash
cd monitoring-dashboard-api

# Генерация templ шаблонов и запуск
make run

# Или просто
make dev
```

Дашборд будет доступен по адресу: http://localhost:8080

---

## Архитектура

```
monitoring-dashboard/
├── monitoring-dashboard-api/      # Backend API (Go)
│   ├── cmd/server/               # Entry point
│   ├── internal/                 # Clean Architecture layers
│   │   ├── domain/              # Бизнес-логика
│   │   ├── application/         # Use Cases
│   │   ├── infrastructure/      # БД, collectors, WebSocket
│   │   └── interfaces/          # HTTP handlers, views
│   ├── pkg/                     # Shared utilities
│   └── go.mod
├── monitoring-dashboard-web-ui/  # Frontend (HTML/CSS/JS)
│   └── static/
│       ├── css/
│       └── js/
└── docker-compose.yml           # Docker setup
```

---

## Возможности

✅ Мониторинг в реальном времени (CPU, Memory, Disk, Network)  
✅ WebSocket обновления каждые 2 секунды  
✅ Исторические данные с Chart.js  
✅ PostgreSQL для хранения метрик  
✅ Clean Architecture + DDD  
✅ Type-safe HTML шаблоны (Templ)  
✅ Docker поддержка  

---

## API Endpoints

- `GET /` - Dashboard
- `GET /api/v1/metrics/history?type=cpu&duration=1h` - Исторические данные
- `WS /ws` - WebSocket для real-time метрик
- `POST /api/v1/screenshots/dashboard` - Сохранение скриншотов (требует AUTH)

---

## Конфигурация

Настройки хранятся в `.env` файле (для локальной разработки) или в переменных окружения Docker Compose.

Основные параметры:
- `SERVER_PORT` - порт HTTP сервера (по умолчанию: 8080)
- `DB_HOST` - хост PostgreSQL
- `METRICS_COLLECTION_INTERVAL` - интервал сбора метрик (по умолчанию: 2s)
- `AUTH_ENABLED` - включить/выключить аутентификацию
- `S3_ENABLED` - включить/выключить S3 storage для скриншотов

---

## Устранение проблем

### WebSocket не подключается
- Проверьте браузерную консоль на ошибки
- Убедитесь, что сервер запущен на порту 8080

### База данных недоступна
```bash
# Docker
docker-compose exec postgres pg_isready -U postgres

# Local
pg_isready -h localhost -p 5432
```

### Приложение не стартует
```bash
# Проверьте логи
docker-compose logs app

# Или локально посмотрите вывод в терминале
```

---

## Лицензия

MIT
