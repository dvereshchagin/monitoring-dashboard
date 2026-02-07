#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="/opt/monitoring-dashboard"
API_DIR="${BASE_DIR}/monitoring-dashboard-api"

mkdir -p "${BASE_DIR}"
mv "${API_DIR}/deploy/docker-compose.prod.yml" "${BASE_DIR}/docker-compose.yml"

mkdir -p "${BASE_DIR}/migrations"
cp -R "${API_DIR}/internal/infrastructure/persistence/postgres/migrations/"* "${BASE_DIR}/migrations/"

cat > "${BASE_DIR}/.env" <<EOF
IMAGE=${IMAGE}
APP_PORT=${APP_PORT:-8080}
DB_HOST=${DB_HOST:-db}
DB_PORT=${DB_PORT:-5432}
DB_USER=${DB_USER}
DB_PASSWORD=${DB_PASSWORD}
DB_NAME=${DB_NAME}
LOG_LEVEL=${LOG_LEVEL:-info}
METRICS_COLLECTION_INTERVAL=${METRICS_COLLECTION_INTERVAL:-2s}
METRICS_RETENTION_DAYS=${METRICS_RETENTION_DAYS:-7}
ALLOWED_ORIGINS=${ALLOWED_ORIGINS:-}
AUTH_ENABLED=${AUTH_ENABLED:-false}
AUTH_BEARER_TOKEN=${AUTH_BEARER_TOKEN}
S3_ENABLED=${S3_ENABLED:-false}
S3_BUCKET=${S3_BUCKET:-}
S3_REGION=${S3_REGION:-ru-central1}
S3_ENDPOINT=${S3_ENDPOINT:-https://storage.yandexcloud.net}
S3_ACCESS_KEY_ID=${S3_ACCESS_KEY_ID:-}
S3_SECRET_ACCESS_KEY=${S3_SECRET_ACCESS_KEY:-}
S3_USE_PATH_STYLE=${S3_USE_PATH_STYLE:-true}
S3_KEY_PREFIX=${S3_KEY_PREFIX:-dashboards}
S3_URL_MODE=${S3_URL_MODE:-presigned}
S3_PRESIGNED_TTL=${S3_PRESIGNED_TTL:-5m}
EOF

echo "${GHCR_TOKEN}" | docker login ghcr.io -u "${GHCR_USERNAME}" --password-stdin

cd "${BASE_DIR}"
docker compose up -d db

for _ in $(seq 1 20); do
  if docker compose exec -T db pg_isready -U "${DB_USER}" -d "${DB_NAME}" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

COMPOSE_NETWORK="$(basename "${BASE_DIR}")_default"
GOOSE_DSN="host=db port=${DB_PORT:-5432} user=${DB_USER} password=${DB_PASSWORD} dbname=${DB_NAME} sslmode=disable"

docker run --rm \
  --network "${COMPOSE_NETWORK}" \
  -e GOOSE_DSN="${GOOSE_DSN}" \
  -v "${BASE_DIR}/migrations:/migrations" \
  golang:1.25-bookworm \
  bash -lc 'set -euo pipefail; /usr/local/go/bin/go install github.com/pressly/goose/v3/cmd/goose@latest; /go/bin/goose -dir /migrations postgres "${GOOSE_DSN}" up'

docker compose pull
docker compose up -d
docker image prune -f
