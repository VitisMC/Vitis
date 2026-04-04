#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "${PROJECT_ROOT}"

echo "Starting Vitis..."
go run ./cmd/vitis/main.go -config configs/vitis.yaml "$@"
