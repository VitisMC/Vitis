#!/bin/bash
set -euo pipefail

REPO="misode/mcmeta"
TAG="1.21.4-data-json"
BASE_URL="https://api.github.com/repos/${REPO}/contents/data/minecraft"
RAW_BASE="https://raw.githubusercontent.com/${REPO}/refs/tags/${TAG}/data/minecraft"
OUT_DIR=".mc1214-data/1.21.4/datapacks"

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
)

WORLDGEN_REGISTRIES=(
  "worldgen/biome"
)

TAG_REGISTRIES=(
  "banner_pattern"
  "block"
  "cat_variant"
  "damage_type"
  "enchantment"
  "entity_type"
  "fluid"
  "game_event"
  "instrument"
  "item"
  "painting_variant"
  "point_of_interest_type"
)

TAG_WORLDGEN_REGISTRIES=(
  "worldgen/biome"
  "worldgen/flat_level_generator_preset"
  "worldgen/structure"
  "worldgen/world_preset"
)

download_registry() {
  local registry="$1"
  local out_path="${OUT_DIR}/${registry}"
  mkdir -p "${out_path}"

  echo "Fetching list for ${registry}..."
  local api_url="${BASE_URL}/${registry}?ref=${TAG}"
  local files
  files=$(curl -sL "${api_url}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
if isinstance(data, list):
    for item in data:
        if item['name'].endswith('.json'):
            print(item['name'])
")

  for f in ${files}; do
    local url="${RAW_BASE}/${registry}/${f}"
    echo "  Downloading ${registry}/${f}"
    curl -sL "${url}" -o "${out_path}/${f}"
  done
}

download_tags_recursive() {
  local api_path="$1"
  local out_base="$2"

  local items
  items=$(curl -sL "${BASE_URL}/${api_path}?ref=${TAG}" 2>/dev/null)

  echo "${items}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
if isinstance(data, list):
    for item in data:
        print(item['type'], item['name'])
" 2>/dev/null | while read -r item_type item_name; do
    if [ "${item_type}" = "file" ] && [[ "${item_name}" == *.json ]]; then
      local url="${RAW_BASE}/${api_path}/${item_name}"
      mkdir -p "${out_base}"
      echo "  Downloading tags/${api_path#tags/}/${item_name}"
      curl -sL "${url}" -o "${out_base}/${item_name}"
    elif [ "${item_type}" = "dir" ]; then
      download_tags_recursive "${api_path}/${item_name}" "${out_base}/${item_name}"
    fi
  done
}

cd "$(dirname "$0")/.."

for reg in "${REGISTRIES[@]}"; do
  download_registry "${reg}"
done

for reg in "${WORLDGEN_REGISTRIES[@]}"; do
  download_registry "${reg}"
done

TAGS_OUT="${OUT_DIR}/../tags"
echo ""
echo "Downloading tags..."
for reg in "${TAG_REGISTRIES[@]}"; do
  download_tags_recursive "tags/${reg}" "${TAGS_OUT}/${reg}"
done
for reg in "${TAG_WORLDGEN_REGISTRIES[@]}"; do
  download_tags_recursive "tags/${reg}" "${TAGS_OUT}/${reg}"
done

echo ""
echo "Download complete. Files saved to ${OUT_DIR}/"
echo "Registry counts:"
for reg in "${REGISTRIES[@]}" "${WORLDGEN_REGISTRIES[@]}"; do
  count=$(ls -1 "${OUT_DIR}/${reg}/"*.json 2>/dev/null | wc -l)
  echo "  ${reg}: ${count} entries"
done
echo "Tag directories:"
find "${TAGS_OUT}" -name '*.json' 2>/dev/null | wc -l | xargs -I{} echo "  Total tag files: {}"
