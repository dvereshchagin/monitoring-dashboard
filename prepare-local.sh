#!/bin/bash

# Мониторинг Dashboard - Локальный запуск
set -e

echo "🚀 Monitoring Dashboard - Локальный запуск"
echo "==========================================="
echo ""

cd monitoring-dashboard-api

# Проверка Go
if ! command -v go &> /dev/null; then
    echo "❌ Go не установлен"
    echo "Установите Go: https://golang.org/dl/"
    exit 1
fi
echo "✅ Go установлен: $(go version)"

# Установка templ если нужно
if ! command -v templ &> /dev/null; then
    echo "📦 Установка templ..."
    go install github.com/a-h/templ/cmd/templ@latest
fi
echo "✅ templ установлен"

# Генерация темплейтов
echo ""
echo "🔧 Генерация темплейтов..."
templ generate
echo "✅ Темплейты сгенерированы"

# Скачивание зависимостей
echo ""
echo "📦 Скачивание зависимостей..."
go mod download
echo "✅ Зависимости скачаны"

echo ""
echo "==========================================="
echo "🎉 Всё готово к запуску!"
echo "==========================================="
echo ""
echo "ВАЖНО: Для работы нужна PostgreSQL база данных"
echo ""
echo "Варианты запуска PostgreSQL:"
echo ""
echo "1. Docker (рекомендуется для разработки):"
echo "   docker run --name monitoring-postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=monitoring -p 5432:5432 -d postgres:16-alpine"
echo ""
echo "2. Локально через Homebrew:"
echo "   brew install postgresql@16"
echo "   brew services start postgresql@16"
echo "   createdb monitoring"
echo ""
echo "После запуска PostgreSQL выполните:"
echo "   make migrate    # Применить миграции"
echo "   make run        # Запустить приложение"
echo ""
echo "Дашборд будет доступен: http://localhost:8080"
