#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "${PROJECT_ROOT}"

OUTPUT="${1:-vitis}"

echo "Building Vitis..."
go build -trimpath -ldflags "-s -w" -o "${OUTPUT}" ./cmd/vitis

echo "✓ ${OUTPUT} ($(wc -c < "${OUTPUT}") bytes)"