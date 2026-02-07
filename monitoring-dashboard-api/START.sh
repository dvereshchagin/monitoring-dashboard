#!/bin/bash

# Monitoring Dashboard - Quick Start Script
# This script will help you get the application running

set -e

echo "🚀 Monitoring Dashboard - Quick Start"
echo "======================================"
echo ""

# Step 1: Check PostgreSQL
echo "Step 1: Checking PostgreSQL..."
if ! pg_isready -q 2>/dev/null; then
    echo "❌ PostgreSQL is not running"
    echo ""
    echo "Please start PostgreSQL first:"
    echo "  macOS:  brew services start postgresql@14"
    echo "  Linux:  sudo systemctl start postgresql"
    echo ""
    exit 1
fi
echo "✅ PostgreSQL is running"
echo ""

# Step 2: Check if database exists
echo "Step 2: Checking database..."
if psql -U postgres -lqt 2>/dev/null | cut -d \| -f 1 | grep -qw monitoring; then
    echo "✅ Database 'monitoring' exists"
else
    echo "📦 Creating database 'monitoring'..."
    createdb monitoring 2>/dev/null || psql -U postgres -c "CREATE DATABASE monitoring;" 2>/dev/null
    echo "✅ Database created"
fi
echo ""

# Step 3: Run migrations
echo "Step 3: Running migrations..."
if [ ! -f "internal/infrastructure/persistence/postgres/migrations/001_init.sql" ]; then
    echo "❌ Migration files not found"
    exit 1
fi

psql -U postgres -d monitoring -f internal/infrastructure/persistence/postgres/migrations/001_init.sql >/dev/null 2>&1
psql -U postgres -d monitoring -f internal/infrastructure/persistence/postgres/migrations/002_indexes.sql >/dev/null 2>&1
echo "✅ Migrations completed"
echo ""

# Step 4: Check .env file
echo "Step 4: Checking configuration..."
if [ ! -f ".env" ]; then
    if [ -f ".env.example" ]; then
        cp .env.example .env
        echo "✅ Created .env from .env.example"
    else
        echo "⚠️  No .env file found (will use defaults)"
    fi
else
    echo "✅ .env file exists"
fi
echo ""

# Step 5: Build application
echo "Step 5: Building application..."
make build
echo "✅ Build successful"
echo ""

# Step 6: Start application
echo "======================================"
echo "🎉 Ready to start!"
echo "======================================"
echo ""
echo "Starting Monitoring Dashboard..."
echo ""
echo "Dashboard will be available at: http://localhost:8080"
echo ""
echo "Press Ctrl+C to stop the server"
echo ""

./bin/monitoring-dashboard
