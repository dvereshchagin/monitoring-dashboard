#!/bin/bash

# Быстрый запуск только PostgreSQL в Docker для локальной разработки
set -e

echo "🐘 Запуск PostgreSQL в Docker..."
echo ""

# Остановка старого контейнера если есть
docker rm -f monitoring-postgres 2>/dev/null || true

# Запуск PostgreSQL
docker run --name monitoring-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=monitoring \
  -p 5432:5432 \
  -d postgres:16-alpine

echo "⏳ Ожидание запуска PostgreSQL..."
sleep 5

# Проверка
if docker exec monitoring-postgres pg_isready -U postgres -d monitoring > /dev/null 2>&1; then
    echo "✅ PostgreSQL запущен успешно!"
    echo ""
    echo "Параметры подключения:"
    echo "  Host: localhost"
    echo "  Port: 5432"
    echo "  User: postgres"
    echo "  Password: postgres"
    echo "  Database: monitoring"
    echo ""
    echo "Применение миграций:"
    echo "  cd monitoring-dashboard-api && make migrate"
    echo ""
    echo "Запуск приложения:"
    echo "  cd monitoring-dashboard-api && make run"
else
    echo "❌ Не удалось запустить PostgreSQL"
    docker logs monitoring-postgres
    exit 1
fi
