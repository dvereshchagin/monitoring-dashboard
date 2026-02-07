#!/usr/bin/env bash
set -euo pipefail

mkdir -p dist
GOOS=linux GOARCH=amd64 go build -o dist/monitoring-dashboard-linux-amd64 cmd/server/main.go

