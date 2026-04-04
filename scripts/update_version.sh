#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

MC_VERSION="${1:-1.21.4}"
DATA_DIR="${PROJECT_ROOT}/.mcdata/${MC_VERSION}"

PRISMARINE_REPO="PrismarineJS/minecraft-data"
PRISMARINE_BRANCH="master"
MCMETA_REPO="misode/mcmeta"
MCMETA_TAG="${MC_VERSION}-data-json"

PRISMARINE_BASE="https://raw.githubusercontent.com/${PRISMARINE_REPO}/${PRISMARINE_BRANCH}/data/pc/${MC_VERSION}"
MCMETA_BASE="https://raw.githubusercontent.com/${MCMETA_REPO}/refs/tags/${MCMETA_TAG}"
MCMETA_API="https://api.github.com/repos/${MCMETA_REPO}/contents/data/minecraft"

PRISMARINE_FILES=(
    "blocks.json"
    "items.json"
    "entities.json"
    "protocol.json"
    "biomes.json"
    "effects.json"
    "enchantments.json"
    "foods.json"
    "particles.json"
    "recipes.json"
    "sounds.json"
    "tints.json"
    "materials.json"
    "blockCollisionShapes.json"
    "language.json"
    "version.json"
)

echo "================================================"
echo "  Vitis Data Generator"
echo "  Minecraft ${MC_VERSION}"
echo "================================================"
echo ""

cd "${PROJECT_ROOT}"
mkdir -p "${DATA_DIR}"

echo "=== Step 1: Download PrismarineJS data ==="
for file in "${PRISMARINE_FILES[@]}"; do
    url="${PRISMARINE_BASE}/${file}"
    out="${DATA_DIR}/${file}"
    printf "  %-30s" "${file}"
    if curl -sfL "${url}" -o "${out}" 2>/dev/null; then
        size=$(wc -c < "${out}")
        echo "✓ (${size} bytes)"
    else
        echo "✗ (not available)"
        rm -f "${out}"
    fi
done

echo ""
echo "=== Step 2: Generate registries.json from server JAR ==="

SERVER_JAR="${PROJECT_ROOT}/.mc-decompiled/downloads/${MC_VERSION}/server.jar"
if [ ! -f "${SERVER_JAR}" ]; then
    echo "  Server JAR not found, will be downloaded in Step 6"
    echo "  Deferring registries.json generation..."
    NEED_REGISTRIES=true
else
    DATAGEN_DIR=$(mktemp -d)
    printf "  %-30s" "registries.json"
    if java -DbundlerMainClass="net.minecraft.data.Main" -jar "${SERVER_JAR}" --reports --output "${DATAGEN_DIR}" > /dev/null 2>&1; then
        if [ -f "${DATAGEN_DIR}/reports/registries.json" ]; then
            cp "${DATAGEN_DIR}/reports/registries.json" "${DATA_DIR}/registries.json"
            size=$(wc -c < "${DATA_DIR}/registries.json")
            echo "✓ (${size} bytes)"
        else
            echo "✗ (reports/registries.json not found in output)"
        fi
    else
        echo "✗ (data generator failed)"
    fi
    rm -rf "${DATAGEN_DIR}"
    NEED_REGISTRIES=false
fi

echo ""
echo "=== Step 3: Download misode/mcmeta datapacks ==="

REGISTRIES=(
    "banner_pattern"
    "chat_type"
    "damage_type"
    "dimension_type"
    "enchantment"
    "instrument"
    "jukebox_song"
    "painting_variant"
    "trim_material"
    "trim_pattern"
    "wolf_variant"
    "worldgen/biome"
    "worldgen/structure"
    "worldgen/noise"
    "worldgen/configured_feature"
    "worldgen/placed_feature"
    "worldgen/noise_settings"
)

download_registry() {
    local registry="$1"
    local out_path="${DATA_DIR}/datapacks/${registry}"
    mkdir -p "${out_path}"

    local api_url="${MCMETA_API}/${registry}?ref=${MCMETA_TAG}"
    local files
    files=$(curl -sL "${api_url}" 2>/dev/null | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    if isinstance(data, list):
        for item in data:
            if item['name'].endswith('.json'):
                print(item['name'])
except: pass
" 2>/dev/null || true)

    local count=0
    for f in ${files}; do
        local url="${MCMETA_BASE}/data/minecraft/${registry}/${f}"
        curl -sfL "${url}" -o "${out_path}/${f}" 2>/dev/null && ((count++)) || true
    done
    echo "  ${registry}: ${count} files"
}

for reg in "${REGISTRIES[@]}"; do
    download_registry "${reg}"
done

echo ""
echo "=== Step 4: Download tags ==="

TAGS_DIR="${DATA_DIR}/tags"
mkdir -p "${TAGS_DIR}"

TAG_REGISTRIES=(
    "block"
    "entity_type"
    "fluid"
    "item"
)

download_tags_recursive() {
    local api_path="$1"
    local out_base="$2"

    local items
    items=$(curl -sL "${MCMETA_API}/${api_path}?ref=${MCMETA_TAG}" 2>/dev/null || true)

    echo "${items}" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    if isinstance(data, list):
        for item in data:
            print(item['type'], item['name'])
except: pass
" 2>/dev/null | while read -r item_type item_name; do
        if [ "${item_type}" = "file" ] && [[ "${item_name}" == *.json ]]; then
            local url="${MCMETA_BASE}/data/minecraft/${api_path}/${item_name}"
            mkdir -p "${out_base}"
            curl -sfL "${url}" -o "${out_base}/${item_name}" 2>/dev/null || true
        elif [ "${item_type}" = "dir" ]; then
            download_tags_recursive "${api_path}/${item_name}" "${out_base}/${item_name}"
        fi
    done
}

for reg in "${TAG_REGISTRIES[@]}"; do
    echo "  tags/${reg}"
    download_tags_recursive "tags/${reg}" "${TAGS_DIR}/${reg}"
done

echo ""
echo "=== Step 5: Decompile Minecraft JAR (if needed) ==="

DECOMPILED_DIR="${PROJECT_ROOT}/.mc-decompiled/${MC_VERSION}-decompiled"
if [ -d "${DECOMPILED_DIR}" ]; then
    echo "  Decompiled code already exists: ${DECOMPILED_DIR}"
else
    echo "  Decompiling Minecraft ${MC_VERSION}..."
    
    WORK_DIR="${PROJECT_ROOT}/.mc-decompiled"
    DECOMPILER_VERSION="3.3.2"
    DECOMPILER_JAR="MinecraftDecompiler.jar"
    DECOMPILER_URL="https://github.com/MaxPixelStudios/MinecraftDecompiler/releases/download/v${DECOMPILER_VERSION}/${DECOMPILER_JAR}"
    
    mkdir -p "${WORK_DIR}"
    cd "${WORK_DIR}"
    
    if [ ! -f "${DECOMPILER_JAR}" ]; then
        echo "  Downloading MinecraftDecompiler v${DECOMPILER_VERSION}..."
        curl -L -o "${DECOMPILER_JAR}" "${DECOMPILER_URL}"
    fi
    
    java -jar "${DECOMPILER_JAR}" \
        --version "${MC_VERSION}" \
        --side SERVER \
        --decompile vineflower \
        --output "${MC_VERSION}-remapped.jar" \
        --decompiled-output "${MC_VERSION}-decompiled"
    
    cd "${PROJECT_ROOT}"
fi

echo ""
echo "=== Step 6: Extract loot tables from server.jar ==="
"${SCRIPT_DIR}/extract_loot_tables.sh" "${MC_VERSION}"

if [ "${NEED_REGISTRIES:-false}" = "true" ]; then
    echo ""
    echo "=== Step 6b: Generate registries.json (deferred) ==="
    SERVER_JAR="${PROJECT_ROOT}/.mc-decompiled/downloads/${MC_VERSION}/server.jar"
    if [ -f "${SERVER_JAR}" ]; then
        DATAGEN_DIR=$(mktemp -d)
        printf "  %-30s" "registries.json"
        if java -DbundlerMainClass="net.minecraft.data.Main" -jar "${SERVER_JAR}" --reports --output "${DATAGEN_DIR}" > /dev/null 2>&1; then
            if [ -f "${DATAGEN_DIR}/reports/registries.json" ]; then
                cp "${DATAGEN_DIR}/reports/registries.json" "${DATA_DIR}/registries.json"
                size=$(wc -c < "${DATA_DIR}/registries.json")
                echo "✓ (${size} bytes)"
            else
                echo "✗ (reports/registries.json not found)"
            fi
        else
            echo "✗ (data generator failed)"
        fi
        rm -rf "${DATAGEN_DIR}"
    else
        echo "  ✗ Server JAR still not available after Step 6"
    fi
fi

echo ""
echo "=== Step 7: Run Go parsers for decompiled data ==="

PARSERS_DIR="${PROJECT_ROOT}/scripts/parsers"
if [ -d "${PARSERS_DIR}" ]; then
    for parser in "${PARSERS_DIR}"/*.go; do
        if [ -f "${parser}" ]; then
            name=$(basename "${parser}" .go)
            echo "  Running parser: ${name}"
            go run "${parser}" -version "${MC_VERSION}" || echo "    ✗ Parser failed"
        fi
    done
else
    echo "  No parsers found in ${PARSERS_DIR}"
fi

echo ""
echo "=== Step 8: Run all generators ==="

run_generator() {
    local name="$1"
    local path="$2"
    printf "  %-25s" "${name}"
    if go run "${path}" -version "${MC_VERSION}" 2>/dev/null; then
        echo "✓"
    else
        echo "✗"
    fi
}

GENERATORS=(
    "block:internal/data/generator/block/main.go"
    "item:internal/data/generator/item/main.go"
    "entity:internal/data/generator/entity/main.go"
    "sound:internal/data/generator/sound/main.go"
    "particle:internal/data/generator/particle/main.go"
    "effect:internal/data/generator/effect/main.go"
    "attribute:internal/data/generator/attribute/main.go"
    "game_event:internal/data/generator/game_event/main.go"
    "fluid:internal/data/generator/fluid/main.go"
    "potion:internal/data/generator/potion/main.go"
    "screen:internal/data/generator/screen/main.go"
    "damage_type:internal/data/generator/damage_type/main.go"
    "dimension:internal/data/generator/dimension/main.go"
    "enchantment:internal/data/generator/enchantment/main.go"
    "jukebox_song:internal/data/generator/jukebox_song/main.go"
    "biome:internal/data/generator/biome/main.go"
    "spawn_egg:internal/data/generator/spawn_egg/main.go"
    "entity_pose:internal/data/generator/entity_pose/main.go"
    "chunk_status:internal/data/generator/chunk_status/main.go"
    "sound_category:internal/data/generator/sound_category/main.go"
    "meta_data_type:internal/data/generator/meta_data_type/main.go"
    "world_event:internal/data/generator/world_event/main.go"
    "recipes:internal/data/generator/recipes/main.go"
    "painting_variant:internal/data/generator/painting_variant/main.go"
    "wolf_variant:internal/data/generator/wolf_variant/main.go"
    "instrument:internal/data/generator/instrument/main.go"
    "trim_pattern:internal/data/generator/trim_pattern/main.go"
    "trim_material:internal/data/generator/trim_material/main.go"
    "banner_pattern:internal/data/generator/banner_pattern/main.go"
    "data_component:internal/data/generator/data_component/main.go"
    "block_entity:internal/data/generator/block_entity/main.go"
    "villager_profession:internal/data/generator/villager_profession/main.go"
    "cat_variant:internal/data/generator/cat_variant/main.go"
    "frog_variant:internal/data/generator/frog_variant/main.go"
    "villager_type:internal/data/generator/villager_type/main.go"
    "scoreboard_slot:internal/data/generator/scoreboard_slot/main.go"
    "game_rules:internal/data/generator/game_rules/main.go"
    "registry:internal/data/generator/registry/main.go"
    "tag:internal/data/generator/tag/main.go"
    "packet:internal/data/generator/packet/main.go"
    "entity_status:internal/data/generator/entity_status/main.go"
    "structures:internal/data/generator/structures/main.go"
    "tracked_data:internal/data/generator/tracked_data/main.go"
    "translation:internal/data/generator/translation/main.go"
    "composter:internal/data/generator/composter/main.go"
    "flower_pot:internal/data/generator/flower_pot/main.go"
    "fuels:internal/data/generator/fuels/main.go"
    "smelting:internal/data/generator/smelting/main.go"
    "status_effect:internal/data/generator/status_effect/main.go"
    "potion_brewing:internal/data/generator/potion_brewing/main.go"
    "noise_parameter:internal/data/generator/noise_parameter/main.go"
    "configured_features:internal/data/generator/configured_features/main.go"
    "placed_features:internal/data/generator/placed_features/main.go"
    "chunk_gen_settings:internal/data/generator/chunk_gen_settings/main.go"
    "block_state_remap:internal/data/generator/block_state_remap/main.go"
    "entity_id_remap:internal/data/generator/entity_id_remap/main.go"
    "item_id_remap:internal/data/generator/item_id_remap/main.go"
    "recipe_remainder:internal/data/generator/recipe_remainder/main.go"
    "collision_shapes:internal/data/generator/collision/shapes/main.go"
)

for gen in "${GENERATORS[@]}"; do
    name="${gen%%:*}"
    path="${gen#*:}"
    if [ -f "${PROJECT_ROOT}/${path}" ]; then
        run_generator "${name}" "${path}"
    fi
done

echo ""
echo "=== Step 9: Verify build ==="
if go build ./...; then
    echo "  ✓ Build successful"
else
    echo "  ✗ Build failed"
    exit 1
fi

echo ""
echo "================================================"
echo "  Update complete!"
echo "  Data directory: ${DATA_DIR}"
echo "================================================"
