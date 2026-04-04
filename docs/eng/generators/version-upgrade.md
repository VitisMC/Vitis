# Version Upgrade Guide

Step-by-step instructions for updating Vitis to a new Minecraft version.

## Prerequisites

- Java 21+ (for server JAR data generator and MinecraftDecompiler)
- Go 1.21+
- curl, python3

## The Easy Way

```bash
./scripts/update_version.sh <NEW_VERSION>
```

This single command handles everything: downloads data, generates `registries.json` from the server JAR, downloads datapacks and tags from misode/mcmeta, decompiles the server JAR, extracts loot tables, and runs all 59 generators.

Pass the target Minecraft version as argument (e.g. `1.22`).

## What `update_version.sh` Does

| Step | Description | Source |
|------|-------------|--------|
| 1 | Download PrismarineJS JSON files | `PrismarineJS/minecraft-data` |
| 2 | Generate `registries.json` | Server JAR data generator |
| 3 | Download configuration registry datapacks | `misode/mcmeta` tag `{version}-data-json` |
| 4 | Download tags | `misode/mcmeta` tag `{version}-data-json` |
| 5 | Decompile server JAR | MinecraftDecompiler + Vineflower |
| 6 | Extract loot tables and recipes | Server JAR inner archive |
| 7 | Run Go parsers on decompiled code | `scripts/parsers/*.go` |
| 8 | Run all 59 generators | `internal/data/generator/*/main.go` |
| 9 | Verify build | `go build ./...` |

## After Running the Script

### Check for new registries or datapacks

If the new version adds configuration registries or worldgen types, update the arrays in `update_version.sh`:

- `REGISTRIES` — datapacks to download from mcmeta (Step 3)
- `TAG_REGISTRIES` — tag directories to download (Step 4)
- `GENERATORS` — code generators to run (Step 8)

### Update version constants

- **Protocol version** — check [Minecraft Wiki protocol versions](https://minecraft.wiki/w/Java_Edition_protocol_version)
- **Known Packs version** — in `internal/session/configuration_handler.go`, update `Version: "1.21.4"` to the new version
- **Registry builtin.go** — if new registries were added, add entries to `builtinIDMap()` in `internal/registry/builtin.go`

### Manual verification

1. Run `go test ./...`
2. Start the server and connect with a vanilla client
3. Check for registry/tag errors in logs
4. Verify new/removed packet types and fields

## Troubleshooting

### `registries.json` generation fails

Requires Java 21+. Check that the server JAR was downloaded:
```bash
ls -la .mc-decompiled/downloads/<version>/server.jar
```

If missing, re-run `update_version.sh` — it downloads the JAR in Step 6 (extract_loot_tables.sh).

### Generator shows ✗

Run the generator manually to see the error:
```bash
go run internal/data/generator/<name>/main.go -version <version>
```

Common causes:
- `registries.json` missing → Step 2 failed
- Datapack JSON missing → check the `REGISTRIES` array includes the needed entry
- Decompiled source missing → Step 5 failed (check Java is installed)

### Client crashes with registry errors

- Missing tag data → ensure `TAG_REGISTRIES` array is complete
- Wrong NBT types → check `doubleFields`/`longFields` in `internal/registry/generator/main.go`
- Missing config registry → check `configRegistries` map in the registry generator

### Datapack download shows 0 files

The GitHub API may be rate-limited. Wait or use a `GITHUB_TOKEN`:
```bash
export GITHUB_TOKEN=ghp_...
```

## Version History

| Vitis Version | Minecraft Version | Date |
|---------------|-------------------|------|
| current | 1.21.4 | 2024-12 |
