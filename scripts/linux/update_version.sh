#!/usr/bin/env bash
# Prepare official sources and regenerate all checked-in Minecraft data transactionally.
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
project_root="$(dirname "$(dirname "$script_dir")")"
version="${1:-}"
if [[ $# != 1 || ! "$version" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ || "$version" == *..* ]]; then
  echo "usage: $0 <minecraft-version>" >&2
  exit 2
fi
for tool in curl jq sha1sum sha256sum java javac go find sort xargs awk sed cmp cp make git; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "missing required command: $tool" >&2
    exit 1
  fi
done
if (( BASH_VERSINFO[0] < 4 )); then
  echo "generation requires Bash 4 or newer" >&2
  exit 1
fi
java_version="$(java -version 2>&1 | sed -nE 's/.*version "([^"]+)".*/\1/p')"
java_major="$(java -version 2>&1 | sed -nE 's/.*version "([0-9]+).*/\1/p' | head -n1)"
javac_major="$(javac -version 2>&1 | sed -nE 's/^javac ([0-9]+).*/\1/p')"
if [[ "$java_major" != 21 || "$javac_major" != 21 ]]; then
  echo "Minecraft generation requires java and javac from JDK 21 on PATH" >&2
  exit 1
fi

# Bash and PowerShell use one pinned toolchain and ordered output inventory.
generation_manifest="$script_dir/../generation_manifest.json"
decompiler_version="$(jq -er '.decompiler.version' "$generation_manifest")"
decompiler_sha256="$(jq -er '.decompiler.sha256' "$generation_manifest")"
vineflower_version="$(jq -er '.vineflower.version' "$generation_manifest")"
vineflower_sha256="$(jq -er '.vineflower.sha256' "$generation_manifest")"
cache_root="$project_root/.mc-decompiled"
cache_dir="$cache_root/downloads/$version"
tools_dir="$cache_root/downloads/tools"
data_dir="$project_root/.mcdata/$version"
mkdir -p "$project_root/.mcdata" "$cache_dir" "$tools_dir"
generation_lock="$project_root/.mcdata/.generate-lock"
if ! mkdir "$generation_lock"; then
  echo "another generation is active; after an ungraceful stop, inspect .generation-backup.* before removing $generation_lock" >&2
  exit 1
fi
source "$script_dir/generation_transaction.sh"
install_generation_traps
scratch_dir="$(mktemp -d "${TMPDIR:-/tmp}/vitis-generation.XXXXXX")"
stage_dir="$(mktemp -d "$project_root/.mcdata/.${version}.stage.XXXXXX")"
source_stage="$(mktemp -d "$cache_root/.${version}.sources.XXXXXX")"
cd "$project_root"
export GOCACHE="${GOCACHE:-/tmp/vitis-go-build}"

# download stages bounded-time HTTPS transfers and never publishes an unverified checked artifact.
download() {
  local url="$1" destination="$2" algorithm="${3:-}" expected="${4:-}" size="${5:-}" temporary actual
  if [[ -n "$algorithm" && -f "$destination" ]]; then
    actual="$("${algorithm}sum" "$destination" | awk '{print $1}')"
    if [[ "$actual" == "$expected" && ( -z "$size" || "$(wc -c <"$destination")" -eq "$size" ) ]]; then return; fi
  fi
  temporary="$(mktemp "${destination}.partial.XXXXXX")"
  generation_partials+=("$temporary")
  run_generation_command curl --fail --location --silent --show-error --retry 3 \
    --proto '=https' --proto-redir '=https' --connect-timeout 15 --max-time 180 "$url" --output "$temporary"
  if [[ ! -s "$temporary" ]]; then echo "empty download: $url" >&2; return 1; fi
  if [[ -n "$size" && "$(wc -c <"$temporary")" -ne "$size" ]]; then echo "size mismatch: $url" >&2; return 1; fi
  if [[ -n "$algorithm" ]]; then
    actual="$("${algorithm}sum" "$temporary" | awk '{print $1}')"
    if [[ "$actual" != "$expected" ]]; then echo "checksum mismatch: $url" >&2; return 1; fi
  fi
  mv -- "$temporary" "$destination"
}

manifest_file="$scratch_dir/manifest.json"
metadata_file="$stage_dir/mojang-version.json"
download https://piston-meta.mojang.com/mc/game/version_manifest_v2.json "$manifest_file"
version_url="$(jq -er --arg version "$version" '.versions[] | select(.id == $version) | .url' "$manifest_file")"
download "$version_url" "$metadata_file"
jq -e --arg version "$version" '.id == $version' "$metadata_file" >/dev/null
server_sha1="$(jq -er '.downloads.server.sha1' "$metadata_file")"
mappings_sha1="$(jq -er '.downloads.server_mappings.sha1' "$metadata_file")"
if [[ ! "$server_sha1" =~ ^[0-9a-f]{40}$ || ! "$mappings_sha1" =~ ^[0-9a-f]{40}$ ]]; then echo "invalid official checksums" >&2; exit 1; fi
server_jar="$cache_dir/server.jar"
mappings_file="$cache_dir/server_mappings.txt"
download "$(jq -er '.downloads.server.url' "$metadata_file")" "$server_jar" sha1 "$server_sha1" "$(jq -er '.downloads.server.size' "$metadata_file")"
download "$(jq -er '.downloads.server_mappings.url' "$metadata_file")" "$mappings_file" sha1 "$mappings_sha1" "$(jq -er '.downloads.server_mappings.size' "$metadata_file")"
decompiler_jar="$tools_dir/MinecraftDecompiler-$decompiler_version.jar"
vineflower_jar="$tools_dir/vineflower-$vineflower_version.jar"
download "$(jq -er '.decompiler.url' "$generation_manifest")" "$decompiler_jar" sha256 "$decompiler_sha256"
download "$(jq -er '.vineflower.url' "$generation_manifest")" "$vineflower_jar" sha256 "$vineflower_sha256"

printf 'minecraft_version=%s\nserver_sha1=%s\nmappings_sha1=%s\ndecompiler_sha256=%s\nvineflower_sha256=%s\njdk_version=%s\noptions=%s\n' \
  "$version" "$server_sha1" "$mappings_sha1" "$decompiler_sha256" "$vineflower_sha256" "$java_version" "$(jq -er '.source_options' "$generation_manifest")" >"$source_stage/$version.sources.lock"
source_cached=0
if [[ -f "$cache_root/$version.sources.lock" && -f "$cache_root/$version.sources.sha256" ]] \
  && cmp -s "$source_stage/$version.sources.lock" "$cache_root/$version.sources.lock" \
  && ! find "$cache_root/$version-decompiled" -type l -print -quit | read -r source_link \
  && (cd "$cache_root"; find "$version-remapped.jar" "$version-decompiled" -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) >"$scratch_dir/sources.sha256" \
  && cmp -s "$scratch_dir/sources.sha256" "$cache_root/$version.sources.sha256"; then
  source_cached=1
  echo "using verified named sources and remapped JAR for $version"
else
  echo "preparing named sources and remapped JAR for $version"
  run_generation_command go run ./cmd/vitis-data -unpack -version "$version" -jar "$server_jar" -output "$source_stage/bundle"
  mkdir -p "$source_stage/work"
  library_args=()
  while IFS= read -r -d '' library; do library_args+=("-e=$library"); done \
    < <(find "$source_stage/bundle/libraries" -name '*.jar' -type f -print0 | LC_ALL=C sort -z)
  cd "$source_stage/work"
  # The ProGuard reader already maps obfuscated names to Mojang names.
  run_generation_command java -Xmx2G -XX:ActiveProcessorCount=4 -jar "$decompiler_jar" \
    --input "$source_stage/bundle/server.jar" --map "$mappings_file" \
    --output "$source_stage/$version-remapped.jar" --temp "$source_stage/work/temporary"
  run_generation_command java -Xmx2G -XX:ActiveProcessorCount=4 -jar "$vineflower_jar" \
    --folder -thr=4 -jrt=1 -sef=1 -log=WARN "${library_args[@]}" \
    "$source_stage/$version-remapped.jar" "$source_stage/$version-decompiled"
  cd "$project_root"
  test -s "$source_stage/$version-remapped.jar"
  mapfile -t required_sources < <(jq -er '.required_sources[]' "$generation_manifest")
  for required in "${required_sources[@]}"; do
    test -s "$source_stage/$version-decompiled/$required"
  done
  (cd "$source_stage"; find "$version-remapped.jar" "$version-decompiled" -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) >"$source_stage/$version.sources.sha256"
fi

# Preserve an existing source lock; a fresh 1.21.4 checkout uses the validated pinned commit.
minecraft_data_commit=""
if [[ -s "$data_dir/source.lock" ]]; then
  minecraft_data_commit="$(awk -F= '$1 == "minecraft_data_commit" {print $2}' "$data_dir/source.lock")"
fi
if [[ -z "$minecraft_data_commit" ]]; then minecraft_data_commit="$(jq -r --arg version "$version" '.minecraft_data_commits[$version] // ""' "$generation_manifest")"; fi
if [[ -z "$minecraft_data_commit" ]]; then
  download "https://api.github.com/repos/PrismarineJS/minecraft-data/commits?path=data/pc/$version&per_page=1" "$scratch_dir/commits.json"
  minecraft_data_commit="$(jq -er '.[0].sha' "$scratch_dir/commits.json")"
fi
if [[ ! "$minecraft_data_commit" =~ ^[0-9a-f]{40}$ ]]; then echo "invalid minecraft-data commit" >&2; exit 1; fi
source_url="https://raw.githubusercontent.com/PrismarineJS/minecraft-data/$minecraft_data_commit/data"
download "$source_url/dataPaths.json" "$stage_dir/dataPaths.json"
mapfile -t prismarine_files < <(jq -er '.prismarine_files[]' "$generation_manifest")
for name in "${prismarine_files[@]}"; do
  source_path="$(jq -er --arg version "$version" --arg name "$name" '.pc[$version][$name]' "$stage_dir/dataPaths.json")"
  if [[ ! "$source_path" =~ ^pc/[A-Za-z0-9._-]+$ ]]; then echo "invalid minecraft-data path" >&2; exit 1; fi
  download "$source_url/$source_path/$name.json" "$stage_dir/$name.json"
  jq -e . "$stage_dir/$name.json" >/dev/null
done
jq -e --arg version "$version" '.minecraftVersion == $version' "$stage_dir/version.json" >/dev/null
printf 'minecraft_data_commit=%s\nserver_sha1=%s\nmappings_sha1=%s\ndecompiler_sha256=%s\nvineflower_sha256=%s\n' \
  "$minecraft_data_commit" "$server_sha1" "$mappings_sha1" "$decompiler_sha256" "$vineflower_sha256" >"$stage_dir/source.lock"
run_generation_command go run ./cmd/vitis-data -version "$version" -jar "$server_jar" -output "$stage_dir/vanilla"
jq -e --argjson protocol "$(jq -er ' .protocol_version' "$stage_dir/vanilla/version.json")" ' .version == $protocol' "$stage_dir/version.json" >/dev/null
cp "$stage_dir/vanilla/reports/registries.json" "$stage_dir/registries.json"
mkdir -p "$stage_dir/datapacks" "$stage_dir/tags" "$stage_dir/loot_tables" "$stage_dir/recipes"
cp -a "$stage_dir/vanilla/data/minecraft/." "$stage_dir/datapacks/"
cp -a "$stage_dir/vanilla/data/minecraft/tags/." "$stage_dir/tags/"
cp -a "$stage_dir/vanilla/data/minecraft/loot_table/." "$stage_dir/loot_tables/"
cp -a "$stage_dir/vanilla/data/minecraft/recipe/." "$stage_dir/recipes/"
resource_count="$(find "$stage_dir/vanilla/data/minecraft" -type f | wc -l)"
recipe_count="$(find "$stage_dir/recipes" -type f | wc -l)"
loot_count="$(find "$stage_dir/loot_tables" -type f | wc -l)"
advancement_count="$(find "$stage_dir/vanilla/data/minecraft/advancement" -type f | wc -l)"
structure_count="$(find "$stage_dir/vanilla/data/minecraft/structure" -type f | wc -l)"
if (( resource_count < 5000 || recipe_count < 1000 || loot_count < 1000 || advancement_count < 1000 || structure_count < 1000 )); then
  echo "incomplete generated resources" >&2; exit 1
fi
(cd "$stage_dir"; find . -type f ! -name checksums.sha256 -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) >"$stage_dir/checksums.sha256"
(cd "$stage_dir"; sha256sum --check --strict checksums.sha256 >/dev/null)

mapfile -t generators < <(jq -er '.generators[]' "$generation_manifest")
# The explicit order puts block/biome inputs before their JVM and Go consumers.
mapfile -t actual_generators < <(find internal/data/generator -name main.go -type f | LC_ALL=C sort)
if (( ${#actual_generators[@]} != ${#generators[@]} )); then
  echo "generator inventory changed; update the explicit commands and output inventory" >&2; exit 1
fi
for generator in "${generators[@]}"; do test -f "internal/data/generator/$generator/main.go"; done
mapfile -t generated_go < <(jq -er '.generated_go[]' "$generation_manifest")
mapfile -t fixtures < <(jq -er '.fixtures[]' "$generation_manifest")
outputs=(".mcdata/$version" internal/data/generated "${generated_go[@]}" "${fixtures[@]}")
if (( source_cached == 0 )); then
  outputs+=(".mc-decompiled/$version-remapped.jar" ".mc-decompiled/$version-decompiled" ".mc-decompiled/$version.sources.lock" ".mc-decompiled/$version.sources.sha256")
fi
snapshot_generation_outputs "${outputs[@]}"
generation_publishing=1
if (( source_cached == 0 )); then
  for artifact in "$version-remapped.jar" "$version-decompiled" "$version.sources.lock" "$version.sources.sha256"; do
    rm -rf -- "$cache_root/$artifact"
    mv -- "$source_stage/$artifact" "$cache_root/$artifact"
  done
fi
rm -rf -- "$data_dir"
mv -- "$stage_dir" "$data_dir"
stage_dir=""
for generator in "${generators[@]}"; do
  if [[ "$generator" == worldgen ]]; then
    run_generation_command go run ./internal/data/generator/worldgen -version "$version" -resources "$data_dir/vanilla/data/minecraft/worldgen"
  else
    run_generation_command go run "internal/data/generator/$generator/main.go" -version "$version"
  fi
done
mapfile -t generated_files < <(find internal/data/generated -name '*.go' -type f | LC_ALL=C sort)
run_generation_command gofmt -w "${generated_files[@]}" "${generated_go[@]}"
run_generation_command make verify
run_generation_command git diff --check
echo "data generation completed for $version: resources=$resource_count recipes=$recipe_count loot=$loot_count"
