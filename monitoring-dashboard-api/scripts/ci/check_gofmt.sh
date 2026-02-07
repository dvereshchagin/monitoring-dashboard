#!/usr/bin/env bash
set -euo pipefail

UNFORMATTED="$(gofmt -l . | grep -v '^vendor/' || true)"
if [[ -n "${UNFORMATTED}" ]]; then
  echo "The following files need gofmt:"
  echo "${UNFORMATTED}"
  exit 1
fi

