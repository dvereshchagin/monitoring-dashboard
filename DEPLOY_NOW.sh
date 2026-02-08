#!/bin/bash
set -euo pipefail

# Quick Deploy Script for Performance Optimizations
# Usage: ./DEPLOY_NOW.sh staging|production

ENVIRONMENT="${1:-staging}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo ""
echo "╔════════════════════════════════════════════════════════════╗"
echo "║  🚀 Performance Optimizations Deployment                  ║"
echo "║  Environment: ${ENVIRONMENT^^}                                       ║"
echo "║  Commit: 0a25111                                          ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Validate environment
if [[ "$ENVIRONMENT" != "staging" && "$ENVIRONMENT" != "production" ]]; then
    echo "${RED}Error: Environment must be 'staging' or 'production'${NC}"
    exit 1
fi

# Production safety check
if [[ "$ENVIRONMENT" == "production" ]]; then
    echo "${YELLOW}⚠️  WARNING: Deploying to PRODUCTION${NC}"
    echo ""
    read -p "Have you tested on staging? (yes/no): " tested
    if [[ "$tested" != "yes" ]]; then
        echo "${RED}Please test on staging first!${NC}"
        exit 1
    fi

    read -p "Type 'DEPLOY TO PRODUCTION' to continue: " confirm
    if [[ "$confirm" != "DEPLOY TO PRODUCTION" ]]; then
        echo "${RED}Deployment cancelled${NC}"
        exit 1
    fi
fi

echo ""
echo "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo "${BLUE}Step 1: Pre-deployment Checks${NC}"
echo "${BLUE}═══════════════════════════════════════════════════════════${NC}"

# Check if on correct branch
CURRENT_BRANCH=$(git branch --show-current)
echo "Current branch: $CURRENT_BRANCH"
if [[ "$CURRENT_BRANCH" != "main" ]]; then
    echo "${YELLOW}⚠️  Warning: Not on main branch${NC}"
    read -p "Continue anyway? (yes/no): " continue_anyway
    if [[ "$continue_anyway" != "yes" ]]; then
        exit 1
    fi
fi

# Check if commit exists
if ! git log --oneline | grep -q "0a25111"; then
    echo "${RED}Error: Performance optimization commit (0a25111) not found${NC}"
    exit 1
fi
echo "${GREEN}✓ Commit found${NC}"

# Check if build succeeds
echo ""
echo "Building application..."
cd monitoring-dashboard-api
if go build -o bin/monitoring-dashboard-api ./cmd/monitoring-dashboard-api; then
    echo "${GREEN}✓ Build successful${NC}"
    BUILD_SIZE=$(ls -lh bin/monitoring-dashboard-api | awk '{print $5}')
    echo "  Binary size: $BUILD_SIZE"
else
    echo "${RED}✗ Build failed${NC}"
    exit 1
fi

cd "$SCRIPT_DIR"

echo ""
echo "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo "${BLUE}Step 2: Database Migration${NC}"
echo "${BLUE}═══════════════════════════════════════════════════════════${NC}"

# Get database credentials
if [[ "$ENVIRONMENT" == "staging" ]]; then
    DB_HOST="${STAGING_DB_HOST:-localhost}"
    DB_USER="${STAGING_DB_USER:-postgres}"
    DB_NAME="${STAGING_DB_NAME:-monitoring}"
else
    DB_HOST="${PROD_DB_HOST:-localhost}"
    DB_USER="${PROD_DB_USER:-postgres}"
    DB_NAME="${PROD_DB_NAME:-monitoring}"
fi

echo "Database: $DB_NAME @ $DB_HOST"
echo ""
echo "${YELLOW}IMPORTANT: Apply migration manually${NC}"
echo ""
echo "Run this command:"
echo ""
echo "  psql -h $DB_HOST -U $DB_USER -d $DB_NAME -f \\"
echo "    monitoring-dashboard-api/internal/infrastructure/persistence/postgres/migrations/003_covering_indexes_optimization.sql"
echo ""
read -p "Have you applied the migration? (yes/no): " migration_done
if [[ "$migration_done" != "yes" ]]; then
    echo "${RED}Please apply migration first!${NC}"
    exit 1
fi
echo "${GREEN}✓ Migration confirmed${NC}"

echo ""
echo "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo "${BLUE}Step 3: Redis Setup${NC}"
echo "${BLUE}═══════════════════════════════════════════════════════════${NC}"

echo "Choose Redis option:"
echo "  1) Use existing Redis"
echo "  2) Deploy new Redis container"
echo "  3) Skip Redis (not recommended)"
read -p "Enter option (1-3): " redis_option

case $redis_option in
    1)
        echo "Using existing Redis"
        read -p "Redis host: " REDIS_HOST
        read -p "Redis port [6379]: " REDIS_PORT
        REDIS_PORT="${REDIS_PORT:-6379}"
        echo "${GREEN}✓ Redis configured${NC}"
        REDIS_ENABLED=true
        ;;
    2)
        echo "Deploying Redis container..."
        if command -v docker &> /dev/null; then
            docker run -d \
                --name redis-cache-${ENVIRONMENT} \
                --restart unless-stopped \
                -p 6379:6379 \
                redis:7-alpine \
                redis-server --maxmemory 1gb --maxmemory-policy allkeys-lru

            sleep 2
            if docker ps | grep redis-cache-${ENVIRONMENT} &> /dev/null; then
                echo "${GREEN}✓ Redis container started${NC}"
                REDIS_HOST=localhost
                REDIS_PORT=6379
                REDIS_ENABLED=true
            else
                echo "${RED}✗ Failed to start Redis${NC}"
                exit 1
            fi
        else
            echo "${RED}Docker not found${NC}"
            exit 1
        fi
        ;;
    3)
        echo "${YELLOW}⚠️  Skipping Redis (performance will be limited)${NC}"
        REDIS_ENABLED=false
        ;;
    *)
        echo "${RED}Invalid option${NC}"
        exit 1
        ;;
esac

echo ""
echo "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo "${BLUE}Step 4: Update Configuration${NC}"
echo "${BLUE}═══════════════════════════════════════════════════════════${NC}"

# Generate .env updates
ENV_FILE="monitoring-dashboard-api/.env.${ENVIRONMENT}"
echo ""
echo "Add these to your .env file ($ENV_FILE):"
echo ""
cat <<EOF
# Performance Optimizations - Added $(date +%Y-%m-%d)
DB_MAX_OPEN_CONNS=100
DB_MAX_IDLE_CONNS=50

REDIS_ENABLED=${REDIS_ENABLED}
EOF

if [[ "$REDIS_ENABLED" == "true" ]]; then
cat <<EOF
REDIS_HOST=${REDIS_HOST}
REDIS_PORT=${REDIS_PORT}
REDIS_PASSWORD=
REDIS_DB=0
REDIS_CACHE_TTL=60s
REDIS_POOL_SIZE=100
REDIS_MIN_IDLE_CONNS=20
EOF
fi

echo ""
read -p "Have you updated the .env file? (yes/no): " env_updated
if [[ "$env_updated" != "yes" ]]; then
    echo "${YELLOW}⚠️  Please update .env before continuing${NC}"
    exit 1
fi
echo "${GREEN}✓ Configuration updated${NC}"

echo ""
echo "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo "${BLUE}Step 5: Deploy Application${NC}"
echo "${BLUE}═══════════════════════════════════════════════════════════${NC}"

echo ""
echo "Deployment method:"
echo "  1) Docker"
echo "  2) Kubernetes (Helm)"
echo "  3) Direct (systemd)"
echo "  4) Manual (I'll do it myself)"
read -p "Enter option (1-4): " deploy_method

case $deploy_method in
    1)
        echo "Building Docker image..."
        cd monitoring-dashboard-api
        docker build -t monitoring-dashboard-api:v2.0.0-perf .

        echo "Stopping old container..."
        docker stop monitoring-dashboard-api-${ENVIRONMENT} 2>/dev/null || true
        docker rm monitoring-dashboard-api-${ENVIRONMENT} 2>/dev/null || true

        echo "Starting new container..."
        docker run -d \
            --name monitoring-dashboard-api-${ENVIRONMENT} \
            --restart unless-stopped \
            -p 8080:8080 \
            --env-file .env.${ENVIRONMENT} \
            monitoring-dashboard-api:v2.0.0-perf

        echo "${GREEN}✓ Container deployed${NC}"
        ;;
    2)
        echo "Deploying with Helm..."
        cd infra/helm
        helm upgrade monitoring-dashboard ./monitoring-dashboard \
            --namespace ${ENVIRONMENT} \
            --set image.tag=v2.0.0-perf \
            --set redis.enabled=${REDIS_ENABLED} \
            --install

        kubectl rollout status deployment/monitoring-dashboard -n ${ENVIRONMENT}
        echo "${GREEN}✓ Helm deployment complete${NC}"
        ;;
    3)
        echo "Restarting systemd service..."
        sudo systemctl restart monitoring-dashboard-${ENVIRONMENT}
        sleep 3
        sudo systemctl status monitoring-dashboard-${ENVIRONMENT}
        echo "${GREEN}✓ Service restarted${NC}"
        ;;
    4)
        echo "${YELLOW}Manual deployment selected${NC}"
        echo "Make sure to:"
        echo "  1. Deploy the new binary"
        echo "  2. Restart the application"
        echo "  3. Verify it's running"
        read -p "Press Enter when deployment is complete..."
        ;;
    *)
        echo "${RED}Invalid option${NC}"
        exit 1
        ;;
esac

echo ""
echo "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo "${BLUE}Step 6: Verification${NC}"
echo "${BLUE}═══════════════════════════════════════════════════════════${NC}"

# Get application URL
if [[ "$ENVIRONMENT" == "staging" ]]; then
    APP_URL="https://staging.xyibank.ru"
else
    APP_URL="https://xyibank.ru"
fi

echo ""
echo "Verifying deployment..."

# Health check
echo -n "Health check... "
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" ${APP_URL}/api/v1/auth/status)
if [[ "$HTTP_CODE" == "200" ]]; then
    echo "${GREEN}✓ OK${NC}"
else
    echo "${RED}✗ FAILED (HTTP $HTTP_CODE)${NC}"
    exit 1
fi

# Response time check
echo -n "Response time... "
RESPONSE_TIME=$(curl -s -w "%{time_total}" -o /dev/null ${APP_URL}/api/v1/metrics/history?type=cpu\&duration=1h)
RESPONSE_MS=$(echo "$RESPONSE_TIME * 1000" | bc)
echo "${RESPONSE_MS}ms"
if (( $(echo "$RESPONSE_TIME < 1.0" | bc -l) )); then
    echo "${GREEN}✓ Fast (<1s)${NC}"
else
    echo "${YELLOW}⚠️  Slow (>1s)${NC}"
fi

# Redis check (if enabled)
if [[ "$REDIS_ENABLED" == "true" ]]; then
    echo -n "Redis connectivity... "
    if redis-cli -h ${REDIS_HOST} -p ${REDIS_PORT} ping &>/dev/null; then
        echo "${GREEN}✓ Connected${NC}"
    else
        echo "${YELLOW}⚠️  Cannot verify${NC}"
    fi
fi

echo ""
echo "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo "${GREEN}║  ✅ Deployment Complete!                                  ║${NC}"
echo "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "📋 Next Steps:"
echo ""
echo "  1. Monitor logs for 30 minutes"
echo "     ${YELLOW}tail -f /var/log/monitoring-dashboard/app.log${NC}"
echo ""
echo "  2. Run load tests"
echo "     ${YELLOW}cd monitoring-dashboard-api${NC}"
echo "     ${YELLOW}./scripts/load/run_k6.sh smoke ./scripts/load/config/${ENVIRONMENT}-external.json${NC}"
echo ""
echo "  3. Check metrics"
echo "     - Response times (should be <500ms P95)"
echo "     - Cache hit rate (should be >70%)"
echo "     - Database connections (should be <100)"
echo "     - Error rate (should be <1%)"
echo ""
echo "  4. Review documentation"
echo "     - DEPLOYMENT_CHECKLIST.md - Full checklist"
echo "     - PERFORMANCE_OPTIMIZATIONS.md - Complete guide"
echo ""
echo "📊 Expected Improvements:"
echo "  • Max RPS: 2-3 → 100+ (40x)"
echo "  • P95 Time: 3000ms → <500ms (6x faster)"
echo "  • Error Rate: 14% → <1% (14x better)"
echo "  • DB Load: -80% reduction"
echo ""
echo "🔧 Rollback command (if needed):"
echo "  ${RED}git revert 0a25111 && ./DEPLOY_NOW.sh ${ENVIRONMENT}${NC}"
echo ""
echo "${GREEN}Deployment successful! 🎉${NC}"
echo ""
