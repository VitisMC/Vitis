#!/bin/bash
set -euo pipefail

REPO="PrismarineJS/minecraft-data"
BRANCH="master"
FILE_PATH="data/pc/1.21.4/protocol.json"
RAW_URL="https://raw.githubusercontent.com/${REPO}/${BRANCH}/${FILE_PATH}"
OUT_DIR=".mc1214-data/1.21.4"
OUT_FILE="${OUT_DIR}/protocol.json"

cd "$(dirname "$0")/.."

mkdir -p "${OUT_DIR}"

echo "Downloading protocol.json for 1.21.4 from PrismarineJS/minecraft-data..."
curl -sL "${RAW_URL}" -o "${OUT_FILE}"

echo "Saved to ${OUT_FILE}"
echo "File size: $(wc -c < "${OUT_FILE}") bytes"
