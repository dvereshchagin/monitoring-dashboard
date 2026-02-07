#!/bin/bash

echo "======================================"
echo "  Monitoring Dashboard - Проверка"
echo "======================================"
echo ""

# Проверка Docker
echo "1. Проверка Docker..."
if docker ps > /dev/null 2>&1; then
    echo "   ✅ Docker работает"
else
    echo "   ❌ Docker не отвечает"
    echo "   Откройте Docker Desktop и дождитесь полного запуска"
    exit 1
fi

# Проверка Go
echo "2. Проверка Go..."
if command -v go > /dev/null 2>&1; then
    echo "   ✅ Go установлен: $(go version)"
else
    echo "   ❌ Go не установлен"
    echo "   Установите Go: https://golang.org/dl/"
    exit 1
fi

# Проверка templ
echo "3. Проверка templ..."
if command -v templ > /dev/null 2>&1; then
    echo "   ✅ templ установлен"
else
    echo "   ⚠️  templ не установлен, устанавливаю..."
    go install github.com/a-h/templ/cmd/templ@latest
    echo "   ✅ templ установлен"
fi

# Проверка goose
echo "4. Проверка goose..."
if command -v goose > /dev/null 2>&1; then
    echo "   ✅ goose установлен"
else
    echo "   ⚠️  goose не установлен, устанавливаю..."
    go install github.com/pressly/goose/v3/cmd/goose@latest
    echo "   ✅ goose установлен"
fi

echo ""
echo "======================================"
echo "  ✅ Всё готово к запуску!"
echo "======================================"
echo ""
echo "Выберите способ запуска:"
echo ""
echo "A. Docker Compose (всё в контейнерах):"
echo "   docker compose up --build"
echo ""
echo "B. Hybrid (PostgreSQL в Docker, App локально):"
echo "   Терминал 1:"
echo "     docker run --rm --name monitoring-postgres \\"
echo "       -e POSTGRES_USER=postgres \\"
echo "       -e POSTGRES_PASSWORD=postgres \\"
echo "       -e POSTGRES_DB=monitoring \\"
echo "       -p 5432:5432 postgres:16-alpine"
echo ""
echo "   Терминал 2:"
echo "     cd monitoring-dashboard-api"
echo "     make migrate"
echo "     make run"
echo ""
echo "   Браузер: http://localhost:8080"
echo ""
