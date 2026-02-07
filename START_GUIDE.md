# 🚀 Monitoring Dashboard - Руководство по запуску

## Обзор

Этот проект состоит из двух частей:
- **Backend API** (Go) - `monitoring-dashboard-api/`
- **Frontend** (HTML/CSS/JS) - `monitoring-dashboard-web-ui/static/`

Frontend встроен в Backend и обслуживается через HTTP сервер Go.

---

## Вариант 1: Запуск через Docker Compose (Полный стек)

### Требования
- Docker Desktop

### Команды

```bash
# 1. Запуск всех сервисов (PostgreSQL + App)
docker compose up --build

# 2. Запуск в фоне
docker compose up --build -d

# 3. Просмотр логов
docker compose logs -f app

# 4. Остановка
docker compose down

# 5. Остановка с очисткой данных
docker compose down -v
```

### Доступ
- Dashboard: http://localhost:8080
- PostgreSQL: localhost:5432

---

## Вариант 2: PostgreSQL в Docker + App локально (Рекомендуется для разработки)

### Шаг 1: Запуск PostgreSQL

```bash
# Запуск контейнера
docker run --name monitoring-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=monitoring \
  -p 5432:5432 \
  -d postgres:16-alpine

# Проверка
docker ps | grep postgres
```

### Шаг 2: Установка зависимостей

```bash
cd monitoring-dashboard-api

# Установка templ (если ещё не установлен)
go install github.com/a-h/templ/cmd/templ@latest

# Установка goose для миграций (если ещё не установлен)
go install github.com/pressly/goose/v3/cmd/goose@latest

# Скачивание Go модулей
go mod download
```

### Шаг 3: Применение миграций

```bash
cd monitoring-dashboard-api

# Через Makefile
make migrate

# Или вручную
goose -dir internal/infrastructure/persistence/postgres/migrations \
  postgres "host=localhost port=5432 user=postgres password=postgres dbname=monitoring sslmode=disable" up
```

### Шаг 4: Запуск приложения

```bash
cd monitoring-dashboard-api

# Через Makefile (генерирует темплейты + запускает)
make run

# Или вручную
templ generate
go run cmd/server/main.go
```

### Доступ
- Dashboard: http://localhost:8080
- API: http://localhost:8080/api/v1/metrics/history
- WebSocket: ws://localhost:8080/ws

---

## Вариант 3: Всё локально (без Docker)

### Требования
- Go 1.25+
- PostgreSQL 16+

### Шаг 1: Установка PostgreSQL

**macOS:**
```bash
brew install postgresql@16
brew services start postgresql@16
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt update
sudo apt install postgresql-16
sudo systemctl start postgresql
```

### Шаг 2: Создание базы данных

```bash
# Создание БД
createdb monitoring

# Или через psql
psql -U postgres -c "CREATE DATABASE monitoring;"
```

### Шаг 3: Установка инструментов

```bash
# templ для генерации HTML темплейтов
go install github.com/a-h/templ/cmd/templ@latest

# goose для миграций БД
go install github.com/pressly/goose/v3/cmd/goose@latest
```

### Шаг 4: Настройка и запуск

```bash
cd monitoring-dashboard-api

# Применение миграций
make migrate

# Запуск приложения
make run
```

---

## Проверка работоспособности

### 1. Проверка PostgreSQL

```bash
# Docker
docker exec monitoring-postgres pg_isready -U postgres -d monitoring

# Локально
pg_isready -h localhost -p 5432 -U postgres -d monitoring
```

### 2. Проверка приложения

```bash
# HTTP запрос
curl http://localhost:8080

# Проверка API
curl http://localhost:8080/api/v1/metrics/history?type=cpu&duration=1h
```

### 3. Проверка WebSocket

Откройте http://localhost:8080 в браузере и откройте DevTools -> Network -> WS. 
Вы должны увидеть активное WebSocket соединение к `ws://localhost:8080/ws`.

---

## Устранение проблем

### Docker команды зависают

**Причина:** Docker Desktop не полностью запущен или требуется перезапуск.

**Решение:**
```bash
# Перезапуск Docker Desktop
# macOS: Приложения -> Docker -> Quit Docker Desktop -> Запустить снова

# Проверка статуса
docker info
```

### База данных недоступна

```bash
# Проверка подключения
psql -h localhost -p 5432 -U postgres -d monitoring

# Если ошибка "connection refused":
# - Проверьте, что PostgreSQL запущен
# - Проверьте порт в .env файле
```

### Приложение не стартует

```bash
# Проверьте переменные окружения
cat monitoring-dashboard-api/.env

# Проверьте логи
go run cmd/server/main.go
```

### templ шаблоны не найдены

```bash
cd monitoring-dashboard-api
templ generate

# Проверка сгенерированных файлов
ls internal/interfaces/view/*_templ.go
```

---

## Полезные команды

### Docker

```bash
# Просмотр всех контейнеров
docker ps -a

# Логи конкретного контейнера
docker logs monitoring-postgres
docker logs monitoring-dashboard-app

# Подключение к PostgreSQL
docker exec -it monitoring-postgres psql -U postgres -d monitoring

# Остановка контейнера
docker stop monitoring-postgres

# Удаление контейнера
docker rm monitoring-postgres
```

### Разработка

```bash
cd monitoring-dashboard-api

# Запуск тестов
make test

# Линтинг
make lint

# Сборка бинарника
make build

# Очистка
make clean
```

### База данных

```bash
# Просмотр таблиц
docker exec -it monitoring-postgres psql -U postgres -d monitoring -c "\dt"

# Просмотр последних метрик
docker exec -it monitoring-postgres psql -U postgres -d monitoring -c "SELECT * FROM metrics ORDER BY collected_at DESC LIMIT 10;"

# Очистка данных
docker exec -it monitoring-postgres psql -U postgres -d monitoring -c "TRUNCATE metrics;"
```

---

## Структура проекта

```
monitoring-dashboard/
├── docker-compose.yml              # Docker Compose конфигурация
├── QUICKSTART.md                   # Это руководство
├── monitoring-dashboard-api/       # Backend
│   ├── cmd/server/main.go         # Entry point
│   ├── internal/                  # Основной код
│   │   ├── domain/               # Бизнес-логика
│   │   ├── application/          # Use cases
│   │   ├── infrastructure/       # БД, collectors
│   │   └── interfaces/           # HTTP, WebSocket
│   ├── .env                      # Конфигурация (локально)
│   ├── Makefile                  # Команды сборки
│   └── go.mod                    # Go зависимости
└── monitoring-dashboard-web-ui/   # Frontend
    └── static/                    # CSS, JS, статика
        ├── css/
        └── js/
```

---

## Конфигурация (.env)

```env
# Server
SERVER_PORT=8080

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=monitoring

# Metrics
METRICS_COLLECTION_INTERVAL=2s
METRICS_RETENTION_DAYS=7

# Security (для локальной разработки можно отключить)
AUTH_ENABLED=false
AUTH_BEARER_TOKEN=dev-token-12345

# S3 (опционально, для скриншотов)
S3_ENABLED=false
```

---

## API Endpoints

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/` | Dashboard UI |
| GET | `/static/*` | Статические файлы (CSS, JS) |
| WS | `/ws` | WebSocket для real-time метрик |
| GET | `/api/v1/metrics/history` | Исторические данные |
| POST | `/api/v1/screenshots/dashboard` | Сохранение скриншотов |
| POST | `/api/v1/auth/login` | Аутентификация |

---

## Следующие шаги

1. ✅ Запустите PostgreSQL (Docker или локально)
2. ✅ Примените миграции (`make migrate`)
3. ✅ Запустите приложение (`make run`)
4. ✅ Откройте http://localhost:8080
5. ✅ Наблюдайте real-time метрики!

---

## Поддержка

Если возникли проблемы:
1. Проверьте логи приложения
2. Проверьте подключение к PostgreSQL
3. Убедитесь, что порт 8080 свободен
4. Проверьте `.env` конфигурацию

Удачи! 🎉
