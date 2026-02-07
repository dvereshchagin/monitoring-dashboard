#!/usr/bin/env bash
set -euo pipefail

if ! command -v goose >/dev/null 2>&1; then
  echo "goose is not installed"
  exit 1
fi

DB_PASSWORD_VALUE="${DB_PASSWORD:-${PGPASSWORD:-}}"
if [[ -z "${DB_PASSWORD_VALUE}" ]]; then
  echo "DB_PASSWORD (or PGPASSWORD) is required"
  exit 1
fi

GOOSE_DSN="host=${DB_HOST:-localhost} port=${DB_PORT:-5432} user=${DB_USER:-postgres} password=${DB_PASSWORD_VALUE} dbname=${DB_NAME:-monitoring} sslmode=${DB_SSLMODE:-disable}"

goose -dir internal/infrastructure/persistence/postgres/migrations postgres "${GOOSE_DSN}" up
