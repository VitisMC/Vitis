#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

MC_VERSION="${1:-1.21.4}"
DATA_DIR="${PROJECT_ROOT}/.mcdata/${MC_VERSION}"
LOOT_DIR="${DATA_DIR}/loot_tables/blocks"
CACHE_DIR="${PROJECT_ROOT}/.mc-decompiled/downloads/${MC_VERSION}"

echo "=== Extracting loot tables for Minecraft ${MC_VERSION} ==="

mkdir -p "${LOOT_DIR}"
mkdir -p "${CACHE_DIR}"

SERVER_JAR="${CACHE_DIR}/server.jar"

if [ ! -f "${SERVER_JAR}" ]; then
    echo "  Downloading server.jar..."
    
    MANIFEST_URL="https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"
    VERSION_JSON_URL=$(curl -sL "${MANIFEST_URL}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for v in data['versions']:
    if v['id'] == '${MC_VERSION}':
        print(v['url'])
        break
")
    
    if [ -z "${VERSION_JSON_URL}" ]; then
        echo "  ✗ Version ${MC_VERSION} not found in manifest"
        exit 1
    fi
    
    SERVER_URL=$(curl -sL "${VERSION_JSON_URL}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(data['downloads']['server']['url'])
")
    
    curl -L -o "${SERVER_JAR}" "${SERVER_URL}"
    echo "  ✓ Downloaded server.jar"
else
    echo "  Using cached server.jar"
fi

echo "  Extracting loot tables..."

rm -rf "${LOOT_DIR}"
mkdir -p "${LOOT_DIR}"

cd "${CACHE_DIR}"
rm -rf tmp_extract
mkdir -p tmp_extract

INNER_JAR="META-INF/versions/${MC_VERSION}/server-${MC_VERSION}.jar"
unzip -q -o "${SERVER_JAR}" "${INNER_JAR}" -d tmp_extract 2>/dev/null || true

EXTRACTED_INNER="${CACHE_DIR}/tmp_extract/${INNER_JAR}"
if [ -f "${EXTRACTED_INNER}" ]; then
    unzip -q -o "${EXTRACTED_INNER}" "data/minecraft/loot_table/blocks/*" -d tmp_extract/inner 2>/dev/null || true
    
    if [ -d "tmp_extract/inner/data/minecraft/loot_table/blocks" ]; then
        mv tmp_extract/inner/data/minecraft/loot_table/blocks/* "${LOOT_DIR}/"
        
        count=$(find "${LOOT_DIR}" -name "*.json" | wc -l)
        echo "  ✓ Extracted ${count} block loot tables to ${LOOT_DIR}"
    else
        rm -rf tmp_extract
        echo "  ✗ No loot tables found in inner server jar"
        exit 1
    fi
    
    RECIPE_DIR="${DATA_DIR}/recipes"
    rm -rf "${RECIPE_DIR}"
    mkdir -p "${RECIPE_DIR}"
    
    unzip -q -o "${EXTRACTED_INNER}" "data/minecraft/recipe/*" -d tmp_extract/recipes 2>/dev/null || true
    
    if [ -d "tmp_extract/recipes/data/minecraft/recipe" ]; then
        mv tmp_extract/recipes/data/minecraft/recipe/* "${RECIPE_DIR}/"
        recipe_count=$(find "${RECIPE_DIR}" -name "*.json" | wc -l)
        echo "  ✓ Extracted ${recipe_count} recipes to ${RECIPE_DIR}"
    fi
    
    rm -rf tmp_extract
else
    rm -rf tmp_extract
    echo "  ✗ Could not find inner server jar at ${INNER_JAR}"
    exit 1
fi
