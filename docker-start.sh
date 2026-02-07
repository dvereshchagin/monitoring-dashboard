#!/bin/bash

# Monitoring Dashboard - Docker Quick Start
set -e

echo "🚀 Monitoring Dashboard - Docker Setup"
echo "========================================"
echo ""

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running"
    echo "Please start Docker Desktop and try again"
    exit 1
fi
echo "✅ Docker is running"
echo ""

# Stop and remove existing containers
echo "🧹 Cleaning up old containers..."
docker-compose down -v 2>/dev/null || true
echo ""

# Build and start services
echo "🔨 Building and starting services..."
docker-compose up --build -d
echo ""

# Wait for services to be healthy
echo "⏳ Waiting for services to be ready..."
sleep 5

# Check if postgres is healthy
MAX_RETRIES=30
RETRY_COUNT=0
until docker-compose exec -T postgres pg_isready -U postgres -d monitoring > /dev/null 2>&1; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
        echo "❌ PostgreSQL failed to start"
        docker-compose logs postgres
        exit 1
    fi
    echo "Waiting for PostgreSQL... ($RETRY_COUNT/$MAX_RETRIES)"
    sleep 2
done
echo "✅ PostgreSQL is ready"
echo ""

# Check if app is healthy
RETRY_COUNT=0
until curl -f http://localhost:8080/health > /dev/null 2>&1; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
        echo "⚠️  App might not be fully ready, checking logs..."
        docker-compose logs app | tail -20
        break
    fi
    sleep 2
done

echo "======================================"
echo "🎉 Monitoring Dashboard is running!"
echo "======================================"
echo ""
echo "📊 Dashboard:    http://localhost:8080"
echo "🔌 WebSocket:    ws://localhost:8080/ws"
echo "📈 API:          http://localhost:8080/api/v1/metrics/history"
echo ""
echo "Useful commands:"
echo "  docker-compose logs -f          # View all logs"
echo "  docker-compose logs -f app      # View app logs"
echo "  docker-compose logs -f postgres # View database logs"
echo "  docker-compose down             # Stop services"
echo "  docker-compose down -v          # Stop and remove volumes"
echo ""
echo "Press Ctrl+C to exit (services will continue running)"
echo ""

# Follow logs
docker-compose logs -f
