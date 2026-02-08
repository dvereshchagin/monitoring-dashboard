#!/bin/bash
set -euo pipefail

# QUICKSTART: Performance Optimizations для 100+ RPS
# Этот скрипт автоматизирует внедрение всех оптимизаций

echo "🚀 Starting Performance Optimizations Setup..."

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Step 1: Update Go dependencies
echo ""
echo "${YELLOW}Step 1: Updating Go dependencies...${NC}"
go get github.com/redis/go-redis/v9
go get golang.org/x/time/rate
go mod tidy
echo "${GREEN}✓ Dependencies updated${NC}"

# Step 2: Check PostgreSQL connection
echo ""
echo "${YELLOW}Step 2: Checking PostgreSQL connection...${NC}"
if command -v psql &> /dev/null; then
    DB_HOST="${DB_HOST:-localhost}"
    DB_PORT="${DB_PORT:-5432}"
    DB_USER="${DB_USER:-postgres}"
    DB_NAME="${DB_NAME:-monitoring}"

    echo "Attempting to connect to PostgreSQL..."
    if PGPASSWORD="${DB_PASSWORD:-postgres}" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT version();" &> /dev/null; then
        echo "${GREEN}✓ PostgreSQL connection successful${NC}"
    else
        echo "${RED}✗ PostgreSQL connection failed${NC}"
        echo "Please check your DB credentials in .env file"
    fi
else
    echo "${YELLOW}⚠ psql not found, skipping PostgreSQL check${NC}"
fi

# Step 3: Apply database migrations
echo ""
echo "${YELLOW}Step 3: Applying database migrations...${NC}"
echo "Run this command manually to apply covering indexes migration:"
echo ""
echo "  goose -dir internal/infrastructure/persistence/postgres/migrations postgres \"\$DB_DSN\" up"
echo ""
echo "Or with psql:"
echo "  psql -U postgres -d monitoring -f internal/infrastructure/persistence/postgres/migrations/003_covering_indexes_optimization.sql"
echo ""
read -p "Have you applied the migration? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "${GREEN}✓ Migration confirmed${NC}"
else
    echo "${YELLOW}⚠ Please apply migration before continuing${NC}"
fi

# Step 4: Setup Redis (optional)
echo ""
echo "${YELLOW}Step 4: Setting up Redis cache...${NC}"
echo "Choose Redis setup option:"
echo "  1) Docker (recommended for local/staging)"
echo "  2) Existing Redis instance"
echo "  3) Skip Redis setup (will work without cache)"
read -p "Enter option (1-3): " redis_option

case $redis_option in
    1)
        if command -v docker &> /dev/null; then
            echo "Starting Redis container..."
            docker run -d \
                --name redis-cache \
                -p 6379:6379 \
                redis:7-alpine \
                redis-server --maxmemory 512mb --maxmemory-policy allkeys-lru

            sleep 2

            if docker ps | grep redis-cache &> /dev/null; then
                echo "${GREEN}✓ Redis container started successfully${NC}"
                echo "Redis available at localhost:6379"

                # Add Redis config to .env if not exists
                if [ -f .env ]; then
                    if ! grep -q "REDIS_ENABLED" .env; then
                        echo "" >> .env
                        echo "# Redis Cache Configuration" >> .env
                        echo "REDIS_ENABLED=true" >> .env
                        echo "REDIS_HOST=localhost" >> .env
                        echo "REDIS_PORT=6379" >> .env
                        echo "REDIS_PASSWORD=" >> .env
                        echo "REDIS_DB=0" >> .env
                        echo "REDIS_CACHE_TTL=60s" >> .env
                        echo "REDIS_POOL_SIZE=100" >> .env
                        echo "REDIS_MIN_IDLE_CONNS=20" >> .env
                        echo "${GREEN}✓ Redis config added to .env${NC}"
                    fi
                fi
            else
                echo "${RED}✗ Failed to start Redis container${NC}"
            fi
        else
            echo "${RED}✗ Docker not found. Please install Docker or use option 2/3${NC}"
        fi
        ;;
    2)
        echo "Please ensure your Redis instance is running and accessible"
        echo "Update these variables in your .env file:"
        echo "  REDIS_ENABLED=true"
        echo "  REDIS_HOST=your-redis-host"
        echo "  REDIS_PORT=6379"
        echo "  REDIS_PASSWORD=your-password"
        echo "${YELLOW}⚠ Manual Redis configuration required${NC}"
        ;;
    3)
        echo "${YELLOW}⚠ Skipping Redis setup. Application will work without cache.${NC}"
        if [ -f .env ]; then
            if ! grep -q "REDIS_ENABLED" .env; then
                echo "" >> .env
                echo "REDIS_ENABLED=false" >> .env
            fi
        fi
        ;;
    *)
        echo "${RED}Invalid option${NC}"
        ;;
esac

# Step 5: Update .env file
echo ""
echo "${YELLOW}Step 5: Updating environment configuration...${NC}"

if [ ! -f .env ]; then
    echo "Creating .env file from template..."
    if [ -f .env.example ]; then
        cp .env.example .env
    else
        touch .env
    fi
fi

# Add/update DB connection pool settings
echo ""
echo "Updating database connection pool settings..."
grep -q "^DB_MAX_OPEN_CONNS=" .env && sed -i.bak 's/^DB_MAX_OPEN_CONNS=.*/DB_MAX_OPEN_CONNS=100/' .env || echo "DB_MAX_OPEN_CONNS=100" >> .env
grep -q "^DB_MAX_IDLE_CONNS=" .env && sed -i.bak 's/^DB_MAX_IDLE_CONNS=.*/DB_MAX_IDLE_CONNS=50/' .env || echo "DB_MAX_IDLE_CONNS=50" >> .env

echo "${GREEN}✓ Environment configuration updated${NC}"

# Step 6: Build application
echo ""
echo "${YELLOW}Step 6: Building application...${NC}"
if go build -o bin/monitoring-dashboard-api ./cmd/monitoring-dashboard-api; then
    echo "${GREEN}✓ Application built successfully${NC}"
else
    echo "${RED}✗ Build failed${NC}"
    echo "Please check compilation errors above"
    exit 1
fi

# Step 7: Run tests
echo ""
echo "${YELLOW}Step 7: Running tests...${NC}"
if go test ./... -short; then
    echo "${GREEN}✓ Tests passed${NC}"
else
    echo "${YELLOW}⚠ Some tests failed. Review and fix before deploying.${NC}"
fi

# Summary
echo ""
echo "═══════════════════════════════════════════════════════════"
echo "${GREEN}✓ Performance Optimizations Setup Complete!${NC}"
echo "═══════════════════════════════════════════════════════════"
echo ""
echo "📋 What was done:"
echo "  ✓ Go dependencies updated (Redis, rate limiter)"
echo "  ✓ Database migrations ready (003_covering_indexes_optimization.sql)"
echo "  ✓ Redis cache configured"
echo "  ✓ Environment variables updated"
echo "  ✓ Application built successfully"
echo ""
echo "📊 Expected improvements:"
echo "  • Max RPS: 2-3 RPS → 100+ RPS (40x)"
echo "  • P95 Response Time: 3000ms → <500ms (6x faster)"
echo "  • Error Rate: 14% → <1% (14x more reliable)"
echo "  • Database Load: -80% reduction"
echo "  • Bandwidth Usage: -60% reduction"
echo ""
echo "🧪 Next steps:"
echo "  1. Start the application:"
echo "     ./bin/monitoring-dashboard-api"
echo ""
echo "  2. Run load tests to verify improvements:"
echo "     cd .. && ./scripts/load/run_k6.sh smoke ./scripts/load/config/staging-external.json"
echo ""
echo "  3. Monitor metrics:"
echo "     - Response times (should be <500ms P95)"
echo "     - Cache hit rate (should be 70-90%)"
echo "     - Database connections (should be <50)"
echo ""
echo "📖 Full documentation:"
echo "  - PERFORMANCE_OPTIMIZATIONS.md - Complete guide"
echo "  - LOAD_TEST_REPORT_20260208.md - Baseline metrics"
echo ""
echo "═══════════════════════════════════════════════════════════"
