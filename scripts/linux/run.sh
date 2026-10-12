#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"

cd "${PROJECT_ROOT}"

go build -o vitis ./cmd/vitis
exec ./vitis "$@"
